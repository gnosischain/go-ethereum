// Copyright 2025 The go-ethereum Authors
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

package txpool

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// gnosisTestConfig returns a chain config with Cancun enabled and Gnosis blob parameters.
func gnosisTestConfig() *params.ChainConfig {
	return &params.ChainConfig{
		ChainID:                 big.NewInt(100),
		HomesteadBlock:          big.NewInt(0),
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0),
		ShanghaiTime:            newUint64(0),
		CancunTime:              newUint64(0),
		TerminalTotalDifficulty: big.NewInt(0),
		BlobScheduleConfig:      params.GnosisChainConfig.BlobScheduleConfig,
		MinBlobGasPrice:         newUint64(params.GnosisBlobTxMinBlobGasprice),
		MaxBlobsPerTransaction:  newInt(params.GnosisBlobTxMaxBlobs),
	}
}

func newUint64(n uint64) *uint64 { return &n }
func newInt(n int) *int          { return &n }

func TestValidateTransactionEIP2681(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   5000000,
		Time:       1,
		Difficulty: big.NewInt(1),
	}

	signer := types.LatestSigner(params.TestChainConfig)

	// Create validation options
	opts := &ValidationOptions{
		Config:       params.TestChainConfig,
		Accept:       0xFF, // Accept all transaction types
		MaxSize:      32 * 1024,
		MaxBlobCount: 6,
		MinTip:       big.NewInt(0),
	}

	tests := []struct {
		name    string
		nonce   uint64
		wantErr error
	}{
		{
			name:    "normal nonce",
			nonce:   42,
			wantErr: nil,
		},
		{
			name:    "max allowed nonce (2^64-2)",
			nonce:   math.MaxUint64 - 1,
			wantErr: nil,
		},
		{
			name:    "EIP-2681 nonce overflow (2^64-1)",
			nonce:   math.MaxUint64,
			wantErr: core.ErrNonceMax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction(key, tt.nonce)
			err := ValidateTransaction(tx, head, signer, opts)

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateTransaction() error = %v, wantErr nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidateTransaction() error = nil, wantErr %v", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateTransaction() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidateBlobTxGnosisParams(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	gnosisConfig := gnosisTestConfig()
	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   30000000,
		Time:       1,
		Difficulty: big.NewInt(0),
	}
	signer := types.LatestSigner(gnosisConfig)

	tests := []struct {
		name       string
		blobCount  int
		blobFeeCap uint64
		wantErr    string
	}{
		{
			name:       "gnosis: 2 blobs with sufficient fee accepted",
			blobCount:  2,
			blobFeeCap: 1000000000,
			wantErr:    "",
		},
		{
			name:       "gnosis: 3 blobs rejected",
			blobCount:  3,
			blobFeeCap: 1000000000,
			wantErr:    "too many blobs in transaction: have 3, permitted 2",
		},
		{
			name:       "gnosis: fee cap below minimum rejected",
			blobCount:  1,
			blobFeeCap: 999999999,
			wantErr:    "blob fee cap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createBlobTx(key, gnosisConfig, tt.blobCount, tt.blobFeeCap)
			opts := &ValidationOptions{
				Config:       gnosisConfig,
				Accept:       1 << types.BlobTxType,
				MaxSize:      1024 * 1024,
				MaxBlobCount: 6, // pool-level limit; chain-level limit is stricter
				MinTip:       big.NewInt(0),
			}
			err := ValidateTransaction(tx, head, signer, opts)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// createBlobTx creates a blob transaction with the given number of blobs and fee cap.
func createBlobTx(key *ecdsa.PrivateKey, config *params.ChainConfig, blobCount int, blobFeeCap uint64) *types.Transaction {
	var (
		blobs       []kzg4844.Blob
		commitments []kzg4844.Commitment
		proofs      []kzg4844.Proof
		hashes      []common.Hash
	)
	for i := 0; i < blobCount; i++ {
		var blob kzg4844.Blob
		blob[0] = byte(i + 1) // non-zero so each blob is unique
		commit, _ := kzg4844.BlobToCommitment(&blob)
		proof, _ := kzg4844.ComputeBlobProof(&blob, commit)
		blobs = append(blobs, blob)
		commitments = append(commitments, commit)
		proofs = append(proofs, proof)
		hashes = append(hashes, kzg4844.CalcBlobHashV1(sha256.New(), &commit))
	}
	sidecar := types.NewBlobTxSidecar(types.BlobSidecarVersion0, blobs, commitments, proofs)
	blobtx := &types.BlobTx{
		ChainID:    uint256.MustFromBig(config.ChainID),
		Nonce:      0,
		GasTipCap:  uint256.NewInt(1),
		GasFeeCap:  uint256.NewInt(1000000000),
		Gas:        21000,
		BlobFeeCap: uint256.NewInt(blobFeeCap),
		BlobHashes: hashes,
		Value:      uint256.NewInt(0),
		Sidecar:    sidecar,
	}
	return types.MustSignNewTx(key, types.LatestSigner(config), blobtx)
}

// createTestTransaction creates a basic transaction for testing
func createTestTransaction(key *ecdsa.PrivateKey, nonce uint64) *types.Transaction {
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")

	txdata := &types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    big.NewInt(1000),
		Gas:      21000,
		GasPrice: big.NewInt(1),
		Data:     nil,
	}

	tx := types.NewTx(txdata)
	signedTx, _ := types.SignTx(tx, types.HomesteadSigner{}, key)
	return signedTx
}
