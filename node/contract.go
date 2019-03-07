package node

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/math"

	"github.com/harmony-one/harmony/contracts"

	"github.com/harmony-one/harmony/internal/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils/contract"
	contract_constants "github.com/harmony-one/harmony/internal/utils/contract"
	"golang.org/x/crypto/sha3"
)

// Constants related to smart contract.
const (
	FaucetContractBinary      = "0x6080604052678ac7230489e8000060015560028054600160a060020a031916331790556101aa806100316000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a5780634ddd108a1461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a723058203e799228fee2fa7c5d15e71c04267a0cc2687c5eff3b48b98f21f355e1064ab30029"
	FaucetContractFund        = 8000000
	FaucetFreeMoneyMethodCall = "0x27c78c42000000000000000000000000"
)

// AddStakingContractToPendingTransactions adds the deposit smart contract the genesis block.
func (node *Node) AddStakingContractToPendingTransactions() {
	// Add a contract deployment transaction
	//Generate contract key and associate funds with the smart contract
	priKey := contract_constants.GenesisBeaconAccountPriKey
	contractAddress := crypto.PubkeyToAddress(priKey.PublicKey)
	//Initially the smart contract should have minimal funds.
	contractFunds := big.NewInt(0)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	dataEnc := common.FromHex(contracts.StakeLockContractBin)
	// Unsigned transaction to avoid the case of transaction address.
	mycontracttx, _ := types.SignTx(types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*100, nil, dataEnc), types.HomesteadSigner{}, priKey)
	//node.StakingContractAddress = crypto.CreateAddress(contractAddress, uint64(0))
	node.StakingContractAddress = node.generateDeployedStakingContractAddress(contractAddress)
	node.addPendingTransactions(types.Transactions{mycontracttx})
}

// In order to get the deployed contract address of a contract, we need to find the nonce of the address that created it.
// (Refer: https://solidity.readthedocs.io/en/v0.5.3/introduction-to-smart-contracts.html#index-8)
// Then we can (re)create the deployed address. Trivially, this is 0 for us.
// The deployed contract address can also be obtained via the receipt of the contract creating transaction.
func (node *Node) generateDeployedStakingContractAddress(contractAddress common.Address) common.Address {
	//Correct Way 1:
	//node.SendTx(mycontracttx)
	//receipts := node.worker.GetCurrentReceipts()
	//deployedcontractaddress = recepits[len(receipts)-1].ContractAddress //get the address from the receipt

	//Correct Way 2:
	//nonce := GetNonce(contractAddress)
	//deployedAddress := crypto.CreateAddress(contractAddress, uint64(nonce))
	nonce := 0
	return crypto.CreateAddress(contractAddress, uint64(nonce))
}

// QueryStakeInfo queries the stake info from the stake contract.
func (node *Node) QueryStakeInfo() *StakeInfo {
	abi, err := abi.JSON(strings.NewReader(contracts.StakeLockContractABI))
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate staking contract's ABI", "error", err)
	}
	bytesData, err := abi.Pack("listLockedAddresses")
	if err != nil {
		utils.GetLogInstance().Error("Failed to generate ABI function bytes data", "error", err)
	}

	priKey := contract_constants.GenesisBeaconAccountPriKey
	deployerAddress := crypto.PubkeyToAddress(priKey.PublicKey)

	state, err := node.blockchain.State()

	stakingContractAddress := crypto.CreateAddress(deployerAddress, uint64(0))
	tx := types.NewTransaction(
		state.GetNonce(deployerAddress),
		stakingContractAddress,
		0,
		nil,
		math.MaxUint64,
		nil,
		bytesData,
	)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, priKey)
	if err != nil {
		utils.GetLogInstance().Error("Failed to sign contract call tx", "error", err)
		return nil
	}
	output, err := node.ContractCaller.CallContract(signedTx)

	if err != nil {
		utils.GetLogInstance().Error("Failed to call staking contract", "error", err)
		return nil
	}

	ret := &StakeInfo{}

	err = abi.Unpack(ret, "listLockedAddresses", output)

	if err != nil {
		utils.GetLogInstance().Error("Failed to unpack stake info", "error", err)
		return nil
	}
	return ret
}

// CreateStakingWithdrawTransaction creates a new withdraw stake transaction
func (node *Node) CreateStakingWithdrawTransaction(stake string) (*types.Transaction, error) {
	//These should be read from somewhere.
	DepositContractPriKey, _ := ecdsa.GenerateKey(crypto.S256(), strings.NewReader("Deposit Smart Contract Key")) //DepositContractPriKey is pk for contract
	DepositContractAddress := crypto.PubkeyToAddress(DepositContractPriKey.PublicKey)                             //DepositContractAddress is the address for the contract
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	nonce := state.GetNonce(crypto.PubkeyToAddress(DepositContractPriKey.PublicKey))

	withdrawFnSignature := []byte("withdraw(uint256)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(withdrawFnSignature)
	methodID := hash.Sum(nil)[:4]

	withdraw := stake
	withdrawstake := new(big.Int)
	withdrawstake.SetString(withdraw, 10)
	paddedAmount := common.LeftPadBytes(withdrawstake.Bytes(), 32)

	var dataEnc []byte
	dataEnc = append(dataEnc, methodID...)
	dataEnc = append(dataEnc, paddedAmount...)

	tx, err := types.SignTx(types.NewTransaction(nonce, DepositContractAddress, node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, node.AccountKey)
	return tx, err
}

func (node *Node) getDeployedStakingContract() common.Address {
	return node.StakingContractAddress
}

// AddFaucetContractToPendingTransactions adds the faucet contract the genesis block.
func (node *Node) AddFaucetContractToPendingTransactions() {
	// Add a contract deployment transactionv
	priKey := node.ContractDeployerKey
	dataEnc := common.FromHex(FaucetContractBinary)
	// Unsigned transaction to avoid the case of transaction address.

	contractFunds := big.NewInt(FaucetContractFund)
	contractFunds = contractFunds.Mul(contractFunds, big.NewInt(params.Ether))
	mycontracttx, _ := types.SignTx(
		types.NewContractCreation(uint64(0), node.Consensus.ShardID, contractFunds, params.TxGasContractCreation*10, nil, dataEnc),
		types.HomesteadSigner{},
		priKey)
	node.ContractAddresses = append(node.ContractAddresses, crypto.CreateAddress(crypto.PubkeyToAddress(priKey.PublicKey), uint64(0)))
	node.addPendingTransactions(types.Transactions{mycontracttx})
}

// CallFaucetContract invokes the faucet contract to give the walletAddress initial money
func (node *Node) CallFaucetContract(address common.Address) common.Hash {
	return node.callGetFreeToken(address)
}

func (node *Node) callGetFreeToken(address common.Address) common.Hash {
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	nonce := state.GetNonce(crypto.PubkeyToAddress(node.ContractDeployerKey.PublicKey))
	return node.callGetFreeTokenWithNonce(address, nonce)
}

func (node *Node) callGetFreeTokenWithNonce(address common.Address, nonce uint64) common.Hash {
	contractData := FaucetFreeMoneyMethodCall + hex.EncodeToString(address.Bytes())
	dataEnc := common.FromHex(contractData)
	utils.GetLogInstance().Info("Sending Free Token to ", "Address", address.Hex())
	tx, _ := types.SignTx(types.NewTransaction(nonce, node.ContractAddresses[0], node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, node.ContractDeployerKey)

	node.addPendingTransactions(types.Transactions{tx})
	return tx.Hash()
}

// DepositToStakingAccounts invokes the faucet contract to give the staking accounts initial money
func (node *Node) DepositToStakingAccounts() {
	state, err := node.blockchain.State()
	if err != nil {
		log.Error("Failed to get chain state", "Error", err)
	}
	nonce := state.GetNonce(crypto.PubkeyToAddress(node.ContractDeployerKey.PublicKey)) + 1 // + 1 because deployer key is already used for faucet contract deployment
	for i, deployAccount := range contract.StakingAccounts {
		address := common.HexToAddress(deployAccount.Address)
		node.callGetFreeTokenWithNonce(address, nonce+uint64(i))
	}
}
