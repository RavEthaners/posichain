package blockchain

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"sync"
)

// UTXOPool stores transactions and balance associated with each address.
type UTXOPool struct {
	// Mapping from address to a map of transaction id to a map of the index of output
	// array in that transaction to that balance.
	/*
	   The 3-d map's structure:
	     address - [
	                txId1 - [
	                        outputIndex1 - value1
	                        outputIndex2 - value2
	                       ]
	                txId2 - [
	                        outputIndex1 - value1
	                        outputIndex2 - value2
	                       ]
	               ]
	*/
	UtxoMap       map[string]map[string]map[int]int
	LockedUtxoMap map[string]map[string]map[int]int
	ShardId       uint32
	mutex         sync.Mutex
}

// VerifyTransactions verifies if a list of transactions valid for this shard.
func (utxoPool *UTXOPool) VerifyTransactions(transactions []*Transaction) bool {
	spentTXOs := make(map[string]map[string]map[int]bool)
	if utxoPool != nil {
		for _, tx := range transactions {
			if valid, crossShard := utxoPool.VerifyOneTransaction(tx, &spentTXOs); !crossShard && !valid {
				return false
			}
		}
	}
	return true
}

// VerifyOneTransaction verifies if a list of transactions valid.
func (utxoPool *UTXOPool) VerifyOneTransaction(tx *Transaction, spentTXOs *map[string]map[string]map[int]bool) (valid, crossShard bool) {
	if len(tx.Proofs) != 0 {
		return utxoPool.VerifyUnlockTransaction(tx)
	}

	if spentTXOs == nil {
		spentTXOs = &map[string]map[string]map[int]bool{}
	}
	inTotal := 0
	// Calculate the sum of TxInput
	for _, in := range tx.TxInput {
		// Only check the input for my own shard.
		if in.ShardId != utxoPool.ShardId {
			crossShard = true
			continue
		}

		inTxID := hex.EncodeToString(in.TxID[:])
		index := in.TxOutputIndex
		// Check if the transaction with the addres is spent or not.
		if val, ok := (*spentTXOs)[in.Address][inTxID][index]; ok {
			if val {
				return false, crossShard
			}
		}
		// Mark the transactions with the address and index spent.
		if _, ok := (*spentTXOs)[in.Address]; !ok {
			(*spentTXOs)[in.Address] = make(map[string]map[int]bool)
		}
		if _, ok := (*spentTXOs)[in.Address][inTxID]; !ok {
			(*spentTXOs)[in.Address][inTxID] = make(map[int]bool)
		}
		(*spentTXOs)[in.Address][inTxID][index] = true

		// Sum the balance up to the inTotal.
		utxoPool.mutex.Lock()
		if val, ok := utxoPool.UtxoMap[in.Address][inTxID][index]; ok {
			inTotal += val
		} else {
			utxoPool.mutex.Unlock()
			return false, crossShard
		}
		utxoPool.mutex.Unlock()
	}

	outTotal := 0
	// Calculate the sum of TxOutput
	for _, out := range tx.TxOutput {
		outTotal += out.Value
	}

	if (crossShard && inTotal >= outTotal) || (!crossShard && inTotal != outTotal) {
		return false, crossShard
	}

	if inTotal == 0 {
		return false, false // Here crossShard is false, because if there is no business for this shard, it's effectively not crossShard no matter what.
	}

	return true, crossShard
}

// Verify a cross shard transaction that contains proofs for unlock-to-commit/abort.
func (utxoPool *UTXOPool) VerifyUnlockTransaction(tx *Transaction) (valid, crossShard bool) {
	valid = true
	crossShard = false // unlock transaction is treated as crossShard=false because it will be finalized now (doesn't need more steps)
	txInputs := make(map[TXInput]bool)
	for _, curProof := range tx.Proofs {
		for _, txInput := range curProof.TxInput {
			txInputs[txInput] = true
		}
	}
	for _, txInput := range tx.TxInput {
		val, ok := txInputs[txInput]
		if !ok || !val {
			valid = false
		}
	}
	return
}

// Update Utxo balances with a list of new transactions.
func (utxoPool *UTXOPool) Update(transactions []*Transaction) {
	if utxoPool != nil {
		for _, tx := range transactions {
			utxoPool.UpdateOneTransaction(tx)
		}
	}
}

// UpdateOneTransaction updates utxoPool in respect to the new Transaction.
func (utxoPool *UTXOPool) UpdateOneTransaction(tx *Transaction) {
	isUnlockTx := len(tx.Proofs) != 0
	unlockToCommit := true
	if isUnlockTx {
		for _, proof := range tx.Proofs {
			if !proof.Accept {
				unlockToCommit = false // if any proof is a rejection, they it's a unlock-to-abort tx. Otherwise, it's unlock-to-commit
			}
		}
	}

	isCrossShard := false
	// check whether it's a cross shard tx.
	for _, in := range tx.TxInput {
		if in.ShardId != utxoPool.ShardId {
			isCrossShard = true
			break
		}
	}

	isValidCrossShard := true
	if isCrossShard {
		// Check whether for this shard this cross transaction is valid or not.
		for _, in := range tx.TxInput {
			// Only check the input for my own shard.
			if in.ShardId != utxoPool.ShardId {
				continue
			}
			inTxID := hex.EncodeToString(in.TxID[:])
			if _, ok := utxoPool.UtxoMap[in.Address][inTxID][in.TxOutputIndex]; !ok {
				isValidCrossShard = false
			}
		}
	}

	utxoPool.mutex.Lock()
	defer utxoPool.mutex.Unlock()
	if utxoPool != nil {
		txID := hex.EncodeToString(tx.ID[:])

		// Remove
		if !isUnlockTx {
			if isValidCrossShard {
				for _, in := range tx.TxInput {
					// Only check the input for my own shard.
					if in.ShardId != utxoPool.ShardId {
						continue
					}

					// NOTE: for the locking phase of cross tx, the utxo is simply removed from the pool.
					inTxID := hex.EncodeToString(in.TxID[:])
					value := utxoPool.UtxoMap[in.Address][inTxID][in.TxOutputIndex]
					utxoPool.DeleteOneUtxo(in.Address, inTxID, in.TxOutputIndex)
					if isCrossShard {
						// put the delete (locked) utxo into a separate locked utxo pool
						inTxID := hex.EncodeToString(in.TxID[:])
						if _, ok := utxoPool.LockedUtxoMap[in.Address]; !ok {
							utxoPool.LockedUtxoMap[in.Address] = make(map[string]map[int]int)
							utxoPool.LockedUtxoMap[in.Address][inTxID] = make(map[int]int)
						}
						if _, ok := utxoPool.LockedUtxoMap[in.Address][inTxID]; !ok {
							utxoPool.LockedUtxoMap[in.Address][inTxID] = make(map[int]int)
						}
						utxoPool.LockedUtxoMap[in.Address][inTxID][in.TxOutputIndex] = value
					}
				}
			}
		}

		// Update
		if !isCrossShard || isUnlockTx {
			if !unlockToCommit {
				if isValidCrossShard {
					// unlock-to-abort, bring back (unlock) the utxo input
					for _, in := range tx.TxInput {
						// Only unlock the input for my own shard.
						if in.ShardId != utxoPool.ShardId {
							continue
						}

						// Simply bring back the locked (removed) utxo
						inTxID := hex.EncodeToString(in.TxID[:])
						if _, ok := utxoPool.UtxoMap[in.Address]; !ok {
							utxoPool.UtxoMap[in.Address] = make(map[string]map[int]int)
							utxoPool.UtxoMap[in.Address][inTxID] = make(map[int]int)
						}
						if _, ok := utxoPool.UtxoMap[in.Address][inTxID]; !ok {
							utxoPool.UtxoMap[in.Address][inTxID] = make(map[int]int)
						}
						value := utxoPool.LockedUtxoMap[in.Address][inTxID][in.TxOutputIndex]
						utxoPool.UtxoMap[in.Address][inTxID][in.TxOutputIndex] = value

						utxoPool.DeleteOneLockedUtxo(in.Address, inTxID, in.TxOutputIndex)
					}
				}
			} else {
				// normal utxo output update
				for index, out := range tx.TxOutput {
					// Only check the input for my own shard.
					if out.ShardId != utxoPool.ShardId {
						continue
					}
					if _, ok := utxoPool.UtxoMap[out.Address]; !ok {
						utxoPool.UtxoMap[out.Address] = make(map[string]map[int]int)
						utxoPool.UtxoMap[out.Address][txID] = make(map[int]int)
					}
					if _, ok := utxoPool.UtxoMap[out.Address][txID]; !ok {
						utxoPool.UtxoMap[out.Address][txID] = make(map[int]int)
					}
					utxoPool.UtxoMap[out.Address][txID][index] = out.Value
				}
			}
		} // If it's a cross shard locking Tx, then don't update so the input UTXOs are locked (removed), and the money is not spendable until unlock-to-commit or unlock-to-abort
	}
}

// VerifyOneTransactionAndUpdate verifies and update a valid transaction.
// Return false if the transaction is not valid.
func (utxoPool *UTXOPool) VerifyOneTransactionAndUpdate(tx *Transaction) bool {
	if valid, _ := utxoPool.VerifyOneTransaction(tx, nil); valid {
		utxoPool.UpdateOneTransaction(tx)
		return true
	}
	return false
}

// VerifyAndUpdate verifies a list of transactions and update utxoPool.
func (utxoPool *UTXOPool) VerifyAndUpdate(transactions []*Transaction) bool {
	if utxoPool.VerifyTransactions(transactions) {
		utxoPool.Update(transactions)
		return true
	}
	return false
}

// CreateUTXOPoolFromTransaction a Utxo pool from a genesis transaction.
func CreateUTXOPoolFromTransaction(tx *Transaction, shardId uint32) *UTXOPool {
	var utxoPool UTXOPool
	txID := hex.EncodeToString(tx.ID[:])
	utxoPool.UtxoMap = make(map[string]map[string]map[int]int)
	utxoPool.LockedUtxoMap = make(map[string]map[string]map[int]int)
	for index, out := range tx.TxOutput {
		utxoPool.UtxoMap[out.Address] = make(map[string]map[int]int)
		utxoPool.UtxoMap[out.Address][txID] = make(map[int]int)
		utxoPool.UtxoMap[out.Address][txID][index] = out.Value
	}
	utxoPool.ShardId = shardId
	return &utxoPool
}

// CreateUTXOPoolFromGenesisBlockChain a Utxo pool from a genesis blockchain.
func CreateUTXOPoolFromGenesisBlockChain(bc *Blockchain) *UTXOPool {
	tx := bc.Blocks[0].Transactions[0]
	shardId := bc.Blocks[0].ShardId
	return CreateUTXOPoolFromTransaction(tx, shardId)
}

// SelectTransactionsForNewBlock returns a list of index of valid transactions for the new block.
func (utxoPool *UTXOPool) SelectTransactionsForNewBlock(transactions []*Transaction, maxNumTxs int) ([]*Transaction, []*Transaction, []*CrossShardTxAndProof) {
	selected, unselected, crossShardTxs := []*Transaction{}, []*Transaction{}, []*CrossShardTxAndProof{}
	spentTXOs := make(map[string]map[string]map[int]bool)
	for _, tx := range transactions {
		valid, crossShard := utxoPool.VerifyOneTransaction(tx, &spentTXOs)

		if len(selected) < maxNumTxs {
			if valid || crossShard {
				selected = append(selected, tx)
				if crossShard {
					proof := CrossShardTxProof{Accept: valid, TxID: tx.ID, TxInput: getShardTxInput(tx, utxoPool.ShardId)}
					txAndProof := CrossShardTxAndProof{tx, &proof}
					crossShardTxs = append(crossShardTxs, &txAndProof)
				}
			} else {
				unselected = append(unselected, tx)
			}
		} else {
			// TODO: discard invalid transactions
			unselected = append(unselected, tx)
		}
	}
	return selected, unselected, crossShardTxs
}

func getShardTxInput(transaction *Transaction, shardId uint32) []TXInput {
	result := []TXInput{}
	for _, txInput := range transaction.TxInput {
		if txInput.ShardId == shardId {
			result = append(result, txInput)
		}
	}
	return result
}

// DeleteOneBalanceItem deletes one balance item of UTXOPool and clean up if possible.
func (utxoPool *UTXOPool) DeleteOneUtxo(address, txID string, index int) {
	delete(utxoPool.UtxoMap[address][txID], index)
	if len(utxoPool.UtxoMap[address][txID]) == 0 {
		delete(utxoPool.UtxoMap[address], txID)
		if len(utxoPool.UtxoMap[address]) == 0 {
			delete(utxoPool.UtxoMap, address)
		}
	}
}

// DeleteOneBalanceItem deletes one balance item of UTXOPool and clean up if possible.
func (utxoPool *UTXOPool) DeleteOneLockedUtxo(address, txID string, index int) {
	delete(utxoPool.LockedUtxoMap[address][txID], index)
	if len(utxoPool.LockedUtxoMap[address][txID]) == 0 {
		delete(utxoPool.LockedUtxoMap[address], txID)
		if len(utxoPool.LockedUtxoMap[address]) == 0 {
			delete(utxoPool.LockedUtxoMap, address)
		}
	}
}

// CleanUp cleans up UTXOPool.
func (utxoPool *UTXOPool) CleanUp() {
	for address, txMap := range utxoPool.UtxoMap {
		for txid, outIndexes := range txMap {
			for index, value := range outIndexes {
				if value == 0 {
					delete(utxoPool.UtxoMap[address][txid], index)
				}
			}
			if len(utxoPool.UtxoMap[address][txid]) == 0 {
				delete(utxoPool.UtxoMap[address], txid)
			}
		}
		if len(utxoPool.UtxoMap[address]) == 0 {
			delete(utxoPool.UtxoMap, address)
		}
	}
}

// Used for debugging.
func (utxoPool *UTXOPool) String() string {
	res := ""
	for address, v1 := range utxoPool.UtxoMap {
		for txid, v2 := range v1 {
			for index, value := range v2 {
				res += fmt.Sprintf("address: %v, tx id: %v, index: %v, value: %v\n", address, txid, index, value)

			}
		}
	}
	return res
}

// Get a snapshot copy of the current pool
func (utxoPool *UTXOPool) GetSizeInByteOfUtxoMap() int {
	utxoPool.mutex.Lock()
	defer utxoPool.mutex.Unlock()
	byteBuffer := bytes.NewBuffer([]byte{})
	encoder := gob.NewEncoder(byteBuffer)
	encoder.Encode(utxoPool.UtxoMap)
	return len(byteBuffer.Bytes())
}
