package aura

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"

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

// nolint
type CallResults struct {
	data      []byte
	proof     [][]byte
	execError string
}

// Type alias for a function we can make calls through synchronously.
// Returns the call result and state proof for each call.
type Call func(common.Address, []byte) (CallResults, error)

// A system-calling closure. Enacts calls on a block's state from the system address.
type SystemCall func(common.Address, []byte) (CallResults, error)

type client interface {
	CallAtBlockHash(common.Hash, common.Address, []byte) (CallResults, error)
	CallAtLatestBlock(common.Address, []byte) (CallResults, error)
	SystemCallAtBlockHash(blockHash common.Hash, contract common.Address, data []byte) (CallResults, error)
}

type ValidatorSet interface {
	// Signalling that a new epoch has begun.
	//
	// All calls here will be from the `SYSTEM_ADDRESS`: 2^160 - 2
	// and will have an effect on the block's state.
	// The caller provided here may not generate proofs.
	//
	// `first` is true if this is the first block in the set.
	onEpochBegin(firstInEpoch bool, header *types.Header, caller Syscall) error

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
	genesisEpochData(header *types.Header, call Syscall) ([]byte, error)

	/*
	 // Returns the current number of validators.
	    fn count(&self, parent: &H256) -> usize {
	        let default = self.default_caller(BlockId::Hash(*parent));
	        self.count_with_caller(parent, &*default)
	    }

	    // Signalling that a new epoch has begun.
	    //
	    // All calls here will be from the `SYSTEM_ADDRESS`: 2^160 - 2
	    // and will have an effect on the block's state.
	    // The caller provided here may not generate proofs.
	    //
	    // `first` is true if this is the first block in the set.
	    fn on_epoch_begin(
	        &self,
	        _first: bool,
	        _header: &Header,
	        _call: &mut SystemCall,
	    ) -> Result<(), ::error::Error> {
	        Ok(())
	    }

	    // Extract genesis epoch data from the genesis state and header.
	    fn genesis_epoch_data(&self, _header: &Header, _call: &Call) -> Result<Vec<u8>, String> {
	        Ok(Vec::new())
	    }
	*/
	// Whether this block is the last one in its epoch.
	//
	// Indicates that the validator set changed at the given block in a manner
	// that doesn't require finality.
	//
	// `first` is true if this is the first block in the set.
	signalEpochEnd(firstInEpoch bool, header *types.Header, receipts types.Receipts) ([]byte, error)
	/*
	   // Whether the given block signals the end of an epoch, but change won't take effect
	   // until finality.
	   //
	   // Engine should set `first` only if the header is genesis. Multiplexing validator
	   // sets can set `first` to internal changes.
	   fn signals_epoch_end(
	       &self,
	       first: bool,
	       header: &Header,
	       aux: AuxiliaryData,
	   ) -> ::engines::EpochChange<EthereumMachine>;

	   // Recover the validator set from the given proof, the block number, and
	   // whether this header is first in its set.
	   //
	   // May fail if the given header doesn't kick off an epoch or
	   // the proof is invalid.
	   //
	   // Returns the set, along with a flag indicating whether finality of a specific
	   // hash should be proven.
	   fn epoch_set(
	       &self,
	       first: bool,
	       machine: &EthereumMachine,
	       number: BlockNumber,
	       proof: &[u8],
	   ) -> Result<(SimpleList, Option<H256>), ::error::Error>;

	   // Checks if a given address is a validator, with the given function
	   // for executing synchronous calls to contracts.
	   fn contains_with_caller(
	       &self,
	       parent_block_hash: &H256,
	       address: &Address,
	       caller: &Call,
	   ) -> bool;

	   // Draws an validator nonce modulo number of validators.
	   fn get_with_caller(&self, parent_block_hash: &H256, nonce: usize, caller: &Call) -> Address;


	   // Notifies about malicious behaviour.
	   fn report_malicious(
	       &self,
	       _validator: &Address,
	       _set_block: BlockNumber,
	       _block: BlockNumber,
	       _proof: Bytes,
	   ) {
	   }
	   // Notifies about benign misbehaviour.
	   fn report_benign(&self, _validator: &Address, _set_block: BlockNumber, _block: BlockNumber) {}
	*/
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

// TODO: do we need add `proof` argument?
// nolint
func (s *Multi) epochSet(firstInEpoch bool, num uint64, proof []byte, evm *vm.EVM) (SimpleList, common.Hash, error) {
	setBlock, set := s.correctSetByNumber(num)
	firstInEpoch = setBlock == num
	return set.epochSet(firstInEpoch, num, proof, evm)
}
func (s *Multi) genesisEpochData(header *types.Header, call Syscall) ([]byte, error) {
	_, set := s.correctSetByNumber(0)
	return set.genesisEpochData(header, call)
}

func (s *Multi) onEpochBegin(_ bool, header *types.Header, caller Syscall) error {
	setTransition, set := s.correctSetByNumber(header.Number.Uint64())
	return set.onEpochBegin(setTransition == header.Number.Uint64(), header, caller)
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
func (s *SimpleList) onEpochBegin(bool, *types.Header, Syscall) error {
	return nil
}

func (s *SimpleList) genesisEpochData(*types.Header, Syscall) ([]byte, error) {
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
	validators      *lru.Cache[common.Hash, *SimpleList] // RwLock<MemoryLruCache<H256, SimpleList>>,
	// The block number where we resent the queued reports last time.
	resentReportsInBlock atomic.Uint64
	// If set, this is the block number at which the consensus engine switches from AuRa to AuRa
	// with POSDAO modifications.
	posdaoTransition *uint64

	abi    abi.ABI
	client client
}

func NewValidatorSafeContract(contractAddress common.Address, posdaoTransition *uint64, client client) *ValidatorSafeContract {
	const MemoizeCapacity = 500
	c, err := lru.New[common.Hash, *SimpleList](MemoizeCapacity)
	if err != nil {
		panic("error creating ValidatorSafeContract cache")
	}

	parsed, err := abi.JSON(strings.NewReader(auraabi.ValidatorSetABI))
	if err != nil {
		panic(err)
	}
	return &ValidatorSafeContract{contractAddress: contractAddress, posdaoTransition: posdaoTransition, validators: c, abi: parsed}
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

		//addresses, err := checkFirstValidatorSetProof(s.contractAddress, oldHeader, state_items)
		//if err != nil {
		//	panic(err)
		//	return SimpleList{}, common.Hash{}, fmt.Errorf("insufitient proof: block=%d,%x: %w", oldHeader.Number.Uint64(), oldHeader.Hash(), err)
		//}

		//fmt.Printf("aaaa: %x,%x\n", common.HexToAddress("0xe8ddc5c7a2d2f0d7a9798459c0104fdf5e987aca"), params.SokolGenesisHash)
		//fmt.Printf("bbbbb: %x,%x\n", proof.ContractAddress, proof.Header.Hash())
		return *l, proof.Header.ParentHash, nil
	}
	var proof ValidatorSetProof
	if err := rlp.DecodeBytes(setProof, &proof); err != nil {
		return SimpleList{}, common.Hash{}, fmt.Errorf("[ValidatorSafeContract.epochSet] %w", err)
	}

	if num > DEBUG_LOG_FROM {
		fmt.Printf("epoch_set1: %d,%d,%d\n", proof.Header.Number.Uint64(), len(setProof), len(proof.Receipts))
	}
	ll, ok := s.extractFromEvent(proof.Header, proof.Receipts)
	if !ok {
		panic(1)
	}

	// ensure receipts match header.
	// TODO: optimize? these were just decoded.
	/*
	   let found_root = ::triehash::ordered_trie_root(receipts.iter().map(|r| r.encode()));
	   if found_root != *old_header.receipts_root() {
	       return Err(::error::BlockError::InvalidReceiptsRoot(Mismatch {
	           expected: *old_header.receipts_root(),
	           found: found_root,
	       })
	       .into());
	   }

	   let bloom = self.expected_bloom(&old_header);

	   match self.extract_from_event(bloom, &old_header, &receipts) {
	       Some(list) => Ok((list, Some(old_header.hash()))),
	       None => Err(::engines::EngineError::InsufficientProof(
	           "No log event in proof.".into(),
	       )
	       .into()),
	   }
	*/
	return *ll, common.Hash{}, nil
	/*
		setProof, err := decodeValidatorSetProof(proof.SetProof)
		if err != nil {
			return SimpleList{}, common.Hash{}, err
		}
		_ = setProof
	*/
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
	out, _, err := evm.Call(params.SystemAddress, s.contractAddress, packed, vm.NewGasBudget(math.MaxUint64), new(uint256.Int))
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

func (s *ValidatorSafeContract) genesisEpochData(header *types.Header, call Syscall) ([]byte, error) {
	return rlp.EncodeToBytes(FirstValidatorSetProof{Header: header, ContractAddress: s.contractAddress})
}

func (s *ValidatorSafeContract) onEpochBegin(firstInEpoch bool, header *types.Header, caller Syscall) error {
	data := common.FromHex("75286211") // s.abi.Pack("finalizeChange")
	_, err := caller(s.contractAddress, data)
	if err != nil {
		return err
	}

	/*
	   let data = validator_set::functions::finalize_change::encode_input();
	   caller(self.contract_address, data)
	       .map(|_| ())
	       .map_err(::engines::EngineError::FailedSystemCall)
	       .map_err(Into::into)
	*/
	return nil
}

func (s *ValidatorSafeContract) signalEpochEnd(firstInEpoch bool, header *types.Header, r types.Receipts) ([]byte, error) {
	// transition to the first block of a contract requires finality but has no log event.
	if firstInEpoch {
		/*
		   let state_proof = Arc::new(FirstValidatorSetProof {
		       contract_address: self.contract_address,
		       header: header.clone(),
		   });
		   return ::engines::EpochChange::Yes(::engines::Proof::WithState(state_proof as Arc<_>));
		*/
		return rlp.EncodeToBytes(FirstValidatorSetProof{Header: header, ContractAddress: s.contractAddress})
	}

	// otherwise, we're checking for logs.
	//let bloom = self.expected_bloom(header);
	//let header_bloom = header.log_bloom();
	//if &bloom & header_bloom != bloom {
	//	return ::engines::EpochChange::No;
	//}

	_, ok := s.extractFromEvent(header, r)
	if !ok {
		return nil, nil
	}
	proof, err := rlp.EncodeToBytes(ValidatorSetProof{Header: header, Receipts: r})
	if err != nil {
		return nil, err
	}
	if header.Number.Uint64() >= DEBUG_LOG_FROM {
		fmt.Printf("signalEpochEnd: %d,%d, proofLen=%d\n", header.Number.Uint64(), len(r), len(proof))
	}
	return proof, nil
}

func (s *ValidatorSafeContract) extractFromEvent(header *types.Header, receipts types.Receipts) (*SimpleList, bool) {
	if len(receipts) == 0 {
		if header.Number.Uint64() >= DEBUG_LOG_FROM {
			fmt.Printf("extractFromEvent1: %d\n", header.Number.Uint64())
		}
		return nil, false
	}
	if header.Number.Uint64() >= DEBUG_LOG_FROM {
		fmt.Printf("extractFromEvent111: %d,%d\n", header.Number.Uint64(), len(receipts))
	}

	// iterate in reverse because only the _last_ change in a given
	// block actually has any effect.
	// the contract should only increment the nonce once.
	for j := len(receipts) - 1; j >= 0; j-- {
		logs := receipts[j].Logs
		/*
			TODO: skipped next bloom check (is it required?)
					expectedBloom := expected_bloom(&self, header: &Header) -> Bloom {
				        let topics = vec![*EVENT_NAME_HASH, *header.parent_hash()];

				        debug!(target: "engine", "Expected topics for header {}: {:?}",
							header.hash(), topics);

				        LogEntry {
				            address: self.contract_address,
				            topics: topics,
				            data: Vec::new(), // irrelevant for bloom.
				        }
				        .bloom()
				    }
					if !r.log_bloom.contains_bloom(&bloom){
						continue
					}
		*/
		for i := 0; i < len(logs); i++ {
			l := logs[i]
			if header.Number.Uint64() >= DEBUG_LOG_FROM {
				fmt.Printf("extractFromEvent3: %d\n", header.Number.Uint64())
			}
			if len(l.Topics) != 2 {
				continue
			}
			found := l.Address == s.contractAddress && l.Topics[0] == EVENT_NAME_HASH && l.Topics[1] == header.ParentHash
			if !found {
				if header.Number.Uint64() >= DEBUG_LOG_FROM {
					fmt.Printf("extractFromEvent4: %d\n", header.Number.Uint64())
				}
				continue
			}

			contract := bind.NewBoundContract(l.Address, s.abi, nil, nil, nil)
			event := new(auraabi.ValidatorSetInitiateChange)
			if err := contract.UnpackLog(event, "InitiateChange", *l); err != nil {
				panic(err)
			}
			if header.Number.Uint64() >= DEBUG_LOG_FROM {
				fmt.Printf("extractFromEvent5: %d\n", header.Number.Uint64())
			}

			// only one last log is taken into account
			return NewSimpleList(event.NewSet), true
		}
	}
	/*
					  let check_log = |log: &LogEntry| {
		            log.address == self.contract_address
		                && log.topics.len() == 2
		                && log.topics[0] == *EVENT_NAME_HASH
		                && log.topics[1] == *header.parent_hash()
		        };

		        //// iterate in reverse because only the _last_ change in a given
		        //// block actually has any effect.
		        //// the contract should only increment the nonce once.
		        let mut decoded_events = receipts
		            .iter()
		            .rev()
		            .filter(|r| r.log_bloom.contains_bloom(&bloom))
		            .flat_map(|r| r.logs.iter())
		            .filter(move |l| check_log(l))
		            .filter_map(|log| {
		                validator_set::events::initiate_change::parse_log(
		                    (log.topics.clone(), log.data.clone()).into(),
		                )
		                .ok()
		            });

		        // only last log is taken into account
		        decoded_events.next().map(|matched_event| {
		            let l = SimpleList::new(matched_event.new_set);
		            println!("matched_event: {:?}", l);
		            l
		        })
	*/
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

func (s *ValidatorContract) onEpochBegin(firstInEpoch bool, header *types.Header, caller Syscall) error {
	return s.validators.onEpochBegin(firstInEpoch, header, caller)
}
func (s *ValidatorContract) genesisEpochData(header *types.Header, call Syscall) ([]byte, error) {
	return s.validators.genesisEpochData(header, call)
}
func (s *ValidatorContract) signalEpochEnd(firstInEpoch bool, header *types.Header, r types.Receipts) ([]byte, error) {
	return s.validators.signalEpochEnd(firstInEpoch, header, r)
}
