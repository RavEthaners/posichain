/*
The btctxgen iterates the btc tx history block by block, transaction by transaction.

The btxtxiter provide a simple api called `NextTx` for us to move thru TXs one by one.

Same as txgen, iterate on each shard to generate simulated TXs (GenerateSimulatedTransactions):

 1. Get a new btc tx
 2. If it's a coinbase tx, create a corresponding coinbase tx in our blockchain
 3. Otherwise, create a normal TX, which might be cross-shard and might not, depending on whether all the TX inputs belong to the current shard.

Same as txgen, send single shard tx shard by shard, then broadcast cross shard tx.

TODO

Some todos for ricl
  * correct the logic to outputing to one of the input shard, rather than the current shard
*/
package main

import (
	"flag"
	"fmt"
	"sync"
	"time"

	"github.com/simple-rules/harmony-benchmark/blockchain"
	"github.com/simple-rules/harmony-benchmark/client"
	"github.com/simple-rules/harmony-benchmark/client/btctxiter"
	client_config "github.com/simple-rules/harmony-benchmark/client/config"
	"github.com/simple-rules/harmony-benchmark/consensus"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
	"github.com/simple-rules/harmony-benchmark/log"
	"github.com/simple-rules/harmony-benchmark/node"
	"github.com/simple-rules/harmony-benchmark/p2p"
	proto_node "github.com/simple-rules/harmony-benchmark/proto/node"
)

type txGenSettings struct {
	crossShard        bool
	maxNumTxsPerBatch int
}

type TXRef struct {
	txID      [32]byte
	shardID   uint32
	toAddress [20]byte // we use the same toAddress in btc and hmy
}

var (
	utxoPoolMutex sync.Mutex
	setting       txGenSettings
	iter          btctxiter.BTCTXIterator
	utxoMapping   map[string]TXRef // btcTXID to { txID, shardID }
	// map from bitcoin address to a int value (the privKey in hmy)
	addressMapping map[[20]byte]int
	currentInt     int
)

func getHmyInt(btcAddr [20]byte) int {
	var privKey int
	if privKey, ok := addressMapping[btcAddr]; !ok { // If cannot find key
		privKey = currentInt
		addressMapping[btcAddr] = privKey
		currentInt++
	}
	return privKey
}

// Generates at most "maxNumTxs" number of simulated transactions based on the current UtxoPools of all shards.
// The transactions are generated by going through the existing utxos and
// randomly select a subset of them as the input for each new transaction. The output
// address of the new transaction are randomly selected from [0 - N), where N is the total number of fake addresses.
//
// When crossShard=true, besides the selected utxo input, select another valid utxo as input from the same address in a second shard.
// Similarly, generate another utxo output in that second shard.
//
// NOTE: the genesis block should contain N coinbase transactions which add
//       token (1000) to each address in [0 - N). See node.AddTestingAddresses()
//
// Params:
//     shardID                    - the shardID for current shard
//     dataNodes                  - nodes containing utxopools of all shards
// Returns:
//     all single-shard txs
//     all cross-shard txs
func generateSimulatedTransactions(shardID int, dataNodes []*node.Node) ([]*blockchain.Transaction, []*blockchain.Transaction) {
	/*
		  UTXO map structure:
		  {
			  address: {
				  txID: {
					  outputIndex: value
				  }
			  }
		  }
	*/

	utxoPoolMutex.Lock()
	txs := []*blockchain.Transaction{}
	crossTxs := []*blockchain.Transaction{}

	nodeShardID := dataNodes[shardID].Consensus.ShardID
	cnt := 0

LOOP:
	for true {
		btcTx := iter.NextTx()
		if btcTx == nil {
			log.Error("Failed to parse tx", "height", iter.GetBlockIndex())
		}
		tx := blockchain.Transaction{}
		isCrossShardTx := false

		if btctxiter.IsCoinBaseTx(btcTx) {
			// ricl: coinbase tx should just have one txo
			btcTXO := btcTx.Vout[0]
			btcTXOAddr := btcTXO.ScriptPubKey.Addresses[0]
			var toAddress [20]byte
			copy(toAddress[:], btcTXOAddr) // TODO(ricl): string to [20]byte
			hmyInt := getHmyInt(toAddress)
			tx = *blockchain.NewCoinbaseTX(pki.GetAddressFromInt(hmyInt), "", nodeShardID)

			utxoMapping[btcTx.Hash] = TXRef{tx.ID, nodeShardID, toAddress}
		} else {
			var btcFromAddresses [][20]byte
			for _, btcTXI := range btcTx.Vin {
				btcTXIDStr := btcTXI.Txid
				txRef := utxoMapping[btcTXIDStr] // find the corresponding harmony tx info
				if txRef.shardID != nodeShardID {
					isCrossShardTx = true
				}
				tx.TxInput = append(tx.TxInput, *blockchain.NewTXInput(blockchain.NewOutPoint(&txRef.txID, btcTXI.Vout), [20]byte{}, txRef.shardID))
				// Add the from address to array, so that we can later use it to sign the tx.
				btcFromAddresses = append(btcFromAddresses, txRef.toAddress)
			}
			for _, btcTXO := range btcTx.Vout {
				for _, btcTXOAddr := range btcTXO.ScriptPubKey.Addresses {
					var toAddress [20]byte
					copy(toAddress[:], btcTXOAddr) //TODO(ricl): string to [20]byte
					txo := blockchain.TXOutput{Amount: int(btcTXO.Value), Address: toAddress, ShardID: nodeShardID}
					tx.TxOutput = append(tx.TxOutput, txo)
					utxoMapping[btcTx.Txid] = TXRef{tx.ID, nodeShardID, toAddress}
				}
			}
			// get private key and sign the tx
			for _, btcFromAddress := range btcFromAddresses {
				hmyInt := getHmyInt(btcFromAddress)
				tx.SetID() // TODO(RJ): figure out the correct way to set Tx ID.
				tx.Sign(pki.GetPrivateKeyScalarFromInt(hmyInt))
			}
		}

		if isCrossShardTx {
			crossTxs = append(crossTxs, &tx)
		} else {
			txs = append(txs, &tx)
		}
		// log.Debug("[Generator] transformed btc tx", "block height", iter.GetBlockIndex(), "block tx count", iter.GetBlock().TxCount, "block tx cnt", len(iter.GetBlock().Txs), "txi", len(tx.TxInput), "txo", len(tx.TxOutput), "txCount", cnt)
		cnt++
		if cnt >= setting.maxNumTxsPerBatch {
			break LOOP
		}
	}

	utxoPoolMutex.Unlock()

	log.Debug("[Generator] generated transations", "single-shard", len(txs), "cross-shard", len(crossTxs))
	return txs, crossTxs
}

func initClient(clientNode *node.Node, clientPort string, shardIdLeaderMap *map[uint32]p2p.Peer, nodes *[]*node.Node) {
	if clientPort == "" {
		return
	}

	clientNode.Client = client.NewClient(shardIdLeaderMap)

	// This func is used to update the client's utxopool when new blocks are received from the leaders
	updateBlocksFunc := func(blocks []*blockchain.Block) {
		log.Debug("Received new block from leader", "len", len(blocks))
		for _, block := range blocks {
			for _, node := range *nodes {
				if node.Consensus.ShardID == block.ShardId {
					log.Debug("Adding block from leader", "shardId", block.ShardId)
					// Add it to blockchain
					utxoPoolMutex.Lock()
					node.AddNewBlock(block)
					utxoPoolMutex.Unlock()
				} else {
					continue
				}
			}
		}
	}
	clientNode.Client.UpdateBlocks = updateBlocksFunc

	// Start the client server to listen to leader's message
	go func() {
		clientNode.StartServer(clientPort)
	}()
}

func main() {
	configFile := flag.String("config_file", "local_config.txt", "file containing all ip addresses and config")
	maxNumTxsPerBatch := flag.Int("max_num_txs_per_batch", 10000, "number of transactions to send per message")
	logFolder := flag.String("log_folder", "latest", "the folder collecting the logs of this execution")
	flag.Parse()

	// Read the configs
	config := client_config.NewConfig()
	config.ReadConfigFile(*configFile)
	shardIdLeaderMap := config.GetShardIdToLeaderMap()

	// Do cross shard tx if there are more than one shard
	setting.crossShard = len(shardIdLeaderMap) > 1
	setting.maxNumTxsPerBatch = *maxNumTxsPerBatch

	// TODO(Richard): refactor this chuck to a single method
	// Setup a logger to stdout and log file.
	logFileName := fmt.Sprintf("./%v/txgen.log", *logFolder)
	h := log.MultiHandler(
		log.StdoutHandler,
		log.Must.FileHandler(logFileName, log.LogfmtFormat()), // Log to file
		// log.Must.NetHandler("tcp", ":3000", log.JSONFormat()) // Log to remote
	)
	log.Root().SetHandler(h)

	iter.Init()
	utxoMapping = make(map[string]TXRef)
	addressMapping = make(map[[20]byte]int)

	currentInt = 1 // start from address 1
	// Nodes containing utxopools to mirror the shards' data in the network
	nodes := []*node.Node{}
	for shardID, _ := range shardIdLeaderMap {
		node := node.New(&consensus.Consensus{ShardID: shardID}, nil)
		// Assign many fake addresses so we have enough address to play with at first
		node.AddTestingAddresses(10000)
		nodes = append(nodes, node)
	}

	// Client/txgenerator server node setup
	clientPort := config.GetClientPort()
	consensusObj := consensus.NewConsensus("0", clientPort, "0", nil, p2p.Peer{})
	clientNode := node.New(consensusObj, nil)

	initClient(clientNode, clientPort, &shardIdLeaderMap, &nodes)

	// Transaction generation process
	time.Sleep(3 * time.Second) // wait for nodes to be ready

	leaders := []p2p.Peer{}
	for _, leader := range shardIdLeaderMap {
		leaders = append(leaders, leader)
	}

	for true {
		allCrossTxs := []*blockchain.Transaction{}
		// Generate simulated transactions
		for shardId, leader := range shardIdLeaderMap {
			txs, crossTxs := generateSimulatedTransactions(int(shardId), nodes)
			allCrossTxs = append(allCrossTxs, crossTxs...)

			log.Debug("[Generator] Sending single-shard txs ...", "leader", leader, "numTxs", len(txs), "numCrossTxs", len(crossTxs), "block height", iter.GetBlockIndex())
			msg := proto_node.ConstructTransactionListMessage(txs)
			p2p.SendMessage(leader, msg)
			// Note cross shard txs are later sent in batch
		}

		if len(allCrossTxs) > 0 {
			log.Debug("[Generator] Broadcasting cross-shard txs ...", "allCrossTxs", len(allCrossTxs))
			msg := proto_node.ConstructTransactionListMessage(allCrossTxs)
			p2p.BroadcastMessage(leaders, msg)

			// Put cross shard tx into a pending list waiting for proofs from leaders
			if clientPort != "" {
				clientNode.Client.PendingCrossTxsMutex.Lock()
				for _, tx := range allCrossTxs {
					clientNode.Client.PendingCrossTxs[tx.ID] = tx
				}
				clientNode.Client.PendingCrossTxsMutex.Unlock()
			}
		}

		time.Sleep(500 * time.Millisecond) // Send a batch of transactions periodically
	}

	// Send a stop message to stop the nodes at the end
	msg := proto_node.ConstructStopMessage()
	peers := append(config.GetValidators(), leaders...)
	p2p.BroadcastMessage(peers, msg)
}
