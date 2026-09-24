package aura

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/aura/auraabi"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

type ValidatorSet interface {
	// Signalling that a new epoch has begun.
	//
	// All calls here will be from the `SYSTEM_ADDRESS`: 2^160 - 2
	// and will have an effect on the block's state.
	// The caller provided here may not generate proofs.
	//
	onEpochBegin(header *types.Header, evm *vm.EVM) error

	// Draws a validator nonce modulo number of validators.
	// getWithCaller(parentHash common.Hash, nonce uint, caller consensus.Call) (common.Address, error)
	// Returns the current number of validators.
	// countWithCaller(parentHash common.Hash, caller consensus.Call) (uint64, error)

	// Recover the validator set from the given proof, the block number, and
	// whether this header is first in its set.
	//
	// May fail if the given header doesn't kick off an epoch or
	// the proof is invalid.
	//
	// Returns the set, along with a flag indicating whether finality of a specific
	// hash should be proven.
	epochSet(firstInEpoch bool, num uint64, setProof []byte, evm *vm.EVM) (SimpleList, common.Hash, error)

	// Extract genesis epoch data from the genesis state and header.
	genesisEpochData(header *types.Header) ([]byte, error)

	// Whether this block is the last one in its epoch.
	//
	// Indicates that the validator set changed at the given block in a manner
	// that doesn't require finality.
	//
	// `first` is true if this is the first block in the set.
	signalEpochEnd(firstInEpoch bool, header *types.Header, receipts types.Receipts) ([]byte, error)
}

// nolint
type MultiItem struct {
	num  uint64
	hash common.Hash
	set  ValidatorSet
}

type Multi struct {
	sorted []MultiItem
}

func (s *Multi) Less(i, j int) bool { return s.sorted[i].num < s.sorted[j].num }
func (s *Multi) Len() int           { return len(s.sorted) }
func (s *Multi) Swap(i, j int)      { s.sorted[i], s.sorted[j] = s.sorted[j], s.sorted[i] }

func NewMulti(m map[uint64]ValidatorSet) *Multi {
	if _, ok := m[0]; !ok {
		panic("ValidatorSet has to be specified from block 0")
	}
	list := make([]MultiItem, len(m))
	i := 0
	for n, v := range m {
		list[i] = MultiItem{num: n, set: v}
		i++
	}
	multi := &Multi{sorted: list}
	sort.Sort(multi)
	return multi
}

func (s *Multi) correctSetByNumber(parentNumber uint64) (uint64, ValidatorSet) {
	// get correct set by block number, along with block number at which
	// this set was activated.
	for i := len(s.sorted) - 1; i >= 0; i-- {
		if s.sorted[i].num <= parentNumber+1 {
			return s.sorted[i].num, s.sorted[i].set
		}
	}
	panic("constructor validation ensures that there is at least one validator set for block 0; block 0 is less than any uint; qed")
}

func (s *Multi) epochSet(firstInEpoch bool, num uint64, proof []byte, evm *vm.EVM) (SimpleList, common.Hash, error) {
	setBlock, set := s.correctSetByNumber(num)
	firstInEpoch = setBlock == num
	return set.epochSet(firstInEpoch, num, proof, evm)
}
func (s *Multi) genesisEpochData(header *types.Header) ([]byte, error) {
	_, set := s.correctSetByNumber(0)
	return set.genesisEpochData(header)
}

func (s *Multi) onEpochBegin(header *types.Header, evm *vm.EVM) error {
	_, set := s.correctSetByNumber(header.Number.Uint64())
	return set.onEpochBegin(header, evm)
}
func (s *Multi) signalEpochEnd(_ bool, header *types.Header, r types.Receipts) ([]byte, error) {
	num := header.Number.Uint64()
	setBlock, set := s.correctSetByNumber(num)
	first := setBlock == num
	return set.signalEpochEnd(first, header, r)
}

type SimpleList struct {
	validators []common.Address
}

func (s *SimpleList) epochSet(bool, uint64, []byte, *vm.EVM) (SimpleList, common.Hash, error) {
	return *s, common.Hash{}, nil
}
func (s *SimpleList) onEpochBegin(*types.Header, *vm.EVM) error {
	return nil
}

func (s *SimpleList) genesisEpochData(*types.Header) ([]byte, error) {
	return []byte{}, nil
}

func (s *SimpleList) signalEpochEnd(_ bool, header *types.Header, r types.Receipts) ([]byte, error) {
	return nil, nil
}

// Draws an validator nonce modulo number of validators.

func NewSimpleList(validators []common.Address) *SimpleList {
	return &SimpleList{validators: validators}
}

// The validator contract should have the following interface:
// nolint
type ValidatorSafeContract struct {
	contractAddress common.Address
	// The block number where we resent the queued reports last time.
	resentReportsInBlock atomic.Uint64
	// If set, this is the block number at which the consensus engine switches from AuRa to AuRa
	// with POSDAO modifications.
	posdaoTransition *uint64

	abi abi.ABI
}

func NewValidatorSafeContract(contractAddress common.Address, posdaoTransition *uint64) *ValidatorSafeContract {
	parsed, err := abi.JSON(strings.NewReader(auraabi.ValidatorSetABI))
	if err != nil {
		panic(err)
	}
	return &ValidatorSafeContract{contractAddress: contractAddress, posdaoTransition: posdaoTransition, abi: parsed}
}

// Called for each new block this node is creating.  If this block is
// the first block of an epoch, this is called *after* `on_epoch_begin()`,
// but with the same parameters.
//
// Returns a list of contract calls to be pushed onto the new block.
// func generateEngineTransactions(_firstInEpoch bool, _header *types.Header, _call SystemCall) -> Result<Vec<(Address, Bytes)>, EthcoreError>
func (s *ValidatorSafeContract) epochSet(firstInEpoch bool, num uint64, setProof []byte, evm *vm.EVM) (SimpleList, common.Hash, error) {
	if firstInEpoch {
		var proof FirstValidatorSetProof
		if err := rlp.DecodeBytes(setProof, &proof); err != nil {
			return SimpleList{}, common.Hash{}, fmt.Errorf("[ValidatorSafeContract.epochSet] %w", err)
		}

		if num == 0 {
			return *NewSimpleList([]common.Address{proof.Header.Coinbase}), proof.Header.ParentHash, nil
		}
		l, ok := s.getListSyscall(evm)
		if !ok {
			panic(1)
		}

		return *l, proof.Header.ParentHash, nil
	}
	var proof ValidatorSetProof
	if err := rlp.DecodeBytes(setProof, &proof); err != nil {
		return SimpleList{}, common.Hash{}, fmt.Errorf("[ValidatorSafeContract.epochSet] %w", err)
	}

	ll, ok := s.extractFromEvent(proof.Header, proof.Receipts)
	if !ok {
		panic(1)
	}

	return *ll, common.Hash{}, nil
}

func (s *ValidatorSafeContract) getListSyscall(evm *vm.EVM) (*SimpleList, bool) {
	packed, err := s.abi.Pack("getValidators")
	if err != nil {
		panic(err)
	}
	rules := evm.ChainConfig().Rules(evm.Context.BlockNumber, evm.Context.Random != nil, evm.Context.Time)
	if !rules.IsAmsterdam {
		evm.Context.Transfer(evm.StateDB, params.SystemAddress, s.contractAddress, new(uint256.Int), &rules)
	}
	out, _, err := evm.Call(params.SystemAddress, s.contractAddress, packed, vm.NewGasBudget(math.MaxUint64, 0), new(uint256.Int))
	if err != nil {
		panic(err)
	}
	res, err := s.abi.Unpack("getValidators", out)
	if err != nil {
		panic(err)
	}
	out0 := *abi.ConvertType(res[0], new([]common.Address)).(*[]common.Address)
	return NewSimpleList(out0), true
}

func (s *ValidatorSafeContract) genesisEpochData(header *types.Header) ([]byte, error) {
	return rlp.EncodeToBytes(FirstValidatorSetProof{Header: header, ContractAddress: s.contractAddress})
}

func (s *ValidatorSafeContract) onEpochBegin(header *types.Header, evm *vm.EVM) error {
	data := common.FromHex("75286211") // s.abi.Pack("finalizeChange")
	_, err := systemCall(evm, s.contractAddress, data)
	if err != nil {
		return err
	}

	return nil
}

func (s *ValidatorSafeContract) signalEpochEnd(firstInEpoch bool, header *types.Header, r types.Receipts) ([]byte, error) {
	// transition to the first block of a contract requires finality but has no log event.
	if firstInEpoch {
		return rlp.EncodeToBytes(FirstValidatorSetProof{Header: header, ContractAddress: s.contractAddress})
	}

	_, ok := s.extractFromEvent(header, r)
	if !ok {
		return nil, nil
	}
	proof, err := rlp.EncodeToBytes(ValidatorSetProof{Header: header, Receipts: r})
	if err != nil {
		return nil, err
	}
	return proof, nil
}

func (s *ValidatorSafeContract) extractFromEvent(header *types.Header, receipts types.Receipts) (*SimpleList, bool) {
	if len(receipts) == 0 {
		return nil, false
	}

	// iterate in reverse because only the _last_ change in a given
	// block actually has any effect.
	// the contract should only increment the nonce once.
	for j := len(receipts) - 1; j >= 0; j-- {
		logs := receipts[j].Logs
		for i := 0; i < len(logs); i++ {
			l := logs[i]
			if len(l.Topics) != 2 {
				continue
			}
			found := l.Address == s.contractAddress && l.Topics[0] == EVENT_NAME_HASH && l.Topics[1] == header.ParentHash
			if !found {
				continue
			}

			contract := bind.NewBoundContract(l.Address, s.abi, nil, nil, nil)
			event := new(auraabi.ValidatorSetInitiateChange)
			if err := contract.UnpackLog(event, "InitiateChange", *l); err != nil {
				panic(err)
			}

			// only one last log is taken into account
			return NewSimpleList(event.NewSet), true
		}
	}
	return nil, false
}

const EVENT_NAME = "InitiateChange(bytes32,address[])"

var EVENT_NAME_HASH = crypto.Keccak256Hash([]byte(EVENT_NAME))

// ValidatorContract a validator contract with reporting.
type ValidatorContract struct {
	contractAddress  common.Address
	validators       *ValidatorSafeContract
	posdaoTransition *uint64
}

func (s *ValidatorContract) epochSet(firstInEpoch bool, num uint64, proof []byte, evm *vm.EVM) (SimpleList, common.Hash, error) {
	return s.validators.epochSet(firstInEpoch, num, proof, evm)
}

func (s *ValidatorContract) onEpochBegin(header *types.Header, evm *vm.EVM) error {
	return s.validators.onEpochBegin(header, evm)
}
func (s *ValidatorContract) genesisEpochData(header *types.Header) ([]byte, error) {
	return s.validators.genesisEpochData(header)
}
func (s *ValidatorContract) signalEpochEnd(firstInEpoch bool, header *types.Header, r types.Receipts) ([]byte, error) {
	return s.validators.signalEpochEnd(firstInEpoch, header, r)
}
