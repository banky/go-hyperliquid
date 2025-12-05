package exchange

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/vmihailenco/msgpack/v5"
)

type signature struct {
	R common.Hash
	S common.Hash
	V byte
}

// MarshalJSON encodes the signature as:
// { "r": "0x...", "s": "0x...", "v": <number> }
func (s signature) MarshalJSON() ([]byte, error) {
	type alias struct {
		R string `json:"r"`
		S string `json:"s"`
		V uint8  `json:"v"`
	}

	a := alias{
		R: hexutil.Encode(s.R[:]),
		S: hexutil.Encode(s.S[:]),
		V: uint8(s.V),
	}

	return json.Marshal(a)
}

var _ msgpack.CustomEncoder = (*signature)(nil)

func (s *signature) EncodeMsgpack(enc *msgpack.Encoder) error {
	type alias struct {
		R string `msgpack:"r"`
		S string `msgpack:"s"`
		V uint8  `msgpack:"v"`
	}

	a := alias{
		R: hexutil.Encode(s.R[:]),
		S: hexutil.Encode(s.S[:]),
		V: uint8(s.V),
	}

	return enc.Encode(a)
}

// UnmarshalJSON decodes from:
// { "r": "0x...", "s": "0x...", "v": <number> }
func (s *signature) UnmarshalJSON(data []byte) error {
	type alias struct {
		R string `json:"r"`
		S string `json:"s"`
		V uint8  `json:"v"`
	}

	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	// Decode R
	rBytes, err := hexutil.Decode(a.R)
	if err != nil {
		return fmt.Errorf("invalid r: %w", err)
	}
	if len(rBytes) != len(s.R) {
		return fmt.Errorf(
			"invalid r length: got %d, want %d",
			len(rBytes),
			len(s.R),
		)
	}
	copy(s.R[:], rBytes)

	// Decode S
	sBytes, err := hexutil.Decode(a.S)
	if err != nil {
		return fmt.Errorf("invalid s: %w", err)
	}
	if len(sBytes) != len(s.S) {
		return fmt.Errorf(
			"invalid s length: got %d, want %d",
			len(sBytes),
			len(s.S),
		)
	}
	copy(s.S[:], sBytes)

	// V
	s.V = byte(a.V)

	return nil
}

func (s signature) String() string {
	return fmt.Sprintf(
		"R: %s, S: %s, V: %d",
		hexutil.Encode(s.R[:]),
		hexutil.Encode(s.S[:]),
		s.V,
	)
}
