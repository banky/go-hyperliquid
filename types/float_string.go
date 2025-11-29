package types

import (
	"encoding/json"

	"github.com/banky/go-hyperliquid/internal/utils"
)

// FloatString represents a floating-point number that can be encoded as a JSON
// string or number
type FloatString float64

// UnmarshalJSON implements json.Unmarshaler for FloatString
func (f *FloatString) UnmarshalJSON(b []byte) error {
	// Handle "null"
	if string(b) == "null" {
		*f = 0
		return nil
	}

	// Remove quotes if needed and parse as string
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, err := utils.StringToFloat(s)
		if err != nil {
			return err
		}
		*f = FloatString(v)
		return nil
	}

	// Otherwise fall back to normal float unmarshal
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = FloatString(v)
	return nil
}

func (f FloatString) String() string {
	s, _ := utils.FloatToWire(f.Raw())
	return s
}

func (f FloatString) Raw() float64 {
	return float64(f)
}
