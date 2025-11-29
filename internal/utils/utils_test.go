package utils

import (
	"math"
	"testing"
)

func TestFloatToWire_Success(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    float64
		expected string
	}{
		{
			name:     "zero",
			input:    0.0,
			expected: "0",
		},
		{
			name:     "negative zero",
			input:    math.Copysign(0.0, -1.0),
			expected: "0",
		},
		{
			name:     "simple positive",
			input:    1.23,
			expected: "1.23", // 1.23000000 -> trim -> 1.23
		},
		{
			name:     "full 8 decimals",
			input:    1.23456789,
			expected: "1.23456789",
		},
		{
			name:     "small number at 8 decimals",
			input:    0.00000001,
			expected: "0.00000001",
		},
		{
			name:     "large number with decimals",
			input:    123456789.12345678,
			expected: "123456789.12345678",
		},
		{
			name:     "integer without decimals",
			input:    42,
			expected: "42",
		},
		{
			name:     "negative value",
			input:    -1.23456789,
			expected: "-1.23456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FloatToWire(tt.input)
			if err != nil {
				t.Fatalf("floatToWire(%v) unexpected error: %v", tt.input, err)
			}
			if got != tt.expected {
				t.Fatalf(
					"floatToWire(%v) = %q, want %q",
					tt.input,
					got,
					tt.expected,
				)
			}
		})
	}
}

func TestFloatToWire_Error(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input float64
	}{
		{
			name:  "NaN",
			input: math.NaN(),
		},
		{
			name:  "positive infinity",
			input: math.Inf(1),
		},
		{
			name:  "negative infinity",
			input: math.Inf(-1),
		},
		{
			// A value that would require more than 8 decimals to be represented
			// within the 1e-12 tolerance.
			name:  "precision loss",
			input: 1.00000000001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FloatToWire(tt.input)
			if err == nil {
				t.Fatalf("floatToWire(%v) expected error, got nil", tt.input)
			}
		})
	}
}

func TestStringToFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		want       float64
		shouldFail bool
	}{
		{
			name:       "valid integer",
			input:      "42",
			want:       42.0,
			shouldFail: false,
		},
		{
			name:       "valid decimal",
			input:      "123.456",
			want:       123.456,
			shouldFail: false,
		},
		{
			name:       "valid scientific notation",
			input:      "1e-8",
			want:       1e-8,
			shouldFail: false,
		},
		{
			name:       "invalid string",
			input:      "not-a-number",
			shouldFail: true,
		},
	}

	const epsilon = 1e-12

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StringToFloat(tt.input)
			if tt.shouldFail {
				if err == nil {
					t.Fatalf(
						"stringToFloat(%q) expected error, got nil",
						tt.input,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf(
					"stringToFloat(%q) unexpected error: %v",
					tt.input,
					err,
				)
			}
			if math.Abs(got-tt.want) > epsilon {
				t.Fatalf(
					"stringToFloat(%q) = %v, want %v",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestRoundToSigfig(t *testing.T) {
	t.Parallel()
	type args struct {
		x float64
		n int64
	}

	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "zero",
			args: args{x: 0, n: 5},
			want: 0,
		},
		{
			name: "large number",
			args: args{x: 123456.789, n: 5},
			want: 123460,
		},
		{
			name: "small number",
			args: args{x: 0.00123456789, n: 5},
			want: 0.0012346,
		},
		{
			name: "one sigfig",
			args: args{x: 987.654, n: 1},
			want: 1000,
		},
		{
			name: "negative number",
			args: args{x: -1234.567, n: 3},
			want: -1230,
		},
	}

	const epsilon = 1e-12

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RoundToSigfig(tt.args.x, tt.args.n)
			if math.Abs(got-tt.want) > epsilon {
				t.Fatalf("roundToSigfig(%v, %d) = %v, want %v",
					tt.args.x, tt.args.n, got, tt.want)
			}
		})
	}
}

func TestRoundToDecimals(t *testing.T) {
	t.Parallel()
	type args struct {
		x        float64
		decimals int64
	}

	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "no decimals",
			args: args{x: 123.4567, decimals: 0},
			want: 123,
		},
		{
			name: "two decimals",
			args: args{x: 123.4567, decimals: 2},
			want: 123.46,
		},
		{
			name: "three decimals",
			args: args{x: 0.0012345, decimals: 3},
			want: 0.001,
		},
		{
			name: "negative decimals (tens)",
			args: args{x: 1234.567, decimals: -1},
			want: 1230,
		},
		{
			name: "negative decimals (hundreds)",
			args: args{x: 1234.567, decimals: -2},
			want: 1200,
		},
		{
			name: "negative number",
			args: args{x: -1.2345, decimals: 3},
			want: -1.234,
		},
	}

	const epsilon = 1e-12

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RoundToDecimals(tt.args.x, tt.args.decimals)
			if math.Abs(got-tt.want) > epsilon {
				t.Fatalf("roundToDecimals(%v, %d) = %v, want %v",
					tt.args.x, tt.args.decimals, got, tt.want)
			}
		})
	}
}

func TestGetDex(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "binance:BTC-USDT",
			want:  "binance",
		},
		{
			input: "ftx:SOL-PERP",
			want:  "ftx",
		},
		{
			input: "BTC-PERP",
			want:  "",
		},
		{
			input: ":weird",
			want:  "",
		},
		{
			input: "abc:def:ghi", // splits at first colon only
			want:  "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := GetDex(tt.input)
			if got != tt.want {
				t.Fatalf("getDex(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFloatToInt(t *testing.T) {
	tests := []struct {
		name    string
		x       float64
		power   int64
		want    int64
		wantErr bool
	}{
		{
			name:    "simple positive",
			x:       12.34,
			power:   2,
			want:    1234,
			wantErr: false,
		},
		{
			name:    "more decimals but still safe",
			x:       1.234567,
			power:   6,
			want:    1234567,
			wantErr: false,
		},
		{
			name:    "negative value",
			x:       -1.2345,
			power:   4,
			want:    -12345,
			wantErr: false,
		},
		{
			name:    "large rounding required - should error",
			x:       0.1234567, // -> 123456.7 with power=6
			power:   6,
			want:    0,
			wantErr: true,
		},
		{
			name:    "already integer after scaling",
			x:       100.0,
			power:   0,
			want:    100,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := FloatToInt(tt.x, tt.power)
			if tt.wantErr {
				if err == nil {
					t.Fatalf(
						"floatToInt(%v, %d) expected error, got nil (value %v)",
						tt.x,
						tt.power,
						got,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf(
					"floatToInt(%v, %d) unexpected error: %v",
					tt.x,
					tt.power,
					err,
				)
			}
			if got != tt.want {
				t.Fatalf(
					"floatToInt(%v, %d) = %v, want %v",
					tt.x,
					tt.power,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestFloatToUsdInt(t *testing.T) {
	tests := []struct {
		name    string
		x       float64
		want    int64
		wantErr bool
	}{
		{
			name:    "exact 6 decimal places",
			x:       12.345678, // -> 12345678
			want:    12345678,
			wantErr: false,
		},
		{
			name:    "more than 6 decimals - should error",
			x:       0.0000015, // -> 1.5 after scaling by 1e6
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative usd value",
			x:       -0.123456,
			want:    -123456,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := FloatToUsdInt(tt.x)
			if tt.wantErr {
				if err == nil {
					t.Fatalf(
						"floatToUsdInt(%v) expected error, got nil (value %v)",
						tt.x,
						got,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("floatToUsdInt(%v) unexpected error: %v", tt.x, err)
			}
			if got != tt.want {
				t.Fatalf("floatToUsdInt(%v) = %v, want %v", tt.x, got, tt.want)
			}
		})
	}
}
