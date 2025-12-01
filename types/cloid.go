package types

import (
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/vmihailenco/msgpack/v5"
)

const cloidLength = 16

type Cloid [cloidLength]byte

var cloidT = reflect.TypeFor[Cloid]()

// BytesToCloid returns Cloid with value b.
// If b is larger than len(c), b will be cropped from the left.
func BytesToCloid(b []byte) Cloid {
	var c Cloid
	c.SetBytes(b)
	return c
}

// HexToCloid returns Cloid with byte values of s.
// If s is larger than len(c), s will be cropped from the left.
func HexToCloid(s string) Cloid {
	return BytesToCloid(common.FromHex(s))
}

// BigToHash sets byte representation of b to cloid.
// If b is larger than len(h), b will be cropped from the left.
func BigToCloid(b *big.Int) Cloid {
	return BytesToCloid(b.Bytes())
}

// SetBytes sets the Cloid to the value of b.
// If b is larger than len(c), b will be cropped from the left.
func (c *Cloid) SetBytes(b []byte) {
	if len(b) > len(c) {
		b = b[len(b)-cloidLength:]
	}

	copy(c[cloidLength-len(b):], b)
}

// Hex converts a Cloid to a hex string.
func (c Cloid) Hex() string { return hexutil.Encode(c[:]) }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (c Cloid) String() string {
	return c.Hex()
}

// UnmarshalJSON parses a Cloid in hex syntax.
func (c *Cloid) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalFixedJSON(cloidT, input, c[:])
}

// MarshalText returns the hex representation of c.
func (c Cloid) MarshalText() ([]byte, error) {
	return hexutil.Bytes(c[:]).MarshalText()
}

func (c Cloid) EncodeMsgpack(enc *msgpack.Encoder) error {
	// Encode as a MessagePack string → will use str8 for this size
	return enc.EncodeString(c.Hex())
}

func (c *Cloid) DecodeMsgpack(dec *msgpack.Decoder) error {
	s, err := dec.DecodeString()
	if err != nil {
		return err
	}

	// Parse back from hex string (e.g. "0x0000...")
	*c = HexToCloid(s)
	return nil
}
