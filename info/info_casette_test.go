package info

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
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
	loader           *cassetteLoader
	cassetteMappings map[string]string
}

// newCassetteRestClient creates a new cassette-based REST client
func newCassetteRestClient(loader *cassetteLoader) *cassetteRestClient {
	return &cassetteRestClient{
		loader:           loader,
		cassetteMappings: make(map[string]string),
	}
}

// registerCassette maps a request type/name combination to a cassette
func (crc *cassetteRestClient) registerCassette(
	name string,
	cassetteName string,
) {
	crc.cassetteMappings[name] = cassetteName
}

// Post implements the rest.ClientInterface Post method using cassettes
func (crc *cassetteRestClient) Post(
	ctx context.Context,
	path string,
	body any,
	result any,
) error {
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
		return fmt.Errorf(
			"failed to load cassette for request type %s: %w",
			requestType,
			err,
		)
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

func (crc *cassetteRestClient) NetworkName() string {
	return "Mainnet"
}

// ===== Test Helpers =====

// loadCassettes helper to load cassettes from files
// Use testing.TB so it works with both *testing.T and *td.T via TB().
func loadCassettes(
	t testing.TB,
	testCassetteNames ...string,
) *cassetteRestClient {
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

		// Also register the cassette under the request type key for automatic
		// lookup
		// Register common mappings
		switch testName {
		case "test_get_all_mids":
			client.registerCassette("allMids", testName)
		case "test_get_user_state":
			client.registerCassette("clearinghouseState", testName)
		case "test_get_open_orders":
			client.registerCassette("openOrders", testName)
		case "test_get_user_fills":
			client.registerCassette("userFills", testName)
		case "test_get_user_fills_by_time":
			client.registerCassette("userFillsByTime", testName)
		case "test_get_info":
			client.registerCassette("meta", testName)
		case "test_get_funding_history[None]":
			client.registerCassette("fundingHistory", testName)
		case "test_get_l2_snapshot":
			client.registerCassette("l2Book", testName)
		case "test_get_candles_snapshot":
			client.registerCassette("candleSnapshot", testName)
		case "test_user_funding_history_with_end_time":
			client.registerCassette("userFunding", testName)
		case "test_spot_user_state":
			client.registerCassette("spotClearinghouseState", testName)
		case "test_user_fees":
			client.registerCassette("userFees", testName)
		}
	}

	return client
}

// loadCassetteFile loads a cassette JSON file
func loadCassetteFile(name string) ([]byte, error) {
	filename := fmt.Sprintf("cassettes/%s.json", name)
	return os.ReadFile(filename)
}

// ===== Suite definition =====

type InfoCassetteSuite struct{}

func (s *InfoCassetteSuite) Setup(t *td.T) error {
	return nil
}

func TestInfoCassetteSuite(t *testing.T) {
	tdsuite.Run(t, &InfoCassetteSuite{})
}

// ===== Cassette-Based Tests as suite methods =====

func (s *InfoCassetteSuite) TestAllMids(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_all_mids")
	info := &Info{rest: client}

	mids, err := info.AllMids(context.Background(), "")
	require.CmpNoError(err)
	require.NotNil(mids)

	// From Python test: checks for BTC, ETH, ATOM, MATIC
	require.Cmp(mids["BTC"], 30135.0)
	require.Cmp(mids["ETH"], 1903.95)
	require.ContainsKey(mids, "ATOM")
	require.ContainsKey(mids, "MATIC")
}

func (s *InfoCassetteSuite) TestUserState(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_user_state")
	info := &Info{rest: client}

	response, err := info.UserState(
		context.Background(),
		common.HexToAddress("0x5e9ee1089755c3435139848e47e6635505d5a13a"),
		"",
	)
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: checks assetPositions length and marginSummary
	require.Cmp(len(response.AssetPositions), 12)
	require.Cmp(response.MarginSummary.AccountValue.Raw(), 1182.312496)
}

func (s *InfoCassetteSuite) TestOpenOrders(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_open_orders")
	info := &Info{rest: client}

	response, err := info.OpenOrders(
		context.Background(),
		common.HexToAddress("0x5e9ee1089755c3435139848e47e6635505d5a13a"),
		"",
	)
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: checks response length
	require.Cmp(len(response), 196)
}

func (s *InfoCassetteSuite) TestAllMidsWithNames(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_all_mids")
	info := &Info{rest: client}

	response, err := info.AllMids(context.Background(), "")
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: test_get_all_mids checks for asset names
	coins := []string{"BTC", "ETH", "ATOM", "MATIC"}
	for _, coin := range coins {
		require.ContainsKey(response, coin)
	}
}

func (s *InfoCassetteSuite) TestAllMidsCoinsPresent(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_all_mids")
	info := &Info{rest: client}

	mids, err := info.AllMids(context.Background(), "")
	require.CmpNoError(err)
	require.NotNil(mids)

	// From Python test: checks for BTC, ETH, ATOM, MATIC
	require.ContainsKey(mids, "BTC")
	require.ContainsKey(mids, "ETH")
	require.ContainsKey(mids, "ATOM")
	require.ContainsKey(mids, "MATIC")
}

func (s *InfoCassetteSuite) TestUserFills(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_user_fills")
	info := &Info{rest: client}

	response, err := info.UserFills(
		context.Background(),
		common.HexToAddress("0xb7b6f3cea3f66bf525f5d8f965f6dbf6d9b017b2"),
	)
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: checks response is a list and first fill has "crossed"
	// = true
	require.Gt(len(response), 0)
	require.NotEmpty(response[0].Coin)
}

func (s *InfoCassetteSuite) TestUserFillsByTime(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_user_fills_by_time")
	info := &Info{rest: client}

	response, err := info.UserFillsByTime(
		context.Background(),
		common.HexToAddress("0xb7b6f3cea3f66bf525f5d8f965f6dbf6d9b017b2"),
		1683245555699,
		nil,
		true,
	)
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: checks response length is 500
	require.Cmp(len(response), 500)
}

func (s *InfoCassetteSuite) TestMeta(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_info")
	info := &Info{rest: client}

	response, err := info.Meta(context.Background(), "")
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: checks universe length and first asset
	require.Cmp(len(response.Universe), 28)
	require.Gt(len(response.Universe), 0)
	require.Cmp(response.Universe[0].Name, "BTC")
	require.Cmp(response.Universe[0].SzDecimals, int64(5))
}

func (s *InfoCassetteSuite) TestFundingHistory(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_funding_history[None]")
	info := &Info{rest: client}

	response, err := info.FundingHistory(
		context.Background(),
		"BTC",
		1681923833000,
		nil,
	)
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: checks response is non-empty and has correct structure
	require.Gt(len(response), 0)
	require.Cmp(response[0].Coin, "BTC")
	// Check expected fields exist
	require.NotZero(response[0].FundingRate)
	require.NotZero(response[0].Premium)
	require.NotZero(response[0].Time)
}

func (s *InfoCassetteSuite) TestL2Snapshot(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_l2_snapshot")
	info := &Info{rest: client, nameToCoin: map[string]string{"DYDX": "DYDX"}}

	response, err := info.L2Snapshot(context.Background(), "DYDX")
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: checks response structure
	require.Cmp(len(response.Levels), 2)
	require.Cmp(response.Coin, "DYDX")
	require.Gt(len(response.Levels[0]), 0)
	require.Gt(len(response.Levels[1]), 0)
}

func (s *InfoCassetteSuite) TestCandlesSnapshot(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_get_candles_snapshot")
	info := &Info{rest: client, nameToCoin: map[string]string{"kPEPE": "kPEPE"}}

	response, err := info.CandlesSnapshot(
		context.Background(),
		"kPEPE",
		"1h",
		1684702007000,
		1684784807000,
	)
	require.CmpNoError(err)
	require.NotNil(response)

	// From Python test: checks response length and structure
	require.Cmp(len(response), 24)
	// Check expected candle fields
	require.Gt(len(response), 0)
	candle := response[0]
	require.NotZero(candle.T)
	require.NotZero(candle.O)
	require.NotZero(candle.C)
}

func (s *InfoCassetteSuite) TestUserFundingHistory(assert, require *td.T) {
	client := loadCassettes(
		require.TB,
		"test_user_funding_history_with_end_time",
	)
	info := &Info{rest: client}

	// UserFundingHistory takes int64 values, not pointers
	var endTime time.Time = time.UnixMilli(1682010233000)
	response, err := info.UserFundingHistory(
		context.Background(),
		common.HexToAddress("0xb7b6f3cea3f66bf525f5d8f965f6dbf6d9b017b2"),
		time.UnixMilli(1681923833000),
		&endTime,
	)

	require.CmpNoError(err)
	require.NotNil(response)
	require.Cmp(len(response), 13, "Unexpected number of responses")
}

func (s *InfoCassetteSuite) TestSpotUserState(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_spot_user_state")
	info := &Info{rest: client}

	response, err := info.SpotUserState(
		context.Background(),
		common.HexToAddress("0x5e9ee1089755c3435139848e47e6635505d5a13a"),
	)
	require.CmpNoError(err)
	require.NotNil(response)

	// From cassette: checks balances length and structure
	require.Cmp(len(response.Balances), 2)
	require.Cmp(response.Balances[0].Coin, "USDC")
	require.NotZero(response.Balances[0].Total)
	require.Cmp(response.Balances[1].Coin, "UETH")
	require.NotZero(response.Balances[1].EntryNtl)
}

func (s *InfoCassetteSuite) TestUserFees(assert, require *td.T) {
	client := loadCassettes(require.TB, "test_user_fees")
	info := &Info{rest: client}

	feeInfo, err := info.UserFees(
		context.Background(),
		common.HexToAddress("0xb7b6f3cea3f66bf525f5d8f965f6dbf6d9b017b2"),
	)
	require.CmpNoError(err)

	// From cassette: checks daily volume data
	require.Cmp(len(feeInfo.DailyUserVlm), 15)
	require.Cmp(feeInfo.DailyUserVlm[0].Date, "2025-11-15")
	require.NotZero(feeInfo.DailyUserVlm[0].Exchange)

	// Check fee schedule structure
	require.NotZero(feeInfo.FeeSchedule.Cross)
	require.Cmp(len(feeInfo.FeeSchedule.Tiers.Vip), 6)
	require.Cmp(len(feeInfo.FeeSchedule.Tiers.Mm), 3)
	require.Cmp(len(feeInfo.FeeSchedule.StakingDiscountTiers), 7)

	// Check user's current rates
	require.NotZero(feeInfo.UserCrossRate)
	require.NotZero(feeInfo.UserAddRate)
	require.NotZero(feeInfo.UserSpotCrossRate)
	require.NotZero(feeInfo.UserSpotAddRate)

	// Check active staking discount
	require.NotNil(feeInfo.ActiveStakingDiscount)
}
