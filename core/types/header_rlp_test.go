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
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

// autoFillHeader uses reflection to set every Header field to a distinct
// non-zero value. It panics if any field remains zero after filling, which
// means fillField needs to be updated to handle the new type.
func autoFillHeader(t *testing.T) *Header {
	t.Helper()
	h := new(Header)
	v := reflect.ValueOf(h).Elem()
	typ := v.Type()

	for i := 0; i < typ.NumField(); i++ {
		fillField(v.Field(i), byte(i+1))
		require.Falsef(t, v.Field(i).IsZero(),
			"autoFillHeader did not set field %s (type %s); update fillField to handle this type",
			typ.Field(i).Name, v.Field(i).Type())
	}
	return h
}

func fillField(field reflect.Value, seed byte) {
	switch field.Type() {
	case reflect.TypeOf(common.Hash{}):
		var h common.Hash
		for j := range h {
			h[j] = seed
		}
		field.Set(reflect.ValueOf(h))
	case reflect.TypeOf(common.Address{}):
		var a common.Address
		for j := range a {
			a[j] = seed
		}
		field.Set(reflect.ValueOf(a))
	case reflect.TypeOf(Bloom{}):
		var b Bloom
		b[0] = seed
		field.Set(reflect.ValueOf(b))
	case reflect.TypeOf(BlockNonce{}):
		var n BlockNonce
		n[0] = seed
		field.Set(reflect.ValueOf(n))
	default:
		switch field.Kind() {
		case reflect.Uint64:
			field.SetUint(uint64(seed))
		case reflect.Slice:
			if field.Type().Elem().Kind() == reflect.Uint8 { // []byte
				field.SetBytes([]byte{seed, seed + 1, seed + 2})
			}
		case reflect.Ptr:
			elem := reflect.New(field.Type().Elem())
			switch field.Type().Elem() {
			case reflect.TypeOf(big.Int{}):
				elem.Interface().(*big.Int).SetUint64(uint64(seed))
			case reflect.TypeOf(common.Hash{}):
				h := elem.Interface().(*common.Hash)
				for j := range h {
					h[j] = seed
				}
			case reflect.TypeOf(uint64(0)):
				*elem.Interface().(*uint64) = uint64(seed)
			}
			field.Set(elem)
		}
	}
}

// TestHeaderRLPAutoFields uses reflection to populate every Header field,
// then verifies encode→decode round-trip fidelity. If a new field is added
// to Header but not to EncodeRLP/DecodeRLP, the decoded header will have a
// zero value where the original had non-zero, and this test will fail.
//
// Two subtests cover both encoding paths:
//   - Ethash: MixDigest + Nonce (Signature empty)
//   - AuRa:   Step + Signature  (MixDigest/Nonce zeroed)
func TestHeaderRLPAutoFields(t *testing.T) {
	t.Run("Ethash", func(t *testing.T) {
		h := autoFillHeader(t)

		// Ethash path: Signature must be empty, Step is unused.
		h.Step = 0
		h.Signature = nil

		enc, err := rlp.EncodeToBytes(h)
		require.NoError(t, err, "encode")

		var decoded Header
		require.NoError(t, rlp.DecodeBytes(enc, &decoded), "decode")
		require.Equal(t, h, &decoded, "Ethash round-trip mismatch")
	})

	t.Run("AuRa", func(t *testing.T) {
		h := autoFillHeader(t)
		// AuRa path: MixDigest and Nonce are unused.
		h.MixDigest = common.Hash{}
		h.Nonce = BlockNonce{}

		enc, err := rlp.EncodeToBytes(h)
		require.NoError(t, err, "encode")

		var decoded Header
		require.NoError(t, rlp.DecodeBytes(enc, &decoded), "decode")
		require.Equal(t, h, &decoded, "AuRa round-trip mismatch")
	})
}

// TestHeaderRLPRoundTripAllForks tests that headers at each fork boundary
// (progressively more optional fields) survive encode→decode round trips.
func TestHeaderRLPRoundTripAllForks(t *testing.T) {
	base := func() *Header {
		return &Header{
			ParentHash:  common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
			UncleHash:   common.HexToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"),
			Coinbase:    common.HexToAddress("0x8888f1f195afa192cfee860698584c030f4c9db1"),
			Root:        common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"),
			TxHash:      common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333"),
			ReceiptHash: common.HexToHash("0x4444444444444444444444444444444444444444444444444444444444444444"),
			Difficulty:  big.NewInt(131072),
			Number:      big.NewInt(100),
			GasLimit:    3141592,
			GasUsed:     21000,
			Time:        1426516743,
			Extra:       []byte("test"),
			MixDigest:   common.HexToHash("0x5555555555555555555555555555555555555555555555555555555555555555"),
			Nonce:       EncodeNonce(0xa13a5a8c8f2bb1c4),
		}
	}

	wh := common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	pbr := common.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	rh := common.HexToHash("0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	balh := common.HexToHash("0xdddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	blobGasUsed := uint64(131072)
	excessBlobGas := uint64(262144)
	slotNumber := uint64(42)

	tests := []struct {
		name   string
		modify func(h *Header)
	}{
		{"legacy (no optional fields)", func(h *Header) {}},
		{"EIP-1559 (BaseFee)", func(h *Header) {
			h.BaseFee = big.NewInt(1000000000)
		}},
		{"Shanghai (WithdrawalsHash)", func(h *Header) {
			h.BaseFee = big.NewInt(1000000000)
			h.WithdrawalsHash = &wh
		}},
		{"Cancun (BlobGas + BeaconRoot)", func(h *Header) {
			h.BaseFee = big.NewInt(1000000000)
			h.WithdrawalsHash = &wh
			h.BlobGasUsed = &blobGasUsed
			h.ExcessBlobGas = &excessBlobGas
			h.ParentBeaconRoot = &pbr
		}},
		{"Prague (RequestsHash)", func(h *Header) {
			h.BaseFee = big.NewInt(1000000000)
			h.WithdrawalsHash = &wh
			h.BlobGasUsed = &blobGasUsed
			h.ExcessBlobGas = &excessBlobGas
			h.ParentBeaconRoot = &pbr
			h.RequestsHash = &rh
		}},
		{"EIP-7928 (BlockAccessListHash)", func(h *Header) {
			h.BaseFee = big.NewInt(1000000000)
			h.WithdrawalsHash = &wh
			h.BlobGasUsed = &blobGasUsed
			h.ExcessBlobGas = &excessBlobGas
			h.ParentBeaconRoot = &pbr
			h.RequestsHash = &rh
			h.BlockAccessListHash = &balh
		}},
		{"EIP-7843 (SlotNumber)", func(h *Header) {
			h.BaseFee = big.NewInt(1000000000)
			h.WithdrawalsHash = &wh
			h.BlobGasUsed = &blobGasUsed
			h.ExcessBlobGas = &excessBlobGas
			h.ParentBeaconRoot = &pbr
			h.RequestsHash = &rh
			h.BlockAccessListHash = &balh
			h.SlotNumber = &slotNumber
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := base()
			tt.modify(orig)

			enc, err := rlp.EncodeToBytes(orig)
			require.NoError(t, err, "encode")

			var decoded Header
			require.NoError(t, rlp.DecodeBytes(enc, &decoded), "decode")
			require.Equal(t, orig, &decoded, "round-trip mismatch")

			// Re-encode and verify byte-level equality.
			reenc, err := rlp.EncodeToBytes(&decoded)
			require.NoError(t, err, "re-encode")
			require.Equal(t, enc, reenc, "re-encoded bytes differ from original encoding")
		})
	}
}
