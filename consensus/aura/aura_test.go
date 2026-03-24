package aura

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check that the first block of Gnosis Chain, which doesn't have any transactions,
// does not change the state root.
// func TestEmptyBlock(t *testing.T) {
// 	require := require.New(t)
// 	genesis := core.DefaultGnosisGenesisBlock()
// 	genesisBlock := genesis.ToBlock()

// 	genesis.Config.TerminalTotalDifficultyPassed = false

// 	chainConfig := genesis.Config
// 	auraDB := memdb.NewTestDB(t)
// 	engine, err := aura.NewAuRa(chainConfig.Aura, auraDB)
// 	require.NoError(err)
// 	m := stages.MockWithGenesisEngine(t, genesis, engine, false)

// 	time := uint64(1539016985)
// 	header := core.MakeEmptyHeader(genesisBlock.Header(), chainConfig, time, nil)
// 	header.UncleHash = types.EmptyUncleHash
// 	header.TxHash = trie.EmptyRoot
// 	header.ReceiptHash = trie.EmptyRoot
// 	header.Coinbase = libcommon.HexToAddress("0xcace5b3c29211740e595850e80478416ee77ca21")
// 	header.Difficulty = engine.CalcDifficulty(nil, time,
// 		0,
// 		genesisBlock.Difficulty(),
// 		genesisBlock.NumberU64(),
// 		genesisBlock.Hash(),
// 		genesisBlock.UncleHash(),
// 		genesisBlock.Header().AuRaStep,
// 	)

// 	block := types.NewBlockWithHeader(header)

// 	headers, blocks, receipts := make([]*types.Header, 1), make(types.Blocks, 1), make([]types.Receipts, 1)
// 	headers[0] = header
// 	blocks[0] = block

// 	chain := &core.ChainPack{Headers: headers, Blocks: blocks, Receipts: receipts, TopBlock: block}
// 	err = m.InsertChain(chain)
// 	require.NoError(err)
// }

// --- NewAuRa tests ---

func minimalAuRaConfig() *params.AuRaConfig {
	stepDuration := uint64(5)
	blockReward := uint64(0)
	return &params.AuRaConfig{
		StepDuration: &stepDuration,
		BlockReward:  &blockReward,
		Validators: &params.ValidatorSetJson{
			List: []common.Address{{1}},
		},
	}
}

func TestNewAuRa(t *testing.T) {
	t.Run("MinimalValidConfig", func(t *testing.T) {
		db := rawdb.NewMemoryDatabase()
		engine, err := NewAuRa(minimalAuRaConfig(), db)
		require.NoError(t, err)
		assert.NotNil(t, engine)
		assert.NotNil(t, engine.EpochManager)
		assert.True(t, engine.step.canPropose.Load())
	})

	t.Run("GnosisMainnetConfig", func(t *testing.T) {
		db := rawdb.NewMemoryDatabase()
		engine, err := NewAuRa(params.GnosisChainConfig.Aura, db)
		require.NoError(t, err)
		assert.NotNil(t, engine)
	})

	t.Run("MissingStepDuration", func(t *testing.T) {
		cfg := minimalAuRaConfig()
		cfg.StepDuration = nil // step 0 duration will be missing
		db := rawdb.NewMemoryDatabase()
		_, err := NewAuRa(cfg, db)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "step 0 duration is undefined")
	})

	t.Run("ZeroStepDuration", func(t *testing.T) {
		cfg := minimalAuRaConfig()
		zero := uint64(0)
		cfg.StepDuration = &zero
		db := rawdb.NewMemoryDatabase()
		_, err := NewAuRa(cfg, db)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "step duration cannot be 0")
	})

	t.Run("WithStartStep", func(t *testing.T) {
		cfg := minimalAuRaConfig()
		startStep := uint64(42)
		cfg.StartStep = &startStep
		db := rawdb.NewMemoryDatabase()
		engine, err := NewAuRa(cfg, db)
		require.NoError(t, err)
		// When StartStep is set, calibrate is false and the step is the given value
		assert.Equal(t, uint64(42), engine.step.inner.inner.Load())
	})
}

// --- VerifyHeader tests ---

// mockChainHeaderReader is a minimal implementation of consensus.ChainHeaderReader for testing.
type mockChainHeaderReader struct {
	config  *params.ChainConfig
	headers map[common.Hash]*types.Header
}

func (m *mockChainHeaderReader) Config() *params.ChainConfig  { return m.config }
func (m *mockChainHeaderReader) CurrentHeader() *types.Header { return nil }
func (m *mockChainHeaderReader) GetHeader(hash common.Hash, _ uint64) *types.Header {
	if m.headers != nil {
		return m.headers[hash]
	}
	return nil
}
func (m *mockChainHeaderReader) GetHeaderByNumber(_ uint64) *types.Header { return nil }
func (m *mockChainHeaderReader) GetHeaderByHash(hash common.Hash) *types.Header {
	if m.headers != nil {
		return m.headers[hash]
	}
	return nil
}

// newTestChainReader returns a ChainHeaderReader with no DAO fork (avoids extra-data DAO checks).
func newTestChainReader() *mockChainHeaderReader {
	return &mockChainHeaderReader{
		config: &params.ChainConfig{
			ChainID:        big.NewInt(100), // Gnosis chain ID
			DAOForkSupport: true,
		},
	}
}

func newTestAuRa() *AuRa {
	step := &Step{
		calibrate: false,
		durations: []StepDurationInfo{{TransitionStep: 0, TransitionTimestamp: 0, StepDuration: 5}},
	}
	step.inner.Store(100)
	return &AuRa{
		step:         PermissionedStep{inner: step},
		cfg:          AuthorityRoundParams{},
		EpochManager: NewEpochManager(),
	}
}

func validHeader() *types.Header {
	return &types.Header{
		Number:   big.NewInt(1),
		Time:     uint64(time.Now().Unix()),
		Extra:    []byte{},
		GasLimit: 10_000_000,
		GasUsed:  5_000_000,
	}
}

func TestVerifyHeader(t *testing.T) {
	engine := newTestAuRa()
	chain := newTestChainReader()

	ptrHash := func(h common.Hash) *common.Hash { return &h }
	ptrUint64 := func(v uint64) *uint64 { return &v }

	tests := []struct {
		name    string
		modify  func(h *types.Header)
		wantErr string // "" means no error expected
	}{
		{
			name:   "valid header",
			modify: func(h *types.Header) {},
		},
		{
			name:    "extra-data too long",
			modify:  func(h *types.Header) { h.Extra = make([]byte, params.MaximumExtraDataSize+1) },
			wantErr: "extra-data too long",
		},
		{
			name:    "future timestamp",
			modify:  func(h *types.Header) { h.Time = uint64(time.Now().Unix()) + 60 },
			wantErr: "future",
		},
		{
			name:   "timestamp at allowed boundary",
			modify: func(h *types.Header) { h.Time = uint64(time.Now().Unix()) + 14 },
		},
		{
			name:    "gas limit too high",
			modify:  func(h *types.Header) { h.GasLimit = params.MaxGasLimit + 1 },
			wantErr: "invalid gasLimit",
		},
		{
			name: "gasUsed exceeds gasLimit",
			modify: func(h *types.Header) {
				h.GasLimit = 10_000_000
				h.GasUsed = 10_000_001
			},
			wantErr: "invalid gasUsed",
		},
		{
			name: "gasUsed equals gasLimit",
			modify: func(h *types.Header) {
				h.GasLimit = 10_000_000
				h.GasUsed = 10_000_000
			},
			wantErr: "", // pass
		},
		{
			name:    "withdrawalsHash present",
			modify:  func(h *types.Header) { h.WithdrawalsHash = ptrHash(common.Hash{1}) },
			wantErr: "invalid withdrawalsHash",
		},
		{
			name:    "excessBlobGas present",
			modify:  func(h *types.Header) { h.ExcessBlobGas = ptrUint64(1) },
			wantErr: "invalid excessBlobGas",
		},
		{
			name:    "blobGasUsed present",
			modify:  func(h *types.Header) { h.BlobGasUsed = ptrUint64(1) },
			wantErr: "invalid blobGasUsed",
		},
		{
			name:    "parentBeaconRoot present",
			modify:  func(h *types.Header) { h.ParentBeaconRoot = ptrHash(common.Hash{1}) },
			wantErr: "invalid parentBeaconRoot",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := validHeader()
			tt.modify(h)
			err := engine.VerifyHeader(chain, h)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// --- calculateScore tests ---

func TestCalculateScore(t *testing.T) {
	// score = maxU128 + parentStep - currentStep + currentEmptySteps
	maxU128 := uint256.NewInt(0).SetAllOne()
	maxU128 = maxU128.Rsh(maxU128, 128)

	tests := []struct {
		name        string
		parentStep  uint64
		currentStep uint64
		emptySteps  uint64
		want        *uint256.Int
	}{
		{
			name:        "basic step increment",
			parentStep:  10,
			currentStep: 11,
			emptySteps:  0,
			want:        new(uint256.Int).Sub(maxU128, uint256.NewInt(1)),
		},
		{
			name:        "empty steps add to score",
			parentStep:  10,
			currentStep: 11,
			emptySteps:  5,
			want:        new(uint256.Int).Add(maxU128, uint256.NewInt(4)), // maxU128 + 10 - 11 + 5
		},
		{
			name:        "current step and empty steps offset",
			parentStep:  math.MaxUint64 - 5,
			currentStep: math.MaxUint64,
			emptySteps:  5,
			want:        maxU128,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateScore(tt.parentStep, tt.currentStep, tt.emptySteps)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- CalcDifficulty tests ---

func TestCalcDifficulty(t *testing.T) {
	// simple test just adds coverage to CalcDifficulty and checks consistency with calculateScore. More extensive testing of the score calculation is done in TestCalculateScore.
	t.Run("ConsistentWithCalculateScore", func(t *testing.T) {
		engine := newTestAuRa()
		// engine step is 100, parent step is 90
		parent := &types.Header{
			Number: big.NewInt(1),
			Step:   90,
		}
		diff := engine.CalcDifficulty(nil, 0, parent)
		expected := calculateScore(90, 100, 0)
		assert.Equal(t, expected.ToBig(), diff)
	})
}

// --- CalculateRewards tests ---

// mockSyscallReward returns a Syscall that ABI-encodes a block reward contract response.
// It records the contract address it was called with for assertion.
func mockSyscallReward(beneficiaries []common.Address, amounts []*big.Int) (Syscall, *common.Address) {
	var calledWith common.Address
	return func(addr common.Address, data []byte) ([]byte, error) {
		calledWith = addr
		// Encode the outputs (address[], uint256[]) using the method's output arguments.
		out, err := blockRewardAbi().Methods["reward"].Outputs.Pack(beneficiaries, amounts)
		if err != nil {
			return nil, err
		}
		return out, nil
	}, &calledWith
}

func TestCalculateRewards(t *testing.T) {
	coinbase := common.HexToAddress("0xaaaa")

	t.Run("StaticReward", func(t *testing.T) {
		engine := newTestAuRa()
		engine.cfg.BlockReward = BlockRewardList{
			{blockNum: 0, amount: uint256.NewInt(5_000_000)},
		}
		header := &types.Header{Number: big.NewInt(10), Coinbase: coinbase}

		rewards, err := engine.CalculateRewards(nil, header, nil)
		require.NoError(t, err)
		require.Len(t, rewards, 1)
		assert.Equal(t, coinbase, rewards[0].Beneficiary)
		assert.Equal(t, consensus.RewardAuthor, rewards[0].Kind)
		assert.Equal(t, *big.NewInt(5_000_000), rewards[0].Amount)
	})

	t.Run("StaticRewardTransition", func(t *testing.T) {
		engine := newTestAuRa()
		// blockNum is an activation threshold: all blocks from that number onward
		// use the associated reward amount, until the next transition.
		engine.cfg.BlockReward = BlockRewardList{
			{blockNum: 0, amount: uint256.NewInt(5_000_000)},
			{blockNum: 100, amount: uint256.NewInt(2_000_000)},
		}
		coinbaseA := common.HexToAddress("0x1111")
		coinbaseB := common.HexToAddress("0x2222")

		// Block 99 falls in the [0, 100) range, so it gets the 5M reward
		header := &types.Header{Number: big.NewInt(99), Coinbase: coinbaseA}
		rewards, err := engine.CalculateRewards(nil, header, nil)
		require.NoError(t, err)
		assert.Equal(t, coinbaseA, rewards[0].Beneficiary)
		assert.Equal(t, consensus.RewardAuthor, rewards[0].Kind)
		assert.Equal(t, *big.NewInt(5_000_000), rewards[0].Amount)

		// Block 100 activates the new threshold, so it gets the 2M reward
		header = &types.Header{Number: big.NewInt(100), Coinbase: coinbaseB}
		rewards, err = engine.CalculateRewards(nil, header, nil)
		require.NoError(t, err)
		assert.Equal(t, coinbaseB, rewards[0].Beneficiary)
		assert.Equal(t, consensus.RewardAuthor, rewards[0].Kind)
		assert.Equal(t, *big.NewInt(2_000_000), rewards[0].Amount)
	})

	t.Run("NoRewardConfigured", func(t *testing.T) {
		engine := newTestAuRa()
		engine.cfg.BlockReward = BlockRewardList{}
		header := &types.Header{Number: big.NewInt(1), Coinbase: coinbase}

		_, err := engine.CalculateRewards(nil, header, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("ContractReward", func(t *testing.T) {
		contractAddr := common.HexToAddress("0xbbbb")
		rewardAmount := big.NewInt(3_000_000)

		syscall, calledWith := mockSyscallReward([]common.Address{coinbase}, []*big.Int{rewardAmount})
		engine := newTestAuRa()
		engine.Syscall = syscall
		engine.cfg.BlockRewardContractTransitions = BlockRewardContractList{
			{blockNum: 0, address: contractAddr},
		}

		header := &types.Header{Number: big.NewInt(10), Coinbase: coinbase}
		rewards, err := engine.CalculateRewards(nil, header, nil)
		require.NoError(t, err)
		require.Len(t, rewards, 1)
		assert.Equal(t, coinbase, rewards[0].Beneficiary)
		assert.Equal(t, consensus.RewardExternal, rewards[0].Kind)
		assert.Equal(t, *rewardAmount, rewards[0].Amount)
		assert.Equal(t, contractAddr, *calledWith)
	})

	t.Run("ContractRewardTransition", func(t *testing.T) {
		contractA := common.HexToAddress("0x1111")
		contractB := common.HexToAddress("0x2222")
		authorA := common.HexToAddress("0xaaaa")
		authorB := common.HexToAddress("0xbbbb")
		rewardAmount := big.NewInt(1_000_000)

		syscall, calledWith := mockSyscallReward([]common.Address{authorA}, []*big.Int{rewardAmount})
		engine := newTestAuRa()
		engine.Syscall = syscall
		engine.cfg.BlockRewardContractTransitions = BlockRewardContractList{
			{blockNum: 0, address: contractA},
			{blockNum: 100, address: contractB},
		}

		// Block 99 should use contractA
		header := &types.Header{Number: big.NewInt(99), Coinbase: authorA}
		rewards, err := engine.CalculateRewards(nil, header, nil)
		require.NoError(t, err)
		assert.Equal(t, contractA, *calledWith)
		assert.Equal(t, consensus.RewardExternal, rewards[0].Kind)
		assert.Equal(t, authorA, rewards[0].Beneficiary)
		assert.Equal(t, *rewardAmount, rewards[0].Amount)

		// reassign mock syscall to return a different author for the second header
		syscall, calledWith = mockSyscallReward([]common.Address{authorB}, []*big.Int{rewardAmount})
		engine.Syscall = syscall

		// Block 100 should use contractB with a different author
		header = &types.Header{Number: big.NewInt(100), Coinbase: authorB}
		rewards, err = engine.CalculateRewards(nil, header, nil)
		require.NoError(t, err)
		assert.Equal(t, contractB, *calledWith)
		assert.Equal(t, consensus.RewardExternal, rewards[0].Kind)
		assert.Equal(t, authorB, rewards[0].Beneficiary)
		assert.Equal(t, *rewardAmount, rewards[0].Amount)
	})

	t.Run("ContractTakesPriorityOverStatic", func(t *testing.T) {
		contractAddr := common.HexToAddress("0xbbbb")
		contractReward := big.NewInt(3_000_000)

		syscall, _ := mockSyscallReward([]common.Address{coinbase}, []*big.Int{contractReward})
		engine := newTestAuRa()
		engine.Syscall = syscall
		engine.cfg.BlockReward = BlockRewardList{
			{blockNum: 0, amount: uint256.NewInt(5_000_000)},
		}
		engine.cfg.BlockRewardContractTransitions = BlockRewardContractList{
			{blockNum: 0, address: contractAddr},
		}

		header := &types.Header{Number: big.NewInt(10), Coinbase: coinbase}
		rewards, err := engine.CalculateRewards(nil, header, nil)
		require.NoError(t, err)
		assert.Equal(t, coinbase, rewards[0].Beneficiary)
		assert.Equal(t, consensus.RewardExternal, rewards[0].Kind)
		assert.Equal(t, *contractReward, rewards[0].Amount)
	})

	t.Run("ContractReturnsMultipleBeneficiaries", func(t *testing.T) {
		contractAddr := common.HexToAddress("0xbbbb")
		extra := common.HexToAddress("0xcccc")

		syscall, _ := mockSyscallReward(
			[]common.Address{coinbase, extra},
			[]*big.Int{big.NewInt(1_000_000), big.NewInt(500_000)},
		)
		engine := newTestAuRa()
		engine.Syscall = syscall
		engine.cfg.BlockRewardContractTransitions = BlockRewardContractList{
			{blockNum: 0, address: contractAddr},
		}

		header := &types.Header{Number: big.NewInt(10), Coinbase: coinbase}
		rewards, err := engine.CalculateRewards(nil, header, nil)
		require.NoError(t, err)
		require.Len(t, rewards, 2)
		assert.Equal(t, coinbase, rewards[0].Beneficiary)
		assert.Equal(t, consensus.RewardExternal, rewards[0].Kind)
		assert.Equal(t, *big.NewInt(1_000_000), rewards[0].Amount)
		assert.Equal(t, extra, rewards[1].Beneficiary)
		assert.Equal(t, consensus.RewardExternal, rewards[1].Kind)
		assert.Equal(t, *big.NewInt(500_000), rewards[1].Amount)
	})
}
