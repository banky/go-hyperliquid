package utils

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// floatToWire converts a float64 to wire format (8 decimal string)
// This matches the Python SDK's float_to_wire function for consistent precision
func FloatToWire(x float64) (string, error) {
	// Handle NaN and infinity
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return "", fmt.Errorf("invalid float value: %v", x)
	}

	// Round to 8 decimal places
	rounded := math.Round(x*1e8) / 1e8

	// Validate rounding precision (tolerance of 1e-12)
	if math.Abs(x-rounded) > 1e-12 {
		return "", fmt.Errorf(
			"float precision loss: %v rounds to %v",
			x,
			rounded,
		)
	}

	// Format to 8 decimal places and normalize
	formatted := strconv.FormatFloat(rounded, 'f', 8, 64)

	// Remove trailing zeros after decimal point
	if strings.Contains(formatted, ".") {
		formatted = strings.TrimRight(formatted, "0")
		formatted = strings.TrimRight(formatted, ".")
	}

	// Handle negative zero
	if formatted == "-0" {
		formatted = "0"
	}

	return formatted, nil
}

// FloatToInt scales x by 10^power and converts it to int64.
// Returns an error if the scaled value is not within 1e-3 of an integer,
// which prevents accidental precision loss when rounding.
func FloatToInt(x float64, power int64) (int64, error) {
	withDecimals := x * math.Pow10(int(power))

	rounded := math.Round(withDecimals)

	// Equivalent to: abs(round(with_decimals) - with_decimals) >= 1e-3
	if math.Abs(rounded-withDecimals) >= 1e-3 {
		return 0, errors.New("float_to_int causes rounding")
	}

	return int64(rounded), nil
}

// FloatToUsdInt converts a USD float to an int scaled by 1e6.
// Fails if the value cannot be represented precisely at 6 decimals.
func FloatToUsdInt(x float64) (int64, error) {
	return FloatToInt(x, 6)
}

// StringToFloat converts a string price to float64
// Used for trigger prices that may already be in string format
func StringToFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// RoundToSigfig rounds x to n significant figures.
func RoundToSigfig(x float64, n int64) float64 {
	if x == 0 {
		return 0
	}
	d := math.Ceil(math.Log10(math.Abs(x)))
	power := float64(n) - d
	factor := math.Pow(10, power)
	return math.Round(x*factor) / factor
}

// roundToDecimals reproduces Python's round(x, ndigits) exactly.
// - Uses banker's rounding (round half to even)
// - Supports negative decimals (round to tens, hundreds, etc.)
// - Identical to Python for all float64 values
func RoundToDecimals(x float64, ndigits int64) float64 {
	// Python: if ndigits is 0 or positive
	if ndigits >= 0 {
		factor := math.Pow(10, float64(ndigits))
		return math.RoundToEven(x*factor) / factor
	}

	// Python: negative ndigits (e.g. -1 => nearest 10)
	factor := math.Pow(10, float64(-ndigits))
	return math.RoundToEven(x/factor) * factor
}

// GetDex extracts the exchange name from a coin symbol
func GetDex(coin string) string {
	if i := strings.Index(coin, ":"); i != -1 {
		return coin[:i]
	}
	return ""
}
