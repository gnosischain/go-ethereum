package rawdb

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

// ReadEpoch returns the stored epoch transition proof for the given block, or
// (nil, nil) if no entry exists for that block. Callers (e.g. AuRa's epoch
// end-signal logic) rely on a missing key being reported as a nil value rather
// than a backend-specific "not found" error, so we probe with Has first to keep
// this accessor free of any concrete database dependency (e.g. pebble).
func ReadEpoch(db ethdb.KeyValueReader, blockNum uint64, blockHash common.Hash) (transitionProof []byte, err error) {
	k := make([]byte, 40 /* block num uint64 + block hash */)
	binary.BigEndian.PutUint64(k, blockNum)
	copy(k[8:], blockHash[:])
	key := epochKey(k)
	has, err := db.Has(key)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return db.Get(key)
}

// TODO use sqlite if leveldb doesn't work
func FindEpochBeforeOrEqualNumber(db ethdb.KeyValueStore, n uint64) (blockNum uint64, blockHash common.Hash, transitionProof []byte, err error) {
	// seek := make([]byte, 8)
	// if n > 1 {
	// move to the previous block - but actually this
	// could be way older than the previous block number.
	// binary.BigEndian.PutUint64(seek, n-1)
	// }

	it := db.NewIterator(EpochPrefix, nil)
	defer it.Release()

	blockNum = 0
	for it.Next() {
		k := it.Key()
		k = k[len(EpochPrefix):]
		num := binary.BigEndian.Uint64(k)
		if num > n {
			break
		}

		blockNum = num
		transitionProof = it.Value()
		blockHash = common.BytesToHash(k[8:])
	}

	return
}

func WriteEpoch(db ethdb.KeyValueWriter, blockNum uint64, blockHash common.Hash, transitionProof []byte) (err error) {
	k := make([]byte, 40)
	binary.BigEndian.PutUint64(k, blockNum)
	copy(k[8:], blockHash[:])
	return db.Put(epochKey(k), transitionProof)
}

// ReadPendingEpoch returns the stored pending epoch transition proof for the
// given block, or (nil, nil) if no entry exists for that block (as happens at
// genesis). Callers rely on a missing key being reported as a nil value rather
// than a backend-specific "not found" error, so we probe with Has first to keep
// this accessor free of any concrete database dependency (e.g. pebble).
func ReadPendingEpoch(db ethdb.KeyValueReader, blockNum uint64, blockHash common.Hash) (transitionProof []byte, err error) {
	k := make([]byte, 8+32)
	binary.BigEndian.PutUint64(k, blockNum)
	copy(k[8:], blockHash[:])
	key := pendingEpochKey(k)
	has, err := db.Has(key)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return db.Get(key)
}

func WritePendingEpoch(db ethdb.KeyValueWriter, blockNum uint64, blockHash common.Hash, transitionProof []byte) (err error) {
	k := make([]byte, 8+32)
	binary.BigEndian.PutUint64(k, blockNum)
	copy(k[8:], blockHash[:])
	return db.Put(pendingEpochKey(k), transitionProof)
}
