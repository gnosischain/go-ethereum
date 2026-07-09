package aura

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

type configReader struct {
	config *params.ChainConfig
}

func (r configReader) Config() *params.ChainConfig                             { return r.config }
func (r configReader) CurrentHeader() *types.Header                            { return nil }
func (r configReader) GetHeader(hash common.Hash, number uint64) *types.Header { return nil }
func (r configReader) GetHeaderByNumber(number uint64) *types.Header           { return nil }
func (r configReader) GetHeaderByHash(hash common.Hash) *types.Header          { return nil }

func londonAtGenesisConfig() *params.ChainConfig {
	return &params.ChainConfig{
		ChainID:             big.NewInt(10200),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
	}
}

// Chains with an active block gas limit contract set the gas limit from
// contract storage, so header verification must not enforce the parent-bound
// gas limit adjustment rule. Mirrors Chiado block 1, where the canonical
// chain jumps 10M -> 12.5M.
func TestVerifyHeaderGasLimitContract(t *testing.T) {
	config := londonAtGenesisConfig()
	chain := configReader{config: config}
	parent := &types.Header{
		Number:   big.NewInt(0),
		GasLimit: 10_000_000,
		BaseFee:  big.NewInt(params.InitialBaseFee),
	}
	header := &types.Header{
		Number:     big.NewInt(1),
		ParentHash: parent.Hash(),
		GasLimit:   12_500_000,
		Time:       5,
		Difficulty: big.NewInt(1),
		BaseFee:    eip1559.CalcBaseFee(config, parent),
	}

	withContract := &AuRa{cfg: AuthorityRoundParams{
		BlockGasLimitContractTransitions: map[uint64]common.Address{0: common.HexToAddress("0x4000000000000000000000000000000000000001")},
	}}
	if err := withContract.verifyHeader(chain, header, parent); err != nil {
		t.Fatalf("gas limit contract chain rejected legal gas limit jump: %v", err)
	}

	// The base fee must still be validated on gas limit contract chains.
	badBaseFee := *header
	badBaseFee.BaseFee = new(big.Int).Add(header.BaseFee, big.NewInt(1))
	if err := withContract.verifyHeader(chain, &badBaseFee, parent); err == nil {
		t.Fatal("gas limit contract chain accepted invalid base fee")
	}

	// Chains without a gas limit contract keep the parent-bound check.
	withoutContract := &AuRa{cfg: AuthorityRoundParams{}}
	err := withoutContract.verifyHeader(chain, header, parent)
	if err == nil || !strings.Contains(err.Error(), "invalid gas limit") {
		t.Fatalf("chain without gas limit contract accepted out-of-bounds gas limit, err: %v", err)
	}
}

// The contract exemption only applies from its transition block onward.
func TestVerifyHeaderGasLimitContractFutureTransition(t *testing.T) {
	config := londonAtGenesisConfig()
	chain := configReader{config: config}
	parent := &types.Header{
		Number:   big.NewInt(0),
		GasLimit: 10_000_000,
		BaseFee:  big.NewInt(params.InitialBaseFee),
	}
	header := &types.Header{
		Number:     big.NewInt(1),
		ParentHash: parent.Hash(),
		GasLimit:   12_500_000,
		Time:       5,
		Difficulty: big.NewInt(1),
		BaseFee:    eip1559.CalcBaseFee(config, parent),
	}

	engine := &AuRa{cfg: AuthorityRoundParams{
		BlockGasLimitContractTransitions: map[uint64]common.Address{100: common.HexToAddress("0x4000000000000000000000000000000000000001")},
	}}
	err := engine.verifyHeader(chain, header, parent)
	if err == nil || !strings.Contains(err.Error(), "invalid gas limit") {
		t.Fatalf("gas limit contract exemption applied before its transition block, err: %v", err)
	}
}
