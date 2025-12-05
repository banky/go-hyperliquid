package info

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/banky/go-hyperliquid/internal/utils"
	"github.com/banky/go-hyperliquid/types"
	"github.com/banky/go-hyperliquid/ws"
	"github.com/ethereum/go-ethereum/common"
	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
)

// ===== Suite wiring =====

type InfoSuite struct{}

func TestInfoSuite(t *testing.T) {
	tdsuite.Run(t, &InfoSuite{})
}

// Mock REST client for testing
type mockRestClient struct {
	postFunc func(ctx context.Context, path string, body any, result any) error
}

func (m *mockRestClient) Post(
	ctx context.Context,
	path string,
	body any,
	result any,
) error {
	return m.postFunc(ctx, path, body, result)
}

func (m *mockRestClient) BaseUrl() string {
	return ""
}

func (m *mockRestClient) IsMainnet() bool {
	return false
}

func (m *mockRestClient) NetworkName() string {
	return "Testnet"
}

// Mock WS client for testing
type mockWsClient struct {
	startFunc                   func(ctx context.Context) error
	stopFunc                    func()
	subscribeAllMidsFunc        func(ctx context.Context, ch chan<- ws.AllMidsMessage) (ws.Subscription, error)
	subscribeL2BookFunc         func(ctx context.Context, coin string, ch chan<- ws.L2BookMessage) (ws.Subscription, error)
	subscribeTradesFunc         func(ctx context.Context, coin string, ch chan<- ws.TradesMessage) (ws.Subscription, error)
	subscribeCandleFunc         func(ctx context.Context, coin string, interval string, ch chan<- ws.CandleMessage) (ws.Subscription, error)
	subscribeBboFunc            func(ctx context.Context, coin string, ch chan<- ws.BboMessage) (ws.Subscription, error)
	subscribeActiveAssetCtxFunc func(ctx context.Context, coin string, ch chan<- ws.ActiveAssetCtxMessage) (ws.Subscription, error)
	subscribeUserEventsFunc     func(ctx context.Context, user common.Address, ch chan<- ws.UserEventsMessage) (ws.Subscription, error)
	subscribeUserFillsFunc      func(ctx context.Context, user string, ch chan<- ws.UserFillsMessage) (ws.Subscription, error)
	subscribeOrderUpdatesFunc   func(ctx context.Context, user string, ch chan<- ws.OrderUpdatesMessage) (ws.Subscription, error)
}

var _ ws.ClientInterface = (*mockWsClient)(nil)

func (m *mockWsClient) Start(ctx context.Context) error {
	if m.startFunc != nil {
		return m.startFunc(ctx)
	}
	return nil
}

func (m *mockWsClient) Close() {
	if m.stopFunc != nil {
		m.stopFunc()
	}
}

func (m *mockWsClient) SubscribeAllMids(
	ctx context.Context,
	ch chan<- ws.AllMidsMessage,
) (ws.Subscription, error) {
	if m.subscribeAllMidsFunc != nil {
		return m.subscribeAllMidsFunc(ctx, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeL2Book(
	ctx context.Context,
	coin string,
	ch chan<- ws.L2BookMessage,
) (ws.Subscription, error) {
	if m.subscribeL2BookFunc != nil {
		return m.subscribeL2BookFunc(ctx, coin, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeTrades(
	ctx context.Context,
	coin string,
	ch chan<- ws.TradesMessage,
) (ws.Subscription, error) {
	if m.subscribeTradesFunc != nil {
		return m.subscribeTradesFunc(ctx, coin, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeCandle(
	ctx context.Context,
	coin string,
	interval string,
	ch chan<- ws.CandleMessage,
) (ws.Subscription, error) {
	if m.subscribeCandleFunc != nil {
		return m.subscribeCandleFunc(ctx, coin, interval, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeBbo(
	ctx context.Context,
	coin string,
	ch chan<- ws.BboMessage,
) (ws.Subscription, error) {
	if m.subscribeBboFunc != nil {
		return m.subscribeBboFunc(ctx, coin, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeActiveAssetCtx(
	ctx context.Context,
	coin string,
	ch chan<- ws.ActiveAssetCtxMessage,
) (ws.Subscription, error) {
	if m.subscribeActiveAssetCtxFunc != nil {
		return m.subscribeActiveAssetCtxFunc(ctx, coin, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeUserEvents(
	ctx context.Context,
	user common.Address,
	ch chan<- ws.UserEventsMessage,
) (ws.Subscription, error) {
	if m.subscribeUserEventsFunc != nil {
		return m.subscribeUserEventsFunc(ctx, user, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeUserFills(
	ctx context.Context,
	user string,
	ch chan<- ws.UserFillsMessage,
) (ws.Subscription, error) {
	if m.subscribeUserFillsFunc != nil {
		return m.subscribeUserFillsFunc(ctx, user, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeOrderUpdates(
	ctx context.Context,
	user string,
	ch chan<- ws.OrderUpdatesMessage,
) (ws.Subscription, error) {
	if m.subscribeOrderUpdatesFunc != nil {
		return m.subscribeOrderUpdatesFunc(ctx, user, ch)
	}
	return nil, nil
}

// ===== REST API Tests =====

func (s *InfoSuite) TestAllMidsSuccess(assert, require *td.T) {
	expectedMids := map[string]string{
		"BTC": "45000.50",
		"ETH": "3000.25",
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				require.Cmp(path, "/info", "expected path /info")
				req := body.(map[string]any)
				require.Cmp(req["type"], "allMids")
				require.Cmp(req["dex"], "testdex")

				// Simulate response
				*result.(*map[string]string) = expectedMids
				return nil
			},
		},
	}

	mids, err := info.AllMids(context.Background(), "testdex")
	require.CmpNoError(err)

	require.Cmp(len(mids), len(expectedMids))
	for k, v := range expectedMids {
		s, err := utils.StringToFloat(v)
		require.CmpNoError(err, "expected valid float for %q", v)
		require.Cmp(mids[k], s)
	}
}

func (s *InfoSuite) TestAllMidsError(assert, require *td.T) {
	expectedErr := errors.New("network error")
	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				return expectedErr
			},
		},
	}

	_, err := info.AllMids(context.Background(), "testdex")
	require.Cmp(err, expectedErr)
}

func (s *InfoSuite) TestL2SnapshotSuccess(assert, require *td.T) {
	expectedSnapshot := &L2BookSnapshot{
		Coin: "BTC",
		Levels: [2][]L2Level{
			{
				{Px: 45000.00, Sz: 1.5, N: 3},
				{Px: 44999.00, Sz: 2.0, N: 5},
			},
			{
				{Px: 45001.00, Sz: 1.0, N: 2},
				{Px: 45002.00, Sz: 3.0, N: 4},
			},
		},
		Time: 1234567890,
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				require.Cmp(path, "/info", "expected path /info")
				req := body.(map[string]any)
				require.Cmp(req["type"], "l2Book")
				require.Cmp(req["coin"], "BTC")
				*result.(*L2BookSnapshot) = *expectedSnapshot
				return nil
			},
		},
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	snapshot, err := info.L2Snapshot(context.Background(), "BTC")
	require.CmpNoError(err)

	require.Cmp(snapshot.Coin, expectedSnapshot.Coin)
	require.Cmp(snapshot.Time, expectedSnapshot.Time)
}

func (s *InfoSuite) TestL2SnapshotNameMapping(assert, require *td.T) {
	expectedSnapshot := &L2BookSnapshot{
		Coin:   "BTC",
		Levels: [2][]L2Level{},
		Time:   1234567890,
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				require.Cmp(req["coin"], "BTC")
				*result.(*L2BookSnapshot) = *expectedSnapshot
				return nil
			},
		},
		nameToCoin: map[string]string{"Bitcoin": "BTC"},
	}

	// Call with mapped name
	snapshot, err := info.L2Snapshot(context.Background(), "Bitcoin")
	require.CmpNoError(err)
	require.Cmp(snapshot.Coin, expectedSnapshot.Coin)
}

func (s *InfoSuite) TestMetaSuccess(assert, require *td.T) {
	expectedMeta := &Meta{
		Universe: []AssetInfo{
			{Name: "BTC", SzDecimals: 8},
			{Name: "ETH", SzDecimals: 8},
		},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				require.Cmp(path, "/info", "expected path /info")
				req := body.(map[string]any)
				require.Cmp(req["type"], "meta")
				require.Cmp(req["dex"], "mainnet")
				*result.(*Meta) = *expectedMeta
				return nil
			},
		},
	}

	meta, err := info.Meta(context.Background(), "mainnet")
	require.CmpNoError(err)
	require.Cmp(len(meta.Universe), len(expectedMeta.Universe))
}

func (s *InfoSuite) TestSpotMetaSuccess(assert, require *td.T) {
	expectedMeta := &SpotMeta{
		Universe: []SpotAssetInfo{
			{Name: "USDC", Tokens: [2]int64{0, 1}, Index: 0, IsCanonical: true},
		},
		Tokens: []SpotTokenInfo{
			{
				Name:        "USDC",
				SzDecimals:  6,
				WeiDecimals: 6,
				Index:       0,
				TokenId:     "0x1",
				IsCanonical: true,
			},
		},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				require.Cmp(req["type"], "spotMeta")
				*result.(*SpotMeta) = *expectedMeta
				return nil
			},
		},
	}

	meta, err := info.SpotMeta(context.Background())
	require.CmpNoError(err)
	require.Cmp(len(meta.Universe), 1)
}

func (s *InfoSuite) TestUserStateSuccess(assert, require *td.T) {
	expectedState := &UserState{
		AssetPositions: []AssetPosition{
			{
				Position: Position{
					Coin:          "BTC",
					EntryPx:       ptr(types.FloatString(45000)),
					MarginUsed:    1000,
					PositionValue: 45000,
					Szi:           1,
				},
				Type: "perp",
			},
		},
		Withdrawable: 50000,
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				require.Cmp(req["type"], "clearinghouseState")
				require.Cmp(req["user"], common.HexToAddress("0x123"))
				require.Cmp(req["dex"], "mainnet")
				*result.(*UserState) = *expectedState
				return nil
			},
		},
	}

	state, err := info.UserState(
		context.Background(),
		common.HexToAddress("0x123"),
		"mainnet",
	)
	require.CmpNoError(err)

	require.Cmp(len(state.AssetPositions), 1)
	require.Cmp(state.Withdrawable.Raw(), 50000.00)
}

func (s *InfoSuite) TestOpenOrdersSuccess(assert, require *td.T) {
	expectedOrders := []OpenOrder{
		{
			Coin:      "BTC",
			LimitPx:   45000,
			Oid:       1,
			Side:      "A",
			Sz:        1,
			Timestamp: 1234567890,
		},
		{
			Coin:      "ETH",
			LimitPx:   3000,
			Oid:       2,
			Side:      "B",
			Sz:        10,
			Timestamp: 1234567891,
		},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				require.Cmp(req["type"], "openOrders")
				*result.(*[]OpenOrder) = expectedOrders
				return nil
			},
		},
	}

	orders, err := info.OpenOrders(
		context.Background(),
		common.HexToAddress("0x123"),
		"mainnet",
	)
	require.CmpNoError(err)
	require.Cmp(len(orders), len(expectedOrders))
}

func (s *InfoSuite) TestUserFillsSuccess(assert, require *td.T) {
	expectedFills := []Fill{
		{
			Coin: "BTC",
			Px:   45000,
			Sz:   1,
			Side: "A",
			Time: 1234567890,
			Oid:  1,
		},
		{
			Coin: "ETH",
			Px:   3000,
			Sz:   10,
			Side: "B",
			Time: 1234567891,
			Oid:  2,
		},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				require.Cmp(req["type"], "userFills")
				*result.(*[]Fill) = expectedFills
				return nil
			},
		},
	}

	fills, err := info.UserFills(
		context.Background(),
		common.HexToAddress("0x123"),
	)
	require.CmpNoError(err)
	require.Cmp(len(fills), len(expectedFills))
}

func (s *InfoSuite) TestUserFillsByTimeSuccess(assert, require *td.T) {
	expectedFills := []Fill{
		{
			Coin: "BTC",
			Px:   45000,
			Sz:   1,
			Side: "A",
			Time: 1234567890,
			Oid:  1,
		},
	}
	endTime := int64(1234567900)

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				require.Cmp(req["type"], "userFillsByTime")
				require.Cmp(req["startTime"], int64(1234567880))
				require.Cmp(req["endTime"], endTime)
				require.Cmp(req["aggregateByTime"], true)
				*result.(*[]Fill) = expectedFills
				return nil
			},
		},
	}

	fills, err := info.UserFillsByTime(
		context.Background(),
		common.HexToAddress("0x123"),
		1234567880,
		&endTime,
		true,
	)
	require.CmpNoError(err)
	require.Cmp(len(fills), 1)
}

func (s *InfoSuite) TestFundingHistorySuccess(assert, require *td.T) {
	expectedHistory := []FundingRecord{
		{Coin: "BTC", FundingRate: 0.0001, Premium: 100, Time: 1234567890},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				require.Cmp(req["type"], "fundingHistory")
				require.Cmp(req["coin"], "BTC")
				*result.(*[]FundingRecord) = expectedHistory
				return nil
			},
		},
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	history, err := info.FundingHistory(
		context.Background(),
		"BTC",
		1234567880,
		nil,
	)
	require.CmpNoError(err)
	require.Cmp(len(history), 1)
}

func (s *InfoSuite) TestCandlesSnapshotSuccess(assert, require *td.T) {
	expectedCandles := []Candle{
		{
			T: 1234567890,
			O: "45000",
			C: "45500",
			H: "46000",
			L: "44500",
			V: "100",
			N: 50,
			S: "BTC",
			I: "1h",
		},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				require.Cmp(req["type"], "candleSnapshot")
				*result.(*[]Candle) = expectedCandles
				return nil
			},
		},
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	candles, err := info.CandlesSnapshot(
		context.Background(),
		"BTC",
		"1h",
		1234567880,
		1234567890,
	)
	require.CmpNoError(err)
	require.Cmp(len(candles), 1)
}

// ===== WebSocket Subscription Tests =====

func (s *InfoSuite) TestSubscribeAllMidsNoWS(assert, require *td.T) {
	info := &Info{}

	ch := make(chan ws.AllMidsMessage)
	_, err := info.SubscribeAllMids(context.Background(), ch)
	require.CmpError(err)
	require.Cmp(err.Error(), "websocket not initialized")
}

func (s *InfoSuite) TestSubscribeAllMidsSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeAllMidsFunc: func(ctx context.Context, ch chan<- ws.AllMidsMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{ws: mockWS}

	ch := make(chan ws.AllMidsMessage)
	sub, err := info.SubscribeAllMids(context.Background(), ch)
	require.CmpNoError(err)
	require.NotNil(sub)
}

func (s *InfoSuite) TestSubscribeL2BookSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeL2BookFunc: func(ctx context.Context, coin string, ch chan<- ws.L2BookMessage) (ws.Subscription, error) {
			require.Cmp(coin, "BTC")
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	ch := make(chan ws.L2BookMessage)
	sub, err := info.SubscribeL2Book(context.Background(), "BTC", ch)
	require.CmpNoError(err)
	require.NotNil(sub)
}

func (s *InfoSuite) TestSubscribeTradesSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeTradesFunc: func(ctx context.Context, coin string, ch chan<- ws.TradesMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"ETH": "ETH"},
	}

	ch := make(chan ws.TradesMessage)
	sub, err := info.SubscribeTrades(context.Background(), "ETH", ch)
	require.CmpNoError(err)
	require.NotNil(sub)
}

func (s *InfoSuite) TestSubscribeCandleSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeCandleFunc: func(ctx context.Context, coin string, interval string, ch chan<- ws.CandleMessage) (ws.Subscription, error) {
			require.Cmp(interval, "1h")
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	ch := make(chan ws.CandleMessage)
	sub, err := info.SubscribeCandle(context.Background(), "BTC", "1h", ch)
	require.CmpNoError(err)
	require.NotNil(sub)
}

func (s *InfoSuite) TestSubscribeBboSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeBboFunc: func(ctx context.Context, coin string, ch chan<- ws.BboMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	ch := make(chan ws.BboMessage)
	sub, err := info.SubscribeBbo(context.Background(), "BTC", ch)
	require.CmpNoError(err)
	require.NotNil(sub)
}

func (s *InfoSuite) TestSubscribeActiveAssetCtxSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeActiveAssetCtxFunc: func(ctx context.Context, coin string, ch chan<- ws.ActiveAssetCtxMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	ch := make(chan ws.ActiveAssetCtxMessage)
	sub, err := info.SubscribeActiveAssetCtx(context.Background(), "BTC", ch)
	require.CmpNoError(err)
	require.NotNil(sub)
}

func (s *InfoSuite) TestSubscribeUserEventsSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeUserEventsFunc: func(ctx context.Context, user common.Address, ch chan<- ws.UserEventsMessage) (ws.Subscription, error) {
			require.Cmp(user, common.HexToAddress("0x123"))
			return &mockSubscription{}, nil
		},
	}

	info := &Info{ws: mockWS}

	ch := make(chan ws.UserEventsMessage)
	sub, err := info.SubscribeUserEvents(
		context.Background(),
		common.HexToAddress("0x123"),
		ch,
	)
	require.CmpNoError(err)
	require.NotNil(sub)
}

func (s *InfoSuite) TestSubscribeUserFillsSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeUserFillsFunc: func(ctx context.Context, user string, ch chan<- ws.UserFillsMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{ws: mockWS}

	ch := make(chan ws.UserFillsMessage)
	sub, err := info.SubscribeUserFills(context.Background(), "0x456", ch)
	require.CmpNoError(err)
	require.NotNil(sub)
}

func (s *InfoSuite) TestSubscribeOrderUpdatesSuccess(assert, require *td.T) {
	mockWS := &mockWsClient{
		subscribeOrderUpdatesFunc: func(ctx context.Context, user string, ch chan<- ws.OrderUpdatesMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{ws: mockWS}

	ch := make(chan ws.OrderUpdatesMessage)
	sub, err := info.SubscribeOrderUpdates(context.Background(), "0x789", ch)
	require.CmpNoError(err)
	require.NotNil(sub)
}

// ===== Coin/Asset Management Tests =====

func (s *InfoSuite) TestSetCoinMapping(assert, require *td.T) {
	info := &Info{
		nameToCoin: make(map[string]string),
	}

	coins := []string{"BTC", "ETH", "SOL"}
	info.SetCoinMapping(coins)

	for _, coin := range coins {
		val, ok := info.nameToCoin[coin]
		require.True(ok, "expected mapping for %s", coin)
		require.Cmp(val, coin)
	}
}

func (s *InfoSuite) TestGetCoinFromNameFound(assert, require *td.T) {
	info := &Info{
		nameToCoin: map[string]string{
			"Bitcoin":  "BTC",
			"Ethereum": "ETH",
		},
	}

	require.Cmp(info.getCoinFromName("Bitcoin"), "BTC")
	require.Cmp(info.getCoinFromName("Ethereum"), "ETH")
}

func (s *InfoSuite) TestGetCoinFromNameNotFound(assert, require *td.T) {
	info := &Info{
		nameToCoin: map[string]string{},
	}

	// When not found, should return the name as-is
	require.Cmp(info.getCoinFromName("BTC"), "BTC")
}

func (s *InfoSuite) TestGetAssetFound(assert, require *td.T) {
	info := &Info{
		coinToAsset: map[string]int64{"BTC": 0, "ETH": 1},
		nameToCoin:  map[string]string{"Bitcoin": "BTC"},
	}

	asset, ok := info.GetAsset("Bitcoin")
	require.True(ok, "expected asset to be found")
	require.Cmp(asset, int64(0))
}

func (s *InfoSuite) TestGetAssetNotFound(assert, require *td.T) {
	info := &Info{
		coinToAsset: map[string]int64{},
		nameToCoin:  map[string]string{},
	}

	_, ok := info.GetAsset("UNKNOWN")
	require.False(ok, "expected asset not to be found")
}

func (s *InfoSuite) TestPullRealData(assert, require *td.T) {
	// Manual test
	tb := require.TB
	tb.Skip("manual integration test")

	info, err := New(Config{})
	require.CmpNoError(err)

	mids, err := info.AllMids(context.Background(), "")
	require.CmpNoError(err)
	require.True(len(mids) > 0, "expected non-zero length of mids")

	midsChan := make(chan ws.AllMidsMessage)
	sub, err := info.SubscribeAllMids(context.Background(), midsChan)
	require.CmpNoError(err)
	defer sub.Unsubscribe()

	// Use a timeout context so we don't block forever
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messageCount := 0
	for {
		select {
		case <-midsChan:
			messageCount++
			if messageCount >= 3 {
				return
			}
		case <-ctx.Done():
			require.True(
				false,
				"timeout waiting for messages after 10s, got %d messages",
				messageCount,
			)
		}
	}
}

// ===== Helper Functions =====

func ptr[T any](s T) *T {
	return &s
}

// Mock subscription for testing
type mockSubscription struct{}

func (m *mockSubscription) Unsubscribe() {}

func (m *mockSubscription) Err() <-chan error {
	return make(chan error)
}
