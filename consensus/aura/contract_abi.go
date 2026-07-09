package aura

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/aura/contracts"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
)

func callBlockRewardAbi(contractAddr common.Address, evm *vm.EVM, beneficiaries []common.Address, rewardKind []consensus.RewardKind) ([]common.Address, []*big.Int) {
	castedKind := make([]uint16, len(rewardKind))
	for i := range rewardKind {
		castedKind[i] = uint16(rewardKind[i])
	}
	packed, err := blockRewardAbi().Pack("reward", beneficiaries, castedKind)
	if err != nil {
		panic(err)
	}
	// TODO: This was done purely to get hive tests passing on no contract
	// test. Discuss whether we change the code here or change the test behavior.
	if evm.StateDB.GetCodeSize(contractAddr) == 0 {
		return nil, nil
	}
	out, err := systemCall(evm, contractAddr, packed)
	if err != nil {
		panic(err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	res, err := blockRewardAbi().Unpack("reward", out)
	if err != nil {
		panic(err)
	}
	beneficiariesRes := res[0].([]common.Address)
	rewardsBig := res[1].([]*big.Int)
	// rewardsU256 := make([]*big.Int, len(rewardsBig))
	// for i := 0; i < len(rewardsBig); i++ {
	// 	var overflow bool
	// 	rewards[i], overflow = uint256.FromBig(rewardsBig[i])
	// 	if overflow {
	// 		panic("Overflow in callBlockRewardAbi")
	// 	}
	// }
	return beneficiariesRes, rewardsBig
}

func blockRewardAbi() abi.ABI {
	a, err := abi.JSON(bytes.NewReader(contracts.BlockReward))
	if err != nil {
		panic(err)
	}
	return a
}

func certifierAbi() abi.ABI {
	a, err := abi.JSON(bytes.NewReader(contracts.Certifier))
	if err != nil {
		panic(err)
	}
	return a
}

func registrarAbi() abi.ABI {
	a, err := abi.JSON(bytes.NewReader(contracts.Registrar))
	if err != nil {
		panic(err)
	}
	return a
}

func withdrawalAbi() abi.ABI {
	a, err := abi.JSON(bytes.NewReader(contracts.Withdrawal))
	if err != nil {
		panic(err)
	}
	return a
}

func getCertifier(registrar common.Address, evm *vm.EVM) *common.Address {
	hashedKey := crypto.Keccak256Hash([]byte("service_transaction_checker"))
	packed, err := registrarAbi().Pack("getAddress", hashedKey, "A")
	if err != nil {
		panic(err)
	}
	out, err := systemCall(evm, registrar, packed)
	if err != nil {
		panic(err)
	}
	if len(out) == 0 {
		return nil
	}
	res, err := registrarAbi().Unpack("getAddress", out)
	if err != nil {
		panic(err)
	}
	certifier := res[0].(common.Address)
	return &certifier
}
