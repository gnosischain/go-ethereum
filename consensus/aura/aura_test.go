package aura_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/aura"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/params"
)

// TestGnosisEmptyBlock checks that blocks in the shape of the first blocks of
// Gnosis Chain, which carry no transactions and predate the block reward
// contract (blockReward is 0 until block 1310), do not change the state root:
// the zero-value reward paid to the validator must not leave an empty account
// behind.
func TestGnosisEmptyBlock(t *testing.T) {
	const blocks = 4

	genesis := core.DefaultGnosisGenesisBlock()

	engine, err := aura.NewAuRa(genesis.Config.Aura, rawdb.NewMemoryDatabase())
	require.NoError(t, err)
	defer engine.Close()

	// The only validator of the initial Gnosis epoch, and the author of block 1.
	validator := common.HexToAddress("0xcace5b3c29211740e595850e80478416ee77ca21")

	db, chain, _ := core.GenerateChainWithGenesis(genesis, beacon.New(engine), blocks, func(i int, b *core.BlockGen) {
		b.SetCoinbase(validator)
	})
	for _, block := range chain {
		require.Equal(t, params.GnosisGenesisStateRoot, block.Root(), "empty block %d changed the state root", block.NumberU64())
	}

	bc, err := core.NewBlockChain(db, genesis, beacon.New(engine), nil)
	require.NoError(t, err)
	defer bc.Stop()

	n, err := bc.InsertChain(chain)
	require.NoError(t, err)
	require.Equal(t, blocks, n)
	require.Equal(t, chain[blocks-1].Hash(), bc.CurrentBlock().Hash())
}
