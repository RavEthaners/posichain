// Copyright 2018 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package rawdb contains a collection of low level database accessors.
package rawdb

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/metrics"
)

// The fields below define the low level database schema prefixing.
var (
	// databaseVerisionKey tracks the current database version.
	databaseVerisionKey = []byte("DatabaseVersion")

	// headHeaderKey tracks the latest know header's hash.
	headHeaderKey = []byte("LastHeader")

	// headBlockKey tracks the latest know full block's hash.
	headBlockKey = []byte("LastBlock")

	// headFastBlockKey tracks the latest known incomplete block's hash duirng fast sync.
	headFastBlockKey = []byte("LastFast")

	// fastTrieProgressKey tracks the number of trie entries imported during fast sync.
	fastTrieProgressKey = []byte("TrieSync")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	headerPrefix       = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
	headerTDSuffix     = []byte("t") // headerPrefix + num (uint64 big endian) + hash + headerTDSuffix -> td
	headerHashSuffix   = []byte("n") // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerNumberPrefix = []byte("H") // headerNumberPrefix + hash -> num (uint64 big endian)

	blockBodyPrefix     = []byte("b") // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockReceiptsPrefix = []byte("r") // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts

	txLookupPrefix  = []byte("l") // txLookupPrefix + hash -> transaction/receipt lookup metadata
	bloomBitsPrefix = []byte("B") // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits

	shardStatePrefix = []byte("ss") // shardStatePrefix + num (uint64 big endian) + hash -> shardState
	lastCommitsKey   = []byte("LastCommits")

	preimagePrefix = []byte("secure-key-")      // preimagePrefix + hash -> preimage
	configPrefix   = []byte("ethereum-config-") // config prefix for the db

	shardLastCrosslinkPrefix = []byte("shard-last-cross-link") // prefix for shard last crosslink
	crosslinkPrefix          = []byte("crosslink")             // prefix for crosslink
	tempCrosslinkPrefix      = []byte("tempCrosslink")         // prefix for tempCrosslink

	cxReceiptPrefix = []byte("cxReceipt") // prefix for cross shard transaction receipt

	// epochBlockNumberPrefix + epoch (big.Int.Bytes())
	// -> epoch block number (big.Int.Bytes())
	epochBlockNumberPrefix = []byte("harmony-epoch-block-number-")

	// epochVrfBlockNumbersPrefix  + epoch (big.Int.Bytes())
	epochVrfBlockNumbersPrefix = []byte("epoch-vrf-block-numbers-")

	// epochVdfBlockNumberPrefix  + epoch (big.Int.Bytes())
	epochVdfBlockNumberPrefix = []byte("epoch-vdf-block-number-")

	// Chain index prefixes (use `i` + single byte to avoid mixing data types).
	BloomBitsIndexPrefix = []byte("iB") // BloomBitsIndexPrefix is the data table of a chain indexer to track its progress

	preimageCounter    = metrics.NewRegisteredCounter("db/preimage/total", nil)
	preimageHitCounter = metrics.NewRegisteredCounter("db/preimage/hits", nil)
)

// TxLookupEntry is a positional metadata to help looking up the data content of
// a transaction or receipt given only its hash.
type TxLookupEntry struct {
	BlockHash  common.Hash
	BlockIndex uint64
	Index      uint64
}

// encodeBlockNumber encodes a block number as big endian uint64
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

// headerKey = headerPrefix + num (uint64 big endian) + hash
func headerKey(number uint64, hash common.Hash) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// headerTDKey = headerPrefix + num (uint64 big endian) + hash + headerTDSuffix
func headerTDKey(number uint64, hash common.Hash) []byte {
	return append(headerKey(number, hash), headerTDSuffix...)
}

// headerHashKey = headerPrefix + num (uint64 big endian) + headerHashSuffix
func headerHashKey(number uint64) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), headerHashSuffix...)
}

// headerNumberKey = headerNumberPrefix + hash
func headerNumberKey(hash common.Hash) []byte {
	return append(headerNumberPrefix, hash.Bytes()...)
}

// blockBodyKey = blockBodyPrefix + num (uint64 big endian) + hash
func blockBodyKey(number uint64, hash common.Hash) []byte {
	return append(append(blockBodyPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// blockReceiptsKey = blockReceiptsPrefix + num (uint64 big endian) + hash
func blockReceiptsKey(number uint64, hash common.Hash) []byte {
	return append(append(blockReceiptsPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// txLookupKey = txLookupPrefix + hash
func txLookupKey(hash common.Hash) []byte {
	return append(txLookupPrefix, hash.Bytes()...)
}

// bloomBitsKey = bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash
func bloomBitsKey(bit uint, section uint64, hash common.Hash) []byte {
	key := append(append(bloomBitsPrefix, make([]byte, 10)...), hash.Bytes()...)

	binary.BigEndian.PutUint16(key[1:], uint16(bit))
	binary.BigEndian.PutUint64(key[3:], section)

	return key
}

// preimageKey = preimagePrefix + hash
func preimageKey(hash common.Hash) []byte {
	return append(preimagePrefix, hash.Bytes()...)
}

// configKey = configPrefix + hash
func configKey(hash common.Hash) []byte {
	return append(configPrefix, hash.Bytes()...)
}

func shardStateKey(epoch *big.Int) []byte {
	return append(shardStatePrefix, epoch.Bytes()...)
}

func epochBlockNumberKey(epoch *big.Int) []byte {
	return append(epochBlockNumberPrefix, epoch.Bytes()...)
}

func epochVrfBlockNumbersKey(epoch *big.Int) []byte {
	return append(epochVrfBlockNumbersPrefix, epoch.Bytes()...)
}

func epochVdfBlockNumberKey(epoch *big.Int) []byte {
	return append(epochVdfBlockNumberPrefix, epoch.Bytes()...)
}

func shardLastCrosslinkKey(shardID uint32) []byte {
	sbKey := make([]byte, 4)
	binary.BigEndian.PutUint32(sbKey, shardID)
	key := append(crosslinkPrefix, sbKey...)
	return key
}

func crosslinkKey(shardID uint32, blockNum uint64) []byte {
	sbKey := make([]byte, 12)
	binary.BigEndian.PutUint32(sbKey, shardID)
	binary.BigEndian.PutUint64(sbKey[4:], blockNum)
	key := append(crosslinkPrefix, sbKey...)
	return key
}

func tempCrosslinkKey(shardID uint32, blockNum uint64) []byte {
	sbKey := make([]byte, 12)
	binary.BigEndian.PutUint32(sbKey, shardID)
	binary.BigEndian.PutUint64(sbKey[4:], blockNum)
	key := append(tempCrosslinkPrefix, sbKey...)
	return key
}

// cxReceiptKey = cxReceiptsPrefix + shardID + num (uint64 big endian) + hash
func cxReceiptKey(shardID uint32, number uint64, hash common.Hash) []byte {
	sKey := make([]byte, 4)
	binary.BigEndian.PutUint32(sKey, shardID)
	tmp := append(cxReceiptPrefix, sKey...)
	tmp1 := append(tmp, encodeBlockNumber(number)...)
	return append(tmp1, hash.Bytes()...)
}
