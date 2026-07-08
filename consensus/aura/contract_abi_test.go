package aura

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// rewardContractCode returns ABI-encoded empty (address[], uint256[]), the
// return shape of the block reward contract's reward() function.
//
//	PUSH1 0x40 PUSH1 0x00 MSTORE  // offset of address[]
//	PUSH1 0x60 PUSH1 0x20 MSTORE  // offset of uint256[]
//	PUSH1 0x80 PUSH1 0x00 RETURN  // lengths at 0x40/0x60 are zero
var rewardContractCode = common.FromHex("6040600052606060205260806000f3")

// preLondonConfig mirrors the pre-merge forks where hive found the missing
// SYSTEM_ADDRESS account (ConstantinopleFix, Istanbul, Berlin).
func preLondonConfig() *params.ChainConfig {
	return &params.ChainConfig{
		ChainID:             big.NewInt(1),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
	}
}

func postMergeConfig(amsterdamTime *uint64) *params.ChainConfig {
	config := preLondonConfig()
	config.LondonBlock = big.NewInt(0)
	shanghai := uint64(0)
	config.ShanghaiTime = &shanghai
	config.AmsterdamTime = amsterdamTime
	return config
}

func newSyscallTestEVM(config *params.ChainConfig, random *common.Hash) (*vm.EVM, *state.StateDB) {
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	ctx := vm.BlockContext{
		CanTransfer: func(db vm.StateDB, addr common.Address, amount *uint256.Int) bool {
			return db.GetBalance(addr).Cmp(amount) >= 0
		},
		Transfer: func(db vm.StateDB, sender, recipient common.Address, amount *uint256.Int, rules *params.Rules) {
			db.SubBalance(sender, amount, tracing.BalanceChangeTransfer)
			db.AddBalance(recipient, amount, tracing.BalanceChangeTransfer)
		},
		BlockNumber: big.NewInt(1),
		Time:        1,
		Random:      random,
		GasLimit:    30_000_000,
	}
	return vm.NewEVM(ctx, statedb, config, vm.Config{}), statedb
}

// The block reward syscall must create the SYSTEM_ADDRESS account on
// pre-Amsterdam blocks, matching the pre-EIP-7928 zero-value transfer in
// evm.Call that OpenEthereum-lineage clients perform. Without it, pre-merge
// chains compute a state root missing the SYSTEM_ADDRESS leaf.
func TestRewardSyscallCreatesSystemAddress(t *testing.T) {
	evm, statedb := newSyscallTestEVM(preLondonConfig(), nil)
	rewardAddr := common.HexToAddress("0x2000000000000000000000000000000000000001")
	statedb.SetCode(rewardAddr, rewardContractCode, tracing.CodeChangeUnspecified)

	callBlockRewardAbi(rewardAddr, evm, []common.Address{{0x01}}, []consensus.RewardKind{consensus.RewardAuthor})

	if !statedb.Exist(params.SystemAddress) {
		t.Fatal("reward syscall did not create SYSTEM_ADDRESS account")
	}
}

// The withdrawal syscall must create the SYSTEM_ADDRESS account on
// pre-Amsterdam blocks, for the same reason as the reward syscall.
func TestWithdrawalSyscallCreatesSystemAddress(t *testing.T) {
	evm, statedb := newSyscallTestEVM(postMergeConfig(nil), &common.Hash{})
	withdrawalAddr := common.HexToAddress("0x2000000000000000000000000000000000000002")
	statedb.SetCode(withdrawalAddr, []byte{0x00}, tracing.CodeChangeUnspecified) // STOP
	engine := &AuRa{cfg: AuthorityRoundParams{WithdrawalContractAddress: &withdrawalAddr}}

	if err := engine.ExecuteSystemWithdrawals(evm, nil); err != nil {
		t.Fatalf("ExecuteSystemWithdrawals: %v", err)
	}
	if !statedb.Exist(params.SystemAddress) {
		t.Fatal("withdrawal syscall did not create SYSTEM_ADDRESS account")
	}
}

// When the configured reward contract has no code, the reward syscall must
// be skipped entirely: no call and no SYSTEM_ADDRESS creation. This matches
// the execution-specs reference, which returns before the system call (and
// before creating SYSTEM_ADDRESS) if the contract's code hash is empty.
func TestRewardSyscallSkipsMissingContract(t *testing.T) {
	evm, statedb := newSyscallTestEVM(preLondonConfig(), nil)
	rewardAddr := common.HexToAddress("0x2000000000000000000000000000000000000001")

	beneficiaries, amounts := callBlockRewardAbi(rewardAddr, evm, []common.Address{{0x01}}, []consensus.RewardKind{consensus.RewardAuthor})

	if beneficiaries != nil || amounts != nil {
		t.Fatal("expected no rewards from codeless reward contract")
	}
	if statedb.Exist(params.SystemAddress) {
		t.Fatal("reward syscall created SYSTEM_ADDRESS despite codeless reward contract")
	}
}

// Post-Amsterdam, EIP-7928 removes the syscall value transfer, so the
// SYSTEM_ADDRESS account must no longer be created.
func TestRewardSyscallPostAmsterdam(t *testing.T) {
	amsterdam := uint64(0)
	evm, statedb := newSyscallTestEVM(postMergeConfig(&amsterdam), &common.Hash{})
	rewardAddr := common.HexToAddress("0x2000000000000000000000000000000000000001")
	statedb.SetCode(rewardAddr, rewardContractCode, tracing.CodeChangeUnspecified)

	callBlockRewardAbi(rewardAddr, evm, []common.Address{{0x01}}, []consensus.RewardKind{consensus.RewardAuthor})

	if statedb.Exist(params.SystemAddress) {
		t.Fatal("reward syscall created SYSTEM_ADDRESS account post-Amsterdam")
	}
}
