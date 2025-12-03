package exchange

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/banky/go-hyperliquid/constants"
	"github.com/banky/go-hyperliquid/rest"
	"github.com/banky/go-hyperliquid/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/samber/mo"
)

// Helper to create a test private key
// IMPORTANT: Set this to match your test wallet's private key
// Default is the standard test key used in the Python SDK tests
func testPrivateKey() *ecdsa.PrivateKey {
	// TODO: Configure this with your test private key
	key, _ := crypto.HexToECDSA(
		"0123456789012345678901234567890123456789012345678901234567890123",
	)
	return key
}

// Helper to create a test Exchange instance
func testExchange(isMainnet bool) *Exchange {
	key := testPrivateKey()
	baseURL := "https://api.hyperliquid.xyz"
	if !isMainnet {
		baseURL = "https://testnet.hyperliquid.xyz"
	}

	restClient := rest.New(rest.Config{
		BaseUrl: baseURL,
	})

	accountAddr := crypto.PubkeyToAddress(key.PublicKey)

	return &Exchange{
		privateKey:     key,
		accountAddress: mo.Some(accountAddr),
		vaultAddress:   mo.None[common.Address](),
		expiresAfter:   mo.None[time.Duration](),
		rest:           restClient,
	}
}

func TestPhantomAgentCreation(t *testing.T) {
	timestamp := 1677777606040
	order := OrderRequest(
		"ETH",
		true,
		0.0147,
		1670.1,
		WithLimitOrder(LimitOrder{Tif: "Ioc"}),
		WithReduceOnly(false),
	)
	wire, err := order.toOrderWire(4)
	if err != nil {
		t.Fatal(err)
	}
	action := ordersToAction(
		[]orderWire{wire},
		mo.None[BuilderInfo](),
		mo.None[OrderGrouping](),
	)
	hash, err := hashAction(
		action,
		mo.None[common.Address](),
		uint64(timestamp),
		mo.None[time.Duration](),
	)
	if err != nil {
		t.Fatal(err)
	}

	phantomAgent := constructPhantomAgent(hash, true)

	connIDRaw := phantomAgent["connectionId"]

	connID, ok := connIDRaw.(common.Hash)
	if !ok {
		t.Fatalf("expected connectionId to be common.Hash, got %T", connIDRaw)
	}

	expected := common.HexToHash(
		"0x0fcbeda5ae3c4950a548021552a4fea2226858c4453571bf3f24ba017eac2908",
	)

	if connID != expected {
		t.Fatalf(
			"connectionId mismatch: expected %s, got %s",
			expected.Hex(),
			connID.Hex(),
		)
	}
}

func TestL1SigningOrderWithCloidMatches(t *testing.T) {
	privateKey, err := crypto.HexToECDSA(
		"0123456789012345678901234567890123456789012345678901234567890123",
	)
	if err != nil {
		t.Fatal(err)
	}

	timestamp := 0
	order := OrderRequest(
		"ETH",
		true,
		100,
		100,
		WithLimitOrder(LimitOrder{Tif: "Gtc"}),
		WithReduceOnly(false),
		WithCloid(types.HexToCloid("0x00000000000000000000000000000001")),
	)

	wire, err := order.toOrderWire(1)
	if err != nil {
		t.Fatal(err)
	}

	action := ordersToAction(
		[]orderWire{wire},
		mo.None[BuilderInfo](),
		mo.None[OrderGrouping](),
	)

	e, err := New(Config{
		SkipInfo:   true,
		PrivateKey: privateKey,
	})

	sig, err := signL1Action(
		action,
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedR := common.HexToHash(
		"0x41ae18e8239a56cacbc5dad94d45d0b747e5da11ad564077fcac71277a946e3",
	)
	expectedS := common.HexToHash(
		"0x3c61f667e747404fe7eea8f90ab0e76cc12ce60270438b2058324681a00116da",
	)
	expectedV := byte(27)

	if sig.R != expectedR {
		t.Fatalf(
			"R mismatch: expected %s, got %s",
			expectedR.Hex(),
			sig.R.Hex(),
		)
	}

	if sig.S != expectedS {
		t.Fatalf(
			"S mismatch: expected %s, got %s",
			expectedS.Hex(),
			sig.S.Hex(),
		)
	}

	if sig.V != expectedV {
		t.Fatalf("V mismatch: expected %d, got %d", expectedV, sig.V)
	}

	eTestnet, err := New(Config{
		BaseURL:    constants.TESTNET_API_URL,
		SkipInfo:   true,
		PrivateKey: privateKey,
	})

	sigTestnet, err := signL1Action(
		action,
		uint64(timestamp),
		eTestnet.privateKey,
		eTestnet.vaultAddress,
		eTestnet.expiresAfter,
		eTestnet.rest.IsMainnet(),
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedRTestnet := common.HexToHash(
		"0xeba0664bed2676fc4e5a743bf89e5c7501aa6d870bdb9446e122c9466c5cd16d",
	)
	expectedSTestnet := common.HexToHash(
		"0x7f3e74825c9114bc59086f1eebea2928c190fdfbfde144827cb02b85bbe90988",
	)
	expectedVTestnet := byte(28)

	if sigTestnet.R != expectedRTestnet {
		t.Fatalf(
			"R mismatch: expected %s, got %s",
			expectedRTestnet.Hex(),
			sigTestnet.R.Hex(),
		)
	}

	if sigTestnet.S != expectedSTestnet {
		t.Fatalf(
			"S mismatch: expected %s, got %s",
			expectedSTestnet.Hex(),
			sigTestnet.S.Hex(),
		)
	}

	if sigTestnet.V != expectedVTestnet {
		t.Fatalf(
			"V mismatch: expected %d, got %d",
			expectedVTestnet,
			sigTestnet.V,
		)
	}
}

func TestSignUsdTransferAction(t *testing.T) {
	privateKey, err := crypto.HexToECDSA(
		"0123456789012345678901234567890123456789012345678901234567890123",
	)
	if err != nil {
		t.Fatal(err)
	}

	action := usdTransferAction{
		Type:             "usdSend",
		Amount:           "1",
		Destination:      "0x5e9ee1089755c3435139848e47e6635505d5a13a",
		Time:             1687816341423,
		HyperliquidChain: "Testnet",
		SignatureChainId: getSignatureChainId(),
	}

	sig, err := signUsdTransferAction(action, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	expectedR := common.HexToHash(
		"0x637b37dd731507cdd24f46532ca8ba6eec616952c56218baeff04144e4a77073",
	)
	expectedS := common.HexToHash(
		"0x11a6a24900e6e314136d2592e2f8d502cd89b7c15b198e1bee043c9589f9fad7",
	)
	expectedV := byte(27)

	if sig.R != expectedR {
		t.Fatalf(
			"R mismatch: expected %s, got %s",
			expectedR.Hex(),
			sig.R.Hex(),
		)
	}

	if sig.S != expectedS {
		t.Fatalf(
			"S mismatch: expected %s, got %s",
			expectedS.Hex(),
			sig.S.Hex(),
		)
	}

	if sig.V != expectedV {
		t.Fatalf("V mismatch: expected %d, got %d", expectedV, sig.V)
	}
}

func TestSubAccountTransferAction(t *testing.T) {
	privateKey, err := crypto.HexToECDSA(
		"0123456789012345678901234567890123456789012345678901234567890123",
	)
	if err != nil {
		t.Fatal(err)
	}

	action := subAccountTransferAction{
		Type:           "subAccountTransfer",
		SubAccountUser: "0x1d9470d4b963f552e6f671a81619d395877bf409",
		IsDeposit:      true,
		Usd:            10,
	}

	sig, err := signL1Action(
		action,
		0,
		privateKey,
		mo.None[common.Address](),
		mo.None[time.Duration](),
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedR := common.HexToHash(
		"0x43592d7c6c7d816ece2e206f174be61249d651944932b13343f4d13f306ae602",
	)
	expectedS := common.HexToHash(
		"0x71a926cb5c9a7c01c3359ec4c4c34c16ff8107d610994d4de0e6430e5cc0f4c9",
	)
	expectedV := byte(28)

	if sig.R != expectedR {
		t.Fatalf(
			"R mismatch: expected %s, got %s",
			expectedR.Hex(),
			sig.R.Hex(),
		)
	}

	if sig.S != expectedS {
		t.Fatalf(
			"S mismatch: expected %s, got %s",
			expectedS.Hex(),
			sig.S.Hex(),
		)
	}

	if sig.V != expectedV {
		t.Fatalf("V mismatch: expected %d, got %d", expectedV, sig.V)
	}
}

// func TestL1ActionSigningProducesValidSignature(t *testing.T) {
// 	ex := testExchange(true)
// 	numStr, _ := floatToWire(1000)
// 	action := map[string]any{
// 		"type": "dummy",
// 		"num":  numStr,
// 	}

// 	// Test mainnet produces a valid signature
// 	sig, err := ex.signL1Action(action, 0)
// 	if err != nil {
// 		t.Errorf("mainnet signing failed: %v", err)
// 	}
// 	if sig.V != 27 && sig.V != 28 {
// 		t.Errorf("mainnet V should be 27 or 28, got %d", sig.V)
// 	}

// 	// Test testnet produces a valid signature
// 	exTestnet := testExchange(false)
// 	sigTestnet, err := exTestnet.signL1Action(action, 0)
// 	if err != nil {
// 		t.Errorf("testnet signing failed: %v", err)
// 	}
// 	if sigTestnet.V != 27 && sigTestnet.V != 28 {
// 		t.Errorf("testnet V should be 27 or 28, got %d", sigTestnet.V)
// 	}
// }

// func TestL1ActionSigningOrderMatches(t *testing.T) {
// 	ex := testExchange(true)
// 	orderRequest := OrderRequest{
// 		Coin:       "ETH",
// 		IsBuy:      true,
// 		Sz:         100,
// 		LimitPx:    100,
// 		ReduceOnly: false,
// 		OrderType: OrderType{
// 			Limit: &LimitOrder{Tif: "Gtc"},
// 		},
// 		CLOID: nil,
// 	}

// 	orderWireItem, _ := orderRequest.toOrderWire(1)
// 	action := ordersToAction([]orderWire{orderWireItem}, nil)

// 	// Test mainnet
// 	sig, _ := ex.signL1Action(action, 0)
// 	if sig.R != [32]byte{
// 		0xd6,
// 		0x53,
// 		0x69,
// 		0x82,
// 		0x5a,
// 		0x9d,
// 		0xf5,
// 		0xd8,
// 		0x00,
// 		0x99,
// 		0xe5,
// 		0x13,
// 		0xcc,
// 		0xe4,
// 		0x30,
// 		0x31,
// 		0x1d,
// 		0x7d,
// 		0x26,
// 		0xdd,
// 		0xf4,
// 		0x77,
// 		0xf5,
// 		0xb3,
// 		0xa3,
// 		0x3d,
// 		0x28,
// 		0x06,
// 		0xb1,
// 		0x00,
// 		0xd7,
// 		0x8e,
// 	} {
// 		t.Errorf("mainnet R mismatch")
// 	}
// 	if sig.V != 28 {
// 		t.Errorf("mainnet V mismatch: got %d, want 28", sig.V)
// 	}

// 	// Test testnet
// 	exTestnet := testExchange(false)
// 	sigTestnet, _ := exTestnet.signL1Action(action, 0)
// 	if sigTestnet.V != 27 {
// 		t.Errorf("testnet V mismatch: got %d, want 27", sigTestnet.V)
// 	}
// }

// func TestL1ActionSigningOrderWithCloidMatches(t *testing.T) {
// 	ex := testExchange(true)
// 	cloidHash := common.HexToHash("0x00000000000000000000000000000001")

// 	orderRequest := OrderRequest{
// 		Coin:       "ETH",
// 		IsBuy:      true,
// 		Sz:         100,
// 		LimitPx:    100,
// 		ReduceOnly: false,
// 		OrderType: OrderType{
// 			Limit: &LimitOrder{Tif: "Gtc"},
// 		},
// 		CLOID: &cloidHash,
// 	}

// 	orderWireItem, _ := orderRequest.toOrderWire(1)
// 	action := ordersToAction([]orderWire{orderWireItem}, nil)

// 	// Test mainnet
// 	sig, _ := ex.signL1Action(action, 0)
// 	if sig.R != [32]byte{
// 		0x41,
// 		0xae,
// 		0x18,
// 		0xe8,
// 		0x23,
// 		0x9a,
// 		0x56,
// 		0xca,
// 		0xcb,
// 		0xc5,
// 		0xda,
// 		0xd9,
// 		0x4d,
// 		0x45,
// 		0xd0,
// 		0xb7,
// 		0x47,
// 		0xe5,
// 		0xda,
// 		0x11,
// 		0xad,
// 		0x56,
// 		0x40,
// 		0x77,
// 		0xfc,
// 		0xac,
// 		0x71,
// 		0x27,
// 		0x7a,
// 		0x94,
// 		0x6e,
// 		0x3,
// 	} {
// 		t.Errorf("mainnet R mismatch")
// 	}
// 	if sig.V != 27 {
// 		t.Errorf("mainnet V mismatch: got %d, want 27", sig.V)
// 	}

// 	// Test testnet
// 	exTestnet := testExchange(false)
// 	sigTestnet, _ := exTestnet.signL1Action(action, 0)
// 	if sigTestnet.V != 28 {
// 		t.Errorf("testnet V mismatch: got %d, want 28", sigTestnet.V)
// 	}
// }

// func TestL1ActionSigningMatchesWithVault(t *testing.T) {
// 	vaultAddr := common.HexToAddress(
// 		"0x1719884eb866cb12b2287399b15f7db5e7d775ea",
// 	)
// 	vaultOpt := mo.Some(vaultAddr)

// 	ex := testExchange(true)
// 	ex.vaultAddress = vaultOpt

// 	numStr, _ := floatToWire(1000)
// 	action := map[string]any{
// 		"type": "dummy",
// 		"num":  numStr,
// 	}

// 	// Test mainnet with vault
// 	sig, _ := ex.signL1ActionWithVault(action, 0, vaultOpt)
// 	if sig.R != [32]byte{
// 		0x03,
// 		0xc5,
// 		0x48,
// 		0xdb,
// 		0x75,
// 		0xe4,
// 		0x79,
// 		0xf8,
// 		0x01,
// 		0x2a,
// 		0xcf,
// 		0x30,
// 		0x00,
// 		0xca,
// 		0x3a,
// 		0x6b,
// 		0x05,
// 		0x60,
// 		0x6b,
// 		0xc2,
// 		0xec,
// 		0x0c,
// 		0x29,
// 		0xc5,
// 		0x0c,
// 		0x51,
// 		0x50,
// 		0x66,
// 		0xa3,
// 		0x26,
// 		0x23,
// 		0x9,
// 	} {
// 		t.Errorf("mainnet R mismatch")
// 	}
// 	if sig.V != 28 {
// 		t.Errorf("mainnet V mismatch: got %d, want 28", sig.V)
// 	}

// 	// Test testnet with vault
// 	exTestnet := testExchange(false)
// 	exTestnet.vaultAddress = vaultOpt
// 	sigTestnet, _ := exTestnet.signL1ActionWithVault(action, 0, vaultOpt)
// 	if sigTestnet.V != 27 {
// 		t.Errorf("testnet V mismatch: got %d, want 27", sigTestnet.V)
// 	}
// }

// func TestL1ActionSigningTpslOrderMatches(t *testing.T) {
// 	ex := testExchange(true)
// 	orderRequest := OrderRequest{
// 		Coin:       "ETH",
// 		IsBuy:      true,
// 		Sz:         100,
// 		LimitPx:    100,
// 		ReduceOnly: false,
// 		OrderType: OrderType{
// 			Trigger: &TriggerOrder{
// 				IsMarket:  true,
// 				TriggerPx: 103,
// 				TpSl:      "sl",
// 			},
// 		},
// 		CLOID: nil,
// 	}

// 	orderWireItem, _ := orderRequest.toOrderWire(1)
// 	action := ordersToAction([]orderWire{orderWireItem}, nil)

// 	// Test mainnet
// 	sig, _ := ex.signL1Action(action, 0)
// 	if sig.R != [32]byte{
// 		0x98,
// 		0x34,
// 		0x3f,
// 		0x2b,
// 		0x5a,
// 		0xe8,
// 		0xe2,
// 		0x6b,
// 		0xb2,
// 		0x58,
// 		0x7d,
// 		0xaa,
// 		0xd3,
// 		0x86,
// 		0x3b,
// 		0xc7,
// 		0x0d,
// 		0x87,
// 		0x92,
// 		0xb0,
// 		0x9a,
// 		0xf1,
// 		0x84,
// 		0x1b,
// 		0x6f,
// 		0xdd,
// 		0x53,
// 		0x0a,
// 		0x20,
// 		0x65,
// 		0xa3,
// 		0xf9,
// 	} {
// 		t.Errorf("mainnet R mismatch")
// 	}
// 	if sig.V != 27 {
// 		t.Errorf("mainnet V mismatch: got %d, want 27", sig.V)
// 	}

// 	// Test testnet
// 	exTestnet := testExchange(false)
// 	sigTestnet, _ := exTestnet.signL1Action(action, 0)
// 	if sigTestnet.V != 28 {
// 		t.Errorf("testnet V mismatch: got %d, want 28", sigTestnet.V)
// 	}
// }

// func TestFloatToWire(t *testing.T) {
// 	tests := []struct {
// 		input    float64
// 		expected string
// 	}{
// 		{123123123123, "12312312312300000000"},
// 		{0.00001231, "1231"},
// 		{1.033, "103300000"},
// 	}

// 	for _, tt := range tests {
// 		result, _ := floatToWire(tt.input)
// 		if result != tt.expected {
// 			t.Errorf(
// 				"floatToWire(%v) = %s, want %s",
// 				tt.input,
// 				result,
// 				tt.expected,
// 			)
// 		}
// 	}

// 	// Test invalid value
// 	_, err := floatToWire(0.000012312312)
// 	if err == nil {
// 		t.Errorf("floatToWire(0.000012312312) should return error")
// 	}
// }

// func TestSignUsdTransferAction(t *testing.T) {
// 	ex := testExchange(false) // testnet

// 	action := map[string]any{
// 		"destination": "0x5e9ee1089755c3435139848e47e6635505d5a13a",
// 		"amount":      "1",
// 		"time":        int64(1687816341423),
// 	}

// 	sig, _ := ex.signUsdTransferAction(action)
// 	if sig.R != [32]byte{
// 		0x63,
// 		0x7b,
// 		0x37,
// 		0xdd,
// 		0x73,
// 		0x15,
// 		0x07,
// 		0xcd,
// 		0xd2,
// 		0x4f,
// 		0x46,
// 		0x53,
// 		0x2c,
// 		0xa8,
// 		0xba,
// 		0x6e,
// 		0xec,
// 		0x61,
// 		0x69,
// 		0x52,
// 		0xc5,
// 		0x62,
// 		0x18,
// 		0xba,
// 		0xef,
// 		0xf0,
// 		0x41,
// 		0x44,
// 		0xe4,
// 		0xa7,
// 		0x70,
// 		0x73,
// 	} {
// 		t.Errorf("R mismatch")
// 	}
// 	if sig.S != [32]byte{
// 		0x11,
// 		0xa6,
// 		0xa2,
// 		0x49,
// 		0x00,
// 		0xe6,
// 		0xe3,
// 		0x14,
// 		0x13,
// 		0x6d,
// 		0x25,
// 		0x92,
// 		0xe2,
// 		0xf8,
// 		0xd5,
// 		0x02,
// 		0xcd,
// 		0x89,
// 		0xb7,
// 		0xc1,
// 		0x5b,
// 		0x19,
// 		0x8e,
// 		0x1b,
// 		0xee,
// 		0x04,
// 		0x3c,
// 		0x95,
// 		0x89,
// 		0xf9,
// 		0xfa,
// 		0xd7,
// 	} {
// 		t.Errorf("S mismatch")
// 	}
// 	if sig.V != 27 {
// 		t.Errorf("V mismatch: got %d, want 27", sig.V)
// 	}
// }

// func TestSignWithdrawFromBridgeAction(t *testing.T) {
// 	ex := testExchange(false) // testnet

// 	action := map[string]any{
// 		"destination": "0x5e9ee1089755c3435139848e47e6635505d5a13a",
// 		"amount":      "1",
// 		"time":        int64(1687816341423),
// 	}

// 	sig, _ := ex.signWithdrawFromBridgeAction(action)
// 	if sig.R != [32]byte{
// 		0x83,
// 		0x63,
// 		0x52,
// 		0x4c,
// 		0x79,
// 		0x9e,
// 		0x90,
// 		0xce,
// 		0x9b,
// 		0xc4,
// 		0x10,
// 		0x22,
// 		0xf7,
// 		0xc3,
// 		0x9b,
// 		0x4e,
// 		0x9b,
// 		0xdb,
// 		0xa7,
// 		0x86,
// 		0xe5,
// 		0xf9,
// 		0xc7,
// 		0x2b,
// 		0x20,
// 		0xe4,
// 		0x3e,
// 		0x14,
// 		0x62,
// 		0xc3,
// 		0x7c,
// 		0xf9,
// 	} {
// 		t.Errorf("R mismatch")
// 	}
// 	if sig.S != [32]byte{
// 		0x58,
// 		0xb1,
// 		0x41,
// 		0x1a,
// 		0x77,
// 		0x59,
// 		0x38,
// 		0xb8,
// 		0x3e,
// 		0x29,
// 		0x18,
// 		0x2a,
// 		0x8e,
// 		0xef,
// 		0x74,
// 		0x97,
// 		0x5f,
// 		0x90,
// 		0x54,
// 		0xc8,
// 		0xe9,
// 		0x7e,
// 		0xbf,
// 		0x5e,
// 		0xc2,
// 		0xdc,
// 		0x8d,
// 		0x51,
// 		0xbf,
// 		0xc8,
// 		0x93,
// 		0x81,
// 	} {
// 		t.Errorf("S mismatch")
// 	}
// 	if sig.V != 28 {
// 		t.Errorf("V mismatch: got %d, want 28", sig.V)
// 	}
// }

// func TestCreateSubAccountAction(t *testing.T) {
// 	ex := testExchange(true)
// 	action := map[string]any{
// 		"type": "createSubAccount",
// 		"name": "example",
// 	}

// 	// Test mainnet
// 	sig, _ := ex.signL1Action(action, 0)
// 	if sig.R != [32]byte{
// 		0x51,
// 		0x09,
// 		0x6f,
// 		0xe3,
// 		0x23,
// 		0x94,
// 		0x21,
// 		0xd1,
// 		0x6b,
// 		0x67,
// 		0x1e,
// 		0x19,
// 		0x2f,
// 		0x57,
// 		0x4a,
// 		0xe2,
// 		0x4a,
// 		0xe1,
// 		0x43,
// 		0x29,
// 		0x09,
// 		0x9b,
// 		0x6d,
// 		0xb2,
// 		0x8e,
// 		0x47,
// 		0x9b,
// 		0x86,
// 		0xcd,
// 		0xd6,
// 		0xca,
// 		0xa7,
// 	} {
// 		t.Errorf("mainnet R mismatch")
// 	}
// 	if sig.V != 27 {
// 		t.Errorf("mainnet V mismatch: got %d, want 27", sig.V)
// 	}

// 	// Test testnet
// 	exTestnet := testExchange(false)
// 	sigTestnet, _ := exTestnet.signL1Action(action, 0)
// 	if sigTestnet.V != 28 {
// 		t.Errorf("testnet V mismatch: got %d, want 28", sigTestnet.V)
// 	}
// }

// func TestSubAccountTransferAction(t *testing.T) {
// 	ex := testExchange(true)
// 	action := map[string]any{
// 		"type":           "subAccountTransfer",
// 		"subAccountUser": "0x1d9470d4b963f552e6f671a81619d395877bf409",
// 		"isDeposit":      true,
// 		"usd":            10,
// 	}

// 	// Test mainnet
// 	sig, _ := ex.signL1Action(action, 0)
// 	if sig.R != [32]byte{
// 		0x43,
// 		0x59,
// 		0x2d,
// 		0x7c,
// 		0x6c,
// 		0x7d,
// 		0x81,
// 		0x6e,
// 		0xce,
// 		0x2e,
// 		0x20,
// 		0x6f,
// 		0x17,
// 		0x4b,
// 		0xe6,
// 		0x12,
// 		0x49,
// 		0xd6,
// 		0x51,
// 		0x94,
// 		0x49,
// 		0x32,
// 		0xb1,
// 		0x33,
// 		0x43,
// 		0xf4,
// 		0xd1,
// 		0x3f,
// 		0x30,
// 		0x6a,
// 		0xe6,
// 		0x02,
// 	} {
// 		t.Errorf("mainnet R mismatch")
// 	}
// 	if sig.V != 28 {
// 		t.Errorf("mainnet V mismatch: got %d, want 28", sig.V)
// 	}

// 	// Test testnet
// 	exTestnet := testExchange(false)
// 	sigTestnet, _ := exTestnet.signL1Action(action, 0)
// 	if sigTestnet.V != 28 {
// 		t.Errorf("testnet V mismatch: got %d, want 28", sigTestnet.V)
// 	}
// }

// func TestScheduleCancelAction(t *testing.T) {
// 	ex := testExchange(true)

// 	// Test without time
// 	action := map[string]any{
// 		"type": "scheduleCancel",
// 	}

// 	sig, _ := ex.signL1Action(action, 0)
// 	if sig.R != [32]byte{
// 		0x6c,
// 		0xdf,
// 		0xb2,
// 		0x86,
// 		0x70,
// 		0x2f,
// 		0x59,
// 		0x17,
// 		0xe7,
// 		0x6c,
// 		0xd9,
// 		0xb3,
// 		0xb8,
// 		0xbf,
// 		0x67,
// 		0x8f,
// 		0xcc,
// 		0x49,
// 		0xae,
// 		0xc1,
// 		0x94,
// 		0xc0,
// 		0x2a,
// 		0x73,
// 		0xe6,
// 		0xd4,
// 		0xf1,
// 		0x68,
// 		0x91,
// 		0x19,
// 		0x5d,
// 		0xf9,
// 	} {
// 		t.Errorf("mainnet R mismatch (no time)")
// 	}
// 	if sig.V != 27 {
// 		t.Errorf("mainnet V mismatch (no time): got %d, want 27", sig.V)
// 	}

// 	// Test with time
// 	action = map[string]any{
// 		"type": "scheduleCancel",
// 		"time": 123456789,
// 	}

// 	sig, _ = ex.signL1Action(action, 0)
// 	if sig.R != [32]byte{
// 		0x60,
// 		0x9c,
// 		0xb2,
// 		0x0c,
// 		0x73,
// 		0x79,
// 		0x45,
// 		0xd0,
// 		0x70,
// 		0x71,
// 		0x6d,
// 		0xcc,
// 		0x69,
// 		0x6b,
// 		0xa0,
// 		0x30,
// 		0xe9,
// 		0x97,
// 		0x6f,
// 		0xcf,
// 		0x5e,
// 		0xda,
// 		0xd8,
// 		0x7a,
// 		0xfa,
// 		0x7d,
// 		0x87,
// 		0x74,
// 		0x93,
// 		0x10,
// 		0x9d,
// 		0x55,
// 	} {
// 		t.Errorf("mainnet R mismatch (with time)")
// 	}
// 	if sig.V != 28 {
// 		t.Errorf("mainnet V mismatch (with time): got %d, want 28", sig.V)
// 	}
// }
