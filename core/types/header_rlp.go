// Copyright 2026 The go-ethereum Authors
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

package types

import (
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

func (obj *Header) EncodeRLP(_w io.Writer) error {
	w := rlp.NewEncoderBuffer(_w)
	_tmp0 := w.List()
	w.WriteBytes(obj.ParentHash[:])
	w.WriteBytes(obj.UncleHash[:])
	w.WriteBytes(obj.Coinbase[:])
	w.WriteBytes(obj.Root[:])
	w.WriteBytes(obj.TxHash[:])
	w.WriteBytes(obj.ReceiptHash[:])
	w.WriteBytes(obj.Bloom[:])
	if obj.Difficulty == nil {
		w.Write(rlp.EmptyString)
	} else {
		if obj.Difficulty.Sign() == -1 {
			return rlp.ErrNegativeBigInt
		}
		w.WriteBigInt(obj.Difficulty)
	}
	if obj.Number == nil {
		w.Write(rlp.EmptyString)
	} else {
		if obj.Number.Sign() == -1 {
			return rlp.ErrNegativeBigInt
		}
		w.WriteBigInt(obj.Number)
	}
	w.WriteUint64(obj.GasLimit)
	w.WriteUint64(obj.GasUsed)
	w.WriteUint64(obj.Time)
	w.WriteBytes(obj.Extra)
	if len(obj.Signature) == 0 {
		w.WriteBytes(obj.MixDigest[:])
		w.WriteBytes(obj.Nonce[:])
	} else {
		w.WriteUint64(obj.Step)
		w.WriteBytes(obj.Signature)
	}
	_tmp1 := obj.BaseFee != nil
	_tmp2 := obj.WithdrawalsHash != nil
	_tmp3 := obj.BlobGasUsed != nil
	_tmp4 := obj.ExcessBlobGas != nil
	_tmp5 := obj.ParentBeaconRoot != nil
	_tmp6 := obj.RequestsHash != nil
	_tmp7 := obj.SlotNumber != nil
	if _tmp1 || _tmp2 || _tmp3 || _tmp4 || _tmp5 || _tmp6 || _tmp7 {
		if obj.BaseFee == nil {
			w.Write(rlp.EmptyString)
		} else {
			if obj.BaseFee.Sign() == -1 {
				return rlp.ErrNegativeBigInt
			}
			w.WriteBigInt(obj.BaseFee)
		}
	}
	if _tmp2 || _tmp3 || _tmp4 || _tmp5 || _tmp6 || _tmp7 {
		if obj.WithdrawalsHash == nil {
			w.Write([]byte{0x80})
		} else {
			w.WriteBytes(obj.WithdrawalsHash[:])
		}
	}
	if _tmp3 || _tmp4 || _tmp5 || _tmp6 || _tmp7 {
		if obj.BlobGasUsed == nil {
			w.Write([]byte{0x80})
		} else {
			w.WriteUint64((*obj.BlobGasUsed))
		}
	}
	if _tmp4 || _tmp5 || _tmp6 || _tmp7 {
		if obj.ExcessBlobGas == nil {
			w.Write([]byte{0x80})
		} else {
			w.WriteUint64((*obj.ExcessBlobGas))
		}
	}
	if _tmp5 || _tmp6 || _tmp7 {
		if obj.ParentBeaconRoot == nil {
			w.Write([]byte{0x80})
		} else {
			w.WriteBytes(obj.ParentBeaconRoot[:])
		}
	}
	if _tmp6 || _tmp7 {
		if obj.RequestsHash == nil {
			w.Write([]byte{0x80})
		} else {
			w.WriteBytes(obj.RequestsHash[:])
		}
	}
	if _tmp7 {
		if obj.SlotNumber == nil {
			w.Write([]byte{0x80})
		} else {
			w.WriteUint64((*obj.SlotNumber))
		}
	}
	w.ListEnd(_tmp0)
	return w.Flush()
}

func (obj *Header) DecodeRLP(s *rlp.Stream) error {
	_, err := s.List()
	if err != nil {
		return err
	}
	var b []byte
	if b, err = s.Bytes(); err != nil {
		return fmt.Errorf("read ParentHash: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for ParentHash: %d", len(b))
	}
	copy(obj.ParentHash[:], b)
	if b, err = s.Bytes(); err != nil {
		return fmt.Errorf("read UncleHash: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for UncleHash: %d", len(b))
	}
	copy(obj.UncleHash[:], b)
	if b, err = s.Bytes(); err != nil {
		return fmt.Errorf("read Coinbase: %w", err)
	}
	if len(b) != 20 {
		return fmt.Errorf("wrong size for Coinbase: %d", len(b))
	}
	copy(obj.Coinbase[:], b)
	if b, err = s.Bytes(); err != nil {
		return fmt.Errorf("read Root: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for Root: %d", len(b))
	}
	copy(obj.Root[:], b)
	if b, err = s.Bytes(); err != nil {
		return fmt.Errorf("read TxHash: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for TxHash: %d", len(b))
	}
	copy(obj.TxHash[:], b)
	if b, err = s.Bytes(); err != nil {
		return fmt.Errorf("read ReceiptHash: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for ReceiptHash: %d", len(b))
	}
	copy(obj.ReceiptHash[:], b)
	if b, err = s.Bytes(); err != nil {
		return fmt.Errorf("read Bloom: %w", err)
	}
	if len(b) != 256 {
		return fmt.Errorf("wrong size for Bloom: %d", len(b))
	}
	copy(obj.Bloom[:], b)
	if obj.Difficulty, err = s.BigInt(); err != nil {
		return fmt.Errorf("read Difficulty: %w", err)
	}
	if obj.Number, err = s.BigInt(); err != nil {
		return fmt.Errorf("read Number: %w", err)
	}
	if obj.GasLimit, err = s.Uint64(); err != nil {
		return fmt.Errorf("read GasLimit: %w", err)
	}
	if obj.GasUsed, err = s.Uint64(); err != nil {
		return fmt.Errorf("read GasUsed: %w", err)
	}
	if obj.Time, err = s.Uint64(); err != nil {
		return fmt.Errorf("read Time: %w", err)
	}
	if obj.Extra, err = s.Bytes(); err != nil {
		return fmt.Errorf("read Extra: %w", err)
	}

	_, size, err := s.Kind()
	if err != nil {
		return fmt.Errorf("read MixDigest: %w", err)
	}
	if size != 32 { // AuRa
		if obj.Step, err = s.Uint64(); err != nil {
			return fmt.Errorf("read AuRaStep: %w", err)
		}
		if obj.Signature, err = s.Bytes(); err != nil {
			return fmt.Errorf("read AuRaSeal: %w", err)
		}
	} else {
		if b, err = s.Bytes(); err != nil {
			return fmt.Errorf("read MixDigest: %w", err)
		}
		copy(obj.MixDigest[:], b)
		if b, err = s.Bytes(); err != nil {
			return fmt.Errorf("read Nonce: %w", err)
		}
		if len(b) != 8 {
			return fmt.Errorf("wrong size for Nonce: %d", len(b))
		}
		copy(obj.Nonce[:], b)
	}

	// BaseFee
	if obj.BaseFee, err = s.BigInt(); err != nil {
		if errors.Is(err, rlp.EOL) {
			obj.BaseFee = nil
			if err := s.ListEnd(); err != nil {
				return fmt.Errorf("close header struct (no BaseFee): %w", err)
			}
			return nil
		}
		return fmt.Errorf("read BaseFee: %w", err)
	}

	// WithdrawalsHash
	if b, err = s.Bytes(); err != nil {
		if errors.Is(err, rlp.EOL) {
			obj.WithdrawalsHash = nil
			if err := s.ListEnd(); err != nil {
				return fmt.Errorf("close header struct (no WithdrawalsHash): %w", err)
			}
			return nil
		}
		return fmt.Errorf("read WithdrawalsHash: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for WithdrawalsHash: %d", len(b))
	}
	obj.WithdrawalsHash = new(common.Hash)
	obj.WithdrawalsHash.SetBytes(b)

	var blobGasUsed uint64
	if blobGasUsed, err = s.Uint64(); err != nil {
		if errors.Is(err, rlp.EOL) {
			obj.BlobGasUsed = nil
			if err := s.ListEnd(); err != nil {
				return fmt.Errorf("close header struct (no BlobGasUsed): %w", err)
			}
			return nil
		}
		return fmt.Errorf("read BlobGasUsed: %w", err)
	}
	obj.BlobGasUsed = &blobGasUsed

	var excessBlobGas uint64
	if excessBlobGas, err = s.Uint64(); err != nil {
		if errors.Is(err, rlp.EOL) {
			obj.ExcessBlobGas = nil
			if err := s.ListEnd(); err != nil {
				return fmt.Errorf("close header struct (no ExcessBlobGas): %w", err)
			}
			return nil
		}
		return fmt.Errorf("read ExcessBlobGas: %w", err)
	}
	obj.ExcessBlobGas = &excessBlobGas

	// ParentBeaconBlockRoot
	if b, err = s.Bytes(); err != nil {
		if errors.Is(err, rlp.EOL) {
			obj.ParentBeaconRoot = nil
			if err := s.ListEnd(); err != nil {
				return fmt.Errorf("close header struct (no ParentBeaconBlockRoot): %w", err)
			}
			return nil
		}
		return fmt.Errorf("read ParentBeaconBlockRoot: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for ParentBeaconBlockRoot: %d", len(b))
	}
	obj.ParentBeaconRoot = new(common.Hash)
	obj.ParentBeaconRoot.SetBytes(b)

	// RequestsHash
	if b, err = s.Bytes(); err != nil {
		if errors.Is(err, rlp.EOL) {
			obj.RequestsHash = nil
			if err := s.ListEnd(); err != nil {
				return fmt.Errorf("close header struct (no RequestsHash): %w", err)
			}
			return nil
		}
		return fmt.Errorf("read RequestsHash: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("wrong size for RequestsHash: %d", len(b))
	}
	obj.RequestsHash = new(common.Hash)
	obj.RequestsHash.SetBytes(b)

	// SlotNumber
	var slotNumber uint64
	if slotNumber, err = s.Uint64(); err != nil {
		if errors.Is(err, rlp.EOL) {
			obj.SlotNumber = nil
			if err := s.ListEnd(); err != nil {
				return fmt.Errorf("close header struct (no SlotNumber): %w", err)
			}
			return nil
		}
		return fmt.Errorf("read SlotNumber: %w", err)
	}
	obj.SlotNumber = &slotNumber

	if err := s.ListEnd(); err != nil {
		return fmt.Errorf("close header struct: %w", err)
	}
	return nil
}
