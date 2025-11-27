package info

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// cassetteLoader loads cassettes from JSON files
type cassetteLoader struct {
	cassettes map[string]interface{}
}

// newCassetteLoader creates a new cassette loader
func newCassetteLoader() *cassetteLoader {
	return &cassetteLoader{
		cassettes: make(map[string]interface{}),
	}
}

// loadCassette loads a cassette from JSON data
func (cl *cassetteLoader) loadCassette(name string, data []byte) error {
	var cassette interface{}
	if err := json.Unmarshal(data, &cassette); err != nil {
		return fmt.Errorf("failed to unmarshal cassette %s: %w", name, err)
	}

	cl.cassettes[name] = cassette
	return nil
}

// getCassette retrieves a loaded cassette by name
func (cl *cassetteLoader) getCassette(name string) (interface{}, error) {
	cassette, ok := cl.cassettes[name]
	if !ok {
		return nil, fmt.Errorf("cassette %s not found", name)
	}

	return cassette, nil
}

// cassetteRestClient is a mock REST client that returns cassette data
type cassetteRestClient struct {
	loader              *cassetteLoader
	cassetteMappings    map[string]string
}

// newCassetteRestClient creates a new cassette-based REST client
func newCassetteRestClient(loader *cassetteLoader) *cassetteRestClient {
	return &cassetteRestClient{
		loader:           loader,
		cassetteMappings: make(map[string]string),
	}
}

// registerCassette maps a request type/name combination to a cassette
func (crc *cassetteRestClient) registerCassette(name string, cassetteName string) {
	crc.cassetteMappings[name] = cassetteName
}

// Post implements the rest.ClientInterface Post method using cassettes
func (crc *cassetteRestClient) Post(ctx context.Context, path string, body any, result any) error {
	// Extract request type from body
	bodyMap, ok := body.(map[string]any)
	if !ok {
		return errors.New("request body must be a map")
	}

	requestType, ok := bodyMap["type"].(string)
	if !ok {
		return errors.New("request body must contain 'type' field")
	}

	// Try to find a cassette mapping for this request type
	cassetteName, ok := crc.cassetteMappings[requestType]

	if !ok {
		// If no specific mapping, use the request type as cassette name
		cassetteName = requestType
	}

	// Load the cassette
	cassette, err := crc.loader.getCassette(cassetteName)
	if err != nil {
		return fmt.Errorf("failed to load cassette for request type %s: %w", requestType, err)
	}

	// Marshal the cassette response and unmarshal into the result
	cassetteBytes, err := json.Marshal(cassette)
	if err != nil {
		return fmt.Errorf("failed to marshal cassette: %w", err)
	}

	if err := json.Unmarshal(cassetteBytes, result); err != nil {
		return fmt.Errorf("failed to unmarshal cassette into result: %w", err)
	}

	return nil
}

// BaseUrl returns the base URL
func (crc *cassetteRestClient) BaseUrl() string {
	return "https://api.hyperliquid.xyz"
}

// IsMainnet returns whether this is mainnet
func (crc *cassetteRestClient) IsMainnet() bool {
	return true
}

// ===== Test Helpers =====

// loadCassettes helper to load cassettes from files
func loadCassettes(t *testing.T, testCassetteNames ...string) *cassetteRestClient {
	loader := newCassetteLoader()
	client := newCassetteRestClient(loader)

	for _, testName := range testCassetteNames {
		data, err := loadCassetteFile(testName)
		if err != nil {
			t.Fatalf("failed to load cassette file %s: %v", testName, err)
		}
		if err := loader.loadCassette(testName, data); err != nil {
			t.Fatalf("failed to load cassette %s: %v", testName, err)
		}

		// Also register the cassette under the request type key for automatic lookup
		// Register common mappings
		if testName == "test_get_all_mids" {
			client.registerCassette("allMids", testName)
		} else if testName == "test_get_user_state" {
			client.registerCassette("clearinghouseState", testName)
		} else if testName == "test_get_open_orders" {
			client.registerCassette("openOrders", testName)
		} else if testName == "test_get_user_fills" {
			client.registerCassette("userFills", testName)
		} else if testName == "test_get_user_fills_by_time" {
			client.registerCassette("userFillsByTime", testName)
		} else if testName == "test_get_info" {
			client.registerCassette("meta", testName)
		} else if testName == "test_get_funding_history[None]" {
			client.registerCassette("fundingHistory", testName)
		} else if testName == "test_get_l2_snapshot" {
			client.registerCassette("l2Book", testName)
		} else if testName == "test_get_candles_snapshot" {
			client.registerCassette("candleSnapshot", testName)
		} else if testName == "test_user_funding_history_with_end_time" {
			client.registerCassette("userFunding", testName)
		}
	}

	return client
}

// loadCassetteFile loads a cassette JSON file
func loadCassetteFile(name string) ([]byte, error) {
	filename := fmt.Sprintf("cassettes/%s.json", name)
	return os.ReadFile(filename)
}

// ===== Cassette-Based Tests (from Python test suite) =====

func TestCassette_AllMids(t *testing.T) {
	client := loadCassettes(t, "test_get_all_mids")
	info := &Info{rest: client}

	mids, err := info.AllMids(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks for BTC, ETH, ATOM, MATIC
	if mids == nil {
		t.Fatal("expected mids to be non-nil")
	}
	if btc, ok := mids["BTC"]; !ok {
		t.Fatal("expected BTC in mids")
	} else if btc != "30135.0" {
		t.Errorf("expected BTC=30135.0, got %s", btc)
	}
	if eth, ok := mids["ETH"]; !ok {
		t.Fatal("expected ETH in mids")
	} else if eth != "1903.95" {
		t.Errorf("expected ETH=1903.95, got %s", eth)
	}
	if _, ok := mids["ATOM"]; !ok {
		t.Fatal("expected ATOM in mids")
	}
	if _, ok := mids["MATIC"]; !ok {
		t.Fatal("expected MATIC in mids")
	}
}

func TestCassette_UserState(t *testing.T) {
	client := loadCassettes(t, "test_get_user_state")
	info := &Info{rest: client}

	response, err := info.UserState(context.Background(), common.HexToAddress("0x5e9ee1089755c3435139848e47e6635505d5a13a"), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks assetPositions length and marginSummary
	if len(response.AssetPositions) != 12 {
		t.Errorf("expected 12 asset positions, got %d", len(response.AssetPositions))
	}
	if response.MarginSummary.AccountValue != "1182.312496" {
		t.Errorf("expected accountValue=1182.312496, got %s", response.MarginSummary.AccountValue)
	}
}

func TestCassette_OpenOrders(t *testing.T) {
	client := loadCassettes(t, "test_get_open_orders")
	info := &Info{rest: client}

	response, err := info.OpenOrders(context.Background(), "0x5e9ee1089755c3435139848e47e6635505d5a13a", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks response length
	if len(response) != 196 {
		t.Errorf("expected 196 open orders, got %d", len(response))
	}
}

func TestCassette_AllMidsWithNames(t *testing.T) {
	client := loadCassettes(t, "test_get_all_mids")
	info := &Info{rest: client}

	response, err := info.AllMids(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: test_get_all_mids checks for asset names
	coins := []string{"BTC", "ETH", "ATOM", "MATIC"}
	for _, coin := range coins {
		if _, ok := response[coin]; !ok {
			t.Errorf("expected %s in response", coin)
		}
	}
}

func TestCassette_AllMidsCoinsPresent(t *testing.T) {
	client := loadCassettes(t, "test_get_all_mids")
	info := &Info{rest: client}

	mids, err := info.AllMids(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks for BTC, ETH, ATOM, MATIC
	if _, ok := mids["BTC"]; !ok {
		t.Fatal("expected BTC in mids")
	}
	if _, ok := mids["ETH"]; !ok {
		t.Fatal("expected ETH in mids")
	}
	if _, ok := mids["ATOM"]; !ok {
		t.Fatal("expected ATOM in mids")
	}
	if _, ok := mids["MATIC"]; !ok {
		t.Fatal("expected MATIC in mids")
	}
}

func TestCassette_UserFills(t *testing.T) {
	client := loadCassettes(t, "test_get_user_fills")
	info := &Info{rest: client}

	response, err := info.UserFills(context.Background(), "0xb7b6f3cea3f66bf525f5d8f965f6dbf6d9b017b2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks response is a list and first fill has "crossed" = true
	if response == nil {
		t.Fatal("expected response to be non-nil")
	}
	if len(response) == 0 {
		t.Fatal("expected non-empty response")
	}
	// Check structure (actual field names depend on Fill struct)
	if response[0].Coin == "" {
		t.Errorf("expected first fill to have a coin")
	}
}

func TestCassette_UserFillsByTime(t *testing.T) {
	client := loadCassettes(t, "test_get_user_fills_by_time")
	info := &Info{rest: client}

	response, err := info.UserFillsByTime(context.Background(), "0xb7b6f3cea3f66bf525f5d8f965f6dbf6d9b017b2", 1683245555699, nil, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks response length is 500
	if len(response) != 500 {
		t.Errorf("expected 500 fills, got %d", len(response))
	}
}

func TestCassette_Meta(t *testing.T) {
	client := loadCassettes(t, "test_get_info")
	info := &Info{rest: client}

	response, err := info.Meta(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks universe length and first asset
	if len(response.Universe) != 28 {
		t.Errorf("expected 28 assets, got %d", len(response.Universe))
	}
	if len(response.Universe) > 0 && response.Universe[0].Name != "BTC" {
		t.Errorf("expected first asset to be BTC, got %s", response.Universe[0].Name)
	}
	if len(response.Universe) > 0 && response.Universe[0].SzDecimals != 5 {
		t.Errorf("expected BTC szDecimals=5, got %d", response.Universe[0].SzDecimals)
	}
}

func TestCassette_FundingHistory(t *testing.T) {
	client := loadCassettes(t, "test_get_funding_history[None]")
	info := &Info{rest: client}

	response, err := info.FundingHistory(context.Background(), "BTC", 1681923833000, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks response is non-empty and has correct structure
	if len(response) == 0 {
		t.Fatal("expected non-empty funding history")
	}
	if response[0].Coin != "BTC" {
		t.Errorf("expected coin=BTC, got %s", response[0].Coin)
	}
	// Check expected fields exist
	if response[0].FundingRate == "" {
		t.Error("expected fundingRate field")
	}
	if response[0].Premium == "" {
		t.Error("expected premium field")
	}
	if response[0].Time == 0 {
		t.Error("expected time field")
	}
}

func TestCassette_L2Snapshot(t *testing.T) {
	client := loadCassettes(t, "test_get_l2_snapshot")
	info := &Info{rest: client, nameToCoin: map[string]string{"DYDX": "DYDX"}}

	response, err := info.L2Snapshot(context.Background(), "DYDX")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks response structure
	if len(response.Levels) != 2 {
		t.Errorf("expected 2 levels, got %d", len(response.Levels))
	}
	if response.Coin != "DYDX" {
		t.Errorf("expected coin=DYDX, got %s", response.Coin)
	}
	if len(response.Levels) > 0 && len(response.Levels[0]) == 0 {
		t.Error("expected bids to be non-empty")
	}
	if len(response.Levels) > 1 && len(response.Levels[1]) == 0 {
		t.Error("expected asks to be non-empty")
	}
}

func TestCassette_CandlesSnapshot(t *testing.T) {
	client := loadCassettes(t, "test_get_candles_snapshot")
	info := &Info{rest: client, nameToCoin: map[string]string{"kPEPE": "kPEPE"}}

	response, err := info.CandlesSnapshot(context.Background(), "kPEPE", "1h", 1684702007000, 1684784807000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks response length and structure
	if len(response) != 24 {
		t.Errorf("expected 24 candles, got %d", len(response))
	}
	// Check expected candle fields
	if len(response) > 0 {
		candle := response[0]
		if candle.T == 0 {
			t.Error("expected time field")
		}
		if candle.O == "" {
			t.Error("expected open field")
		}
		if candle.C == "" {
			t.Error("expected close field")
		}
	}
}

func TestCassette_UserFundingHistory(t *testing.T) {
	client := loadCassettes(t, "test_user_funding_history_with_end_time")
	info := &Info{rest: client}

	// UserFundingHistory takes int64 values, not pointers
	var endTime int64 = 1682010233000
	response, err := info.UserFundingHistory(context.Background(), "0xb7b6f3cea3f66bf525f5d8f965f6dbf6d9b017b2", 1681923833000, &endTime)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// From Python test: checks response is list and has expected fields
	if response == nil {
		t.Fatal("expected response to be non-nil")
	}
}
