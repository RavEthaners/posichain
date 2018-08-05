package blockchain

import (
	"testing"
)

func TestVerifyOneTransactionAndUpdate(t *testing.T) {
	bc := CreateBlockchain("minh", 0)
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, "minh", "alok", 3, 0)
	bc.AddNewUserTransfer(utxoPool, "minh", "rj", 100, 0)

	tx := bc.NewUTXOTransaction("minh", "mark", 10, 0)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}

	if valid, _ := utxoPool.VerifyOneTransaction(tx, nil); !valid {
		t.Error("failed to verify a valid transaction.")
	}
	utxoPool.VerifyOneTransactionAndUpdate(tx)
}

func TestVerifyOneTransactionFail(t *testing.T) {
	bc := CreateBlockchain("minh", 0)
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, "minh", "alok", 3, 0)
	bc.AddNewUserTransfer(utxoPool, "minh", "rj", 100, 0)

	tx := bc.NewUTXOTransaction("minh", "mark", 10, 0)
	if tx == nil {
		t.Error("failed to create a new transaction.")
	}

	tx.TxInput = append(tx.TxInput, tx.TxInput[0])
	if valid, _ := utxoPool.VerifyOneTransaction(tx, nil); valid {
		t.Error("Tx with multiple identical TxInput shouldn't be valid")
	}
}

func TestDeleteOneBalanceItem(t *testing.T) {
	bc := CreateBlockchain("minh", 0)
	utxoPool := CreateUTXOPoolFromGenesisBlockChain(bc)

	bc.AddNewUserTransfer(utxoPool, "minh", "alok", 3, 0)
	bc.AddNewUserTransfer(utxoPool, "alok", "rj", 3, 0)

	if _, ok := utxoPool.UtxoMap["alok"]; ok {
		t.Errorf("alok should not be contained in the balance map")
	}
}

func TestCleanUp(t *testing.T) {
	var utxoPool UTXOPool
	utxoPool.UtxoMap = make(UtxoMap)
	utxoPool.UtxoMap["minh"] = make(TXHash2Vout2AmountMap)
	utxoPool.UtxoMap["rj"] = TXHash2Vout2AmountMap{
		"abcd": {
			0: 1,
		},
	}
	utxoPool.CleanUp()
	if _, ok := utxoPool.UtxoMap["minh"]; ok {
		t.Errorf("minh should not be contained in the balance map")
	}
}
