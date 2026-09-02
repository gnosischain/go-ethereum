package rawdb

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
)

// readEpochProof retrieves the proof stored under key, returning a nil proof if
// the key is absent. Key-value backends signal a missing key with their own
// error value (pebble, leveldb and memorydb all differ), so presence is probed
// explicitly instead of matching on the returned error.
func readEpochProof(db ethdb.KeyValueReader, key []byte) (transitionProof []byte, err error) {
	ok, err := db.Has(key)
	if err != nil || !ok {
		return nil, err
	}
	return db.Get(key)
}

func ReadEpoch(db ethdb.KeyValueReader, blockNum uint64, blockHash common.Hash) (transitionProof []byte, err error) {
	k := make([]byte, 40 /* block num uint64 + block hash */)
	binary.BigEndian.PutUint64(k, blockNum)
	copy(k[8:], blockHash[:])
	return readEpochProof(db, epochKey(k))
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

func ReadPendingEpoch(db ethdb.KeyValueReader, blockNum uint64, blockHash common.Hash) (transitionProof []byte, err error) {
	k := make([]byte, 8+32)
	binary.BigEndian.PutUint64(k, blockNum)
	copy(k[8:], blockHash[:])
	return readEpochProof(db, pendingEpochKey(k))
}

func WritePendingEpoch(db ethdb.KeyValueWriter, blockNum uint64, blockHash common.Hash, transitionProof []byte) (err error) {
	k := make([]byte, 8+32)
	binary.BigEndian.PutUint64(k, blockNum)
	copy(k[8:], blockHash[:])
	return db.Put(pendingEpochKey(k), transitionProof)
}
