package info

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/banky/go-hyperliquid/rest"
	"github.com/banky/go-hyperliquid/ws"
)

// Mock REST client for testing
type mockRestClient struct {
	postFunc func(ctx context.Context, path string, body any, result any) error
}

var _ rest.ClientInterface = (*mockRestClient)(nil)

func (m *mockRestClient) Post(ctx context.Context, path string, body any, result any) error {
	return m.postFunc(ctx, path, body, result)
}

// Mock WS client for testing
type mockWsClient struct {
	startFunc                   func(ctx context.Context) error
	stopFunc                    func()
	subscribeAllMidsFunc        func(ctx context.Context, ch chan ws.AllMidsMessage) (ws.Subscription, error)
	subscribeL2BookFunc         func(ctx context.Context, coin string, ch chan ws.L2BookMessage) (ws.Subscription, error)
	subscribeTradesFunc         func(ctx context.Context, coin string, ch chan ws.TradesMessage) (ws.Subscription, error)
	subscribeCandleFunc         func(ctx context.Context, coin string, interval string, ch chan ws.CandleMessage) (ws.Subscription, error)
	subscribeBboFunc            func(ctx context.Context, coin string, ch chan ws.BboMessage) (ws.Subscription, error)
	subscribeActiveAssetCtxFunc func(ctx context.Context, coin string, ch chan ws.ActiveAssetCtxMessage) (ws.Subscription, error)
	subscribeUserEventsFunc     func(ctx context.Context, user string, ch chan ws.UserEventsMessage) (ws.Subscription, error)
	subscribeUserFillsFunc      func(ctx context.Context, user string, ch chan ws.UserFillsMessage) (ws.Subscription, error)
	subscribeOrderUpdatesFunc   func(ctx context.Context, user string, ch chan ws.OrderUpdatesMessage) (ws.Subscription, error)
}

var _ ws.ClientInterface = (*mockWsClient)(nil)

func (m *mockWsClient) Start(ctx context.Context) error {
	if m.startFunc != nil {
		return m.startFunc(ctx)
	}
	return nil
}

func (m *mockWsClient) Stop() {
	if m.stopFunc != nil {
		m.stopFunc()
	}
}

func (m *mockWsClient) SubscribeAllMids(ctx context.Context, ch chan ws.AllMidsMessage) (ws.Subscription, error) {
	if m.subscribeAllMidsFunc != nil {
		return m.subscribeAllMidsFunc(ctx, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeL2Book(ctx context.Context, coin string, ch chan ws.L2BookMessage) (ws.Subscription, error) {
	if m.subscribeL2BookFunc != nil {
		return m.subscribeL2BookFunc(ctx, coin, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeTrades(ctx context.Context, coin string, ch chan ws.TradesMessage) (ws.Subscription, error) {
	if m.subscribeTradesFunc != nil {
		return m.subscribeTradesFunc(ctx, coin, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeCandle(ctx context.Context, coin string, interval string, ch chan ws.CandleMessage) (ws.Subscription, error) {
	if m.subscribeCandleFunc != nil {
		return m.subscribeCandleFunc(ctx, coin, interval, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeBbo(ctx context.Context, coin string, ch chan ws.BboMessage) (ws.Subscription, error) {
	if m.subscribeBboFunc != nil {
		return m.subscribeBboFunc(ctx, coin, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeActiveAssetCtx(ctx context.Context, coin string, ch chan ws.ActiveAssetCtxMessage) (ws.Subscription, error) {
	if m.subscribeActiveAssetCtxFunc != nil {
		return m.subscribeActiveAssetCtxFunc(ctx, coin, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeUserEvents(ctx context.Context, user string, ch chan ws.UserEventsMessage) (ws.Subscription, error) {
	if m.subscribeUserEventsFunc != nil {
		return m.subscribeUserEventsFunc(ctx, user, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeUserFills(ctx context.Context, user string, ch chan ws.UserFillsMessage) (ws.Subscription, error) {
	if m.subscribeUserFillsFunc != nil {
		return m.subscribeUserFillsFunc(ctx, user, ch)
	}
	return nil, nil
}

func (m *mockWsClient) SubscribeOrderUpdates(ctx context.Context, user string, ch chan ws.OrderUpdatesMessage) (ws.Subscription, error) {
	if m.subscribeOrderUpdatesFunc != nil {
		return m.subscribeOrderUpdatesFunc(ctx, user, ch)
	}
	return nil, nil
}

// ===== REST API Tests =====

func TestAllMidsSuccess(t *testing.T) {
	expectedMids := map[string]string{
		"BTC": "45000.50",
		"ETH": "3000.25",
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				if path != "/info" {
					t.Errorf("expected path /info, got %s", path)
				}
				req := body.(map[string]any)
				if req["type"] != "allMids" {
					t.Errorf("expected type allMids, got %v", req["type"])
				}
				if req["dex"] != "testdex" {
					t.Errorf("expected dex testdex, got %v", req["dex"])
				}
				// Simulate response
				*result.(*map[string]string) = expectedMids
				return nil
			},
		},
	}

	mids, err := info.AllMids(context.Background(), "testdex")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(mids) != len(expectedMids) {
		t.Errorf("expected %d mids, got %d", len(expectedMids), len(mids))
	}
	for k, v := range expectedMids {
		if mids[k] != v {
			t.Errorf("expected mids[%s]=%s, got %s", k, v, mids[k])
		}
	}
}

func TestAllMidsError(t *testing.T) {
	expectedErr := errors.New("network error")
	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				return expectedErr
			},
		},
	}

	_, err := info.AllMids(context.Background(), "testdex")
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestL2SnapshotSuccess(t *testing.T) {
	expectedSnapshot := &L2BookSnapshot{
		Coin: "BTC",
		Levels: [2][]L2Level{
			{
				{Px: "45000.00", Sz: "1.5", N: 3},
				{Px: "44999.00", Sz: "2.0", N: 5},
			},
			{
				{Px: "45001.00", Sz: "1.0", N: 2},
				{Px: "45002.00", Sz: "3.0", N: 4},
			},
		},
		Time: 1234567890,
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				if path != "/info" {
					t.Errorf("expected path /info, got %s", path)
				}
				req := body.(map[string]any)
				if req["type"] != "l2Book" {
					t.Errorf("expected type l2Book, got %v", req["type"])
				}
				if req["coin"] != "BTC" {
					t.Errorf("expected coin BTC, got %v", req["coin"])
				}
				*result.(*L2BookSnapshot) = *expectedSnapshot
				return nil
			},
		},
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	snapshot, err := info.L2Snapshot(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.Coin != expectedSnapshot.Coin {
		t.Errorf("expected coin %s, got %s", expectedSnapshot.Coin, snapshot.Coin)
	}
	if snapshot.Time != expectedSnapshot.Time {
		t.Errorf("expected time %d, got %d", expectedSnapshot.Time, snapshot.Time)
	}
}

func TestL2SnapshotNameMapping(t *testing.T) {
	expectedSnapshot := &L2BookSnapshot{
		Coin:   "BTC",
		Levels: [2][]L2Level{},
		Time:   1234567890,
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["coin"] != "BTC" {
					t.Errorf("expected coin BTC, got %v", req["coin"])
				}
				*result.(*L2BookSnapshot) = *expectedSnapshot
				return nil
			},
		},
		nameToCoin: map[string]string{"Bitcoin": "BTC"},
	}

	// Call with mapped name
	snapshot, err := info.L2Snapshot(context.Background(), "Bitcoin")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if snapshot.Coin != expectedSnapshot.Coin {
		t.Errorf("expected coin %s, got %s", expectedSnapshot.Coin, snapshot.Coin)
	}
}

func TestMetaSuccess(t *testing.T) {
	expectedMeta := &Meta{
		Universe: []AssetInfo{
			{Name: "BTC", SzDecimals: 8},
			{Name: "ETH", SzDecimals: 8},
		},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				if path != "/info" {
					t.Errorf("expected path /info, got %s", path)
				}
				req := body.(map[string]any)
				if req["type"] != "meta" {
					t.Errorf("expected type meta, got %v", req["type"])
				}
				if req["dex"] != "mainnet" {
					t.Errorf("expected dex mainnet, got %v", req["dex"])
				}
				*result.(*Meta) = *expectedMeta
				return nil
			},
		},
	}

	meta, err := info.Meta(context.Background(), "mainnet")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(meta.Universe) != len(expectedMeta.Universe) {
		t.Errorf("expected %d assets, got %d", len(expectedMeta.Universe), len(meta.Universe))
	}
}

func TestSpotMetaSuccess(t *testing.T) {
	expectedMeta := &SpotMeta{
		Universe: []SpotAssetInfo{
			{Name: "USDC", Tokens: [2]int{0, 1}, Index: 0, IsCanonical: true},
		},
		Tokens: []SpotTokenInfo{
			{Name: "USDC", SzDecimals: 6, WeiDecimals: 6, Index: 0, TokenId: "0x1", IsCanonical: true},
		},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["type"] != "spotMeta" {
					t.Errorf("expected type spotMeta, got %v", req["type"])
				}
				*result.(*SpotMeta) = *expectedMeta
				return nil
			},
		},
	}

	meta, err := info.SpotMeta(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(meta.Universe) != 1 {
		t.Errorf("expected 1 asset, got %d", len(meta.Universe))
	}
}

func TestUserStateSuccess(t *testing.T) {
	expectedState := &UserState{
		AssetPositions: []AssetPosition{
			{
				Position: Position{
					Coin:          "BTC",
					EntryPx:       strPtr("45000"),
					MarginUsed:    "1000",
					PositionValue: "45000",
					Szi:           "1",
				},
				Type: "perp",
			},
		},
		Withdrawable: "50000",
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["type"] != "clearinghouseState" {
					t.Errorf("expected type clearinghouseState, got %v", req["type"])
				}
				if req["user"] != "0x123" {
					t.Errorf("expected user 0x123, got %v", req["user"])
				}
				if req["dex"] != "mainnet" {
					t.Errorf("expected dex mainnet, got %v", req["dex"])
				}
				*result.(*UserState) = *expectedState
				return nil
			},
		},
	}

	state, err := info.UserState(context.Background(), "0x123", "mainnet")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(state.AssetPositions) != 1 {
		t.Errorf("expected 1 position, got %d", len(state.AssetPositions))
	}
	if state.Withdrawable != "50000" {
		t.Errorf("expected withdrawable 50000, got %s", state.Withdrawable)
	}
}

func TestOpenOrdersSuccess(t *testing.T) {
	expectedOrders := []OpenOrder{
		{Coin: "BTC", LimitPx: "45000", Oid: 1, Side: "A", Sz: "1", Timestamp: 1234567890},
		{Coin: "ETH", LimitPx: "3000", Oid: 2, Side: "B", Sz: "10", Timestamp: 1234567891},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["type"] != "openOrders" {
					t.Errorf("expected type openOrders, got %v", req["type"])
				}
				*result.(*[]OpenOrder) = expectedOrders
				return nil
			},
		},
	}

	orders, err := info.OpenOrders(context.Background(), "0x123", "mainnet")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(orders) != len(expectedOrders) {
		t.Errorf("expected %d orders, got %d", len(expectedOrders), len(orders))
	}
}

func TestUserFillsSuccess(t *testing.T) {
	expectedFills := []Fill{
		{Coin: "BTC", Px: "45000", Sz: "1", Side: "A", Time: 1234567890, Oid: 1},
		{Coin: "ETH", Px: "3000", Sz: "10", Side: "B", Time: 1234567891, Oid: 2},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["type"] != "userFills" {
					t.Errorf("expected type userFills, got %v", req["type"])
				}
				*result.(*[]Fill) = expectedFills
				return nil
			},
		},
	}

	fills, err := info.UserFills(context.Background(), "0x123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(fills) != len(expectedFills) {
		t.Errorf("expected %d fills, got %d", len(expectedFills), len(fills))
	}
}

func TestUserFillsByTimeSuccess(t *testing.T) {
	expectedFills := []Fill{
		{Coin: "BTC", Px: "45000", Sz: "1", Side: "A", Time: 1234567890, Oid: 1},
	}
	endTime := int64(1234567900)

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["type"] != "userFillsByTime" {
					t.Errorf("expected type userFillsByTime, got %v", req["type"])
				}
				if req["startTime"] != int64(1234567880) {
					t.Errorf("expected startTime 1234567880, got %v", req["startTime"])
				}
				if req["endTime"] != endTime {
					t.Errorf("expected endTime %d, got %v", endTime, req["endTime"])
				}
				if req["aggregateByTime"] != true {
					t.Errorf("expected aggregateByTime true, got %v", req["aggregateByTime"])
				}
				*result.(*[]Fill) = expectedFills
				return nil
			},
		},
	}

	fills, err := info.UserFillsByTime(context.Background(), "0x123", 1234567880, &endTime, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(fills) != 1 {
		t.Errorf("expected 1 fill, got %d", len(fills))
	}
}

func TestFundingHistorySuccess(t *testing.T) {
	expectedHistory := []FundingRecord{
		{Coin: "BTC", FundingRate: "0.0001", Premium: "100", Time: 1234567890},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["type"] != "fundingHistory" {
					t.Errorf("expected type fundingHistory, got %v", req["type"])
				}
				if req["coin"] != "BTC" {
					t.Errorf("expected coin BTC, got %v", req["coin"])
				}
				*result.(*[]FundingRecord) = expectedHistory
				return nil
			},
		},
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	history, err := info.FundingHistory(context.Background(), "BTC", 1234567880, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(history) != 1 {
		t.Errorf("expected 1 record, got %d", len(history))
	}
}

func TestCandlesSnapshotSuccess(t *testing.T) {
	expectedCandles := []Candle{
		{T: 1234567890, O: "45000", C: "45500", H: "46000", L: "44500", V: "100", N: 50, S: "BTC", I: "1h"},
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["type"] != "candleSnapshot" {
					t.Errorf("expected type candleSnapshot, got %v", req["type"])
				}
				*result.(*[]Candle) = expectedCandles
				return nil
			},
		},
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	candles, err := info.CandlesSnapshot(context.Background(), "BTC", "1h", 1234567880, 1234567890)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(candles) != 1 {
		t.Errorf("expected 1 candle, got %d", len(candles))
	}
}

func TestUserFeesSuccess(t *testing.T) {
	expectedFees := map[string]any{
		"takerFee":  "0.0002",
		"makerFee":  "0.0001",
		"volume30d": "50000",
	}

	info := &Info{
		rest: &mockRestClient{
			postFunc: func(ctx context.Context, path string, body any, result any) error {
				req := body.(map[string]any)
				if req["type"] != "userFees" {
					t.Errorf("expected type userFees, got %v", req["type"])
				}
				*result.(*any) = expectedFees
				return nil
			},
		},
	}

	fees, err := info.UserFees(context.Background(), "0x123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if fees == nil {
		t.Fatal("expected fees to be non-nil")
	}
}

// ===== WebSocket Subscription Tests =====

func TestSubscribeAllMidsNoWS(t *testing.T) {
	info := &Info{}

	ch := make(chan ws.AllMidsMessage)
	_, err := info.SubscribeAllMids(context.Background(), ch)
	if err == nil {
		t.Fatal("expected error when ws is nil")
	}
	if err.Error() != "websocket not initialized" {
		t.Errorf("expected 'websocket not initialized', got %s", err.Error())
	}
}

func TestSubscribeAllMidsSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeAllMidsFunc: func(ctx context.Context, ch chan ws.AllMidsMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{ws: mockWS}

	ch := make(chan ws.AllMidsMessage)
	sub, err := info.SubscribeAllMids(context.Background(), ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

func TestSubscribeL2BookSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeL2BookFunc: func(ctx context.Context, coin string, ch chan ws.L2BookMessage) (ws.Subscription, error) {
			if coin != "BTC" {
				t.Errorf("expected coin BTC, got %s", coin)
			}
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	ch := make(chan ws.L2BookMessage)
	sub, err := info.SubscribeL2Book(context.Background(), "BTC", ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

func TestSubscribeTradesSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeTradesFunc: func(ctx context.Context, coin string, ch chan ws.TradesMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"ETH": "ETH"},
	}

	ch := make(chan ws.TradesMessage)
	sub, err := info.SubscribeTrades(context.Background(), "ETH", ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

func TestSubscribeCandleSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeCandleFunc: func(ctx context.Context, coin string, interval string, ch chan ws.CandleMessage) (ws.Subscription, error) {
			if interval != "1h" {
				t.Errorf("expected interval 1h, got %s", interval)
			}
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	ch := make(chan ws.CandleMessage)
	sub, err := info.SubscribeCandle(context.Background(), "BTC", "1h", ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

func TestSubscribeBboSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeBboFunc: func(ctx context.Context, coin string, ch chan ws.BboMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	ch := make(chan ws.BboMessage)
	sub, err := info.SubscribeBbo(context.Background(), "BTC", ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

func TestSubscribeActiveAssetCtxSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeActiveAssetCtxFunc: func(ctx context.Context, coin string, ch chan ws.ActiveAssetCtxMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{
		ws:         mockWS,
		nameToCoin: map[string]string{"BTC": "BTC"},
	}

	ch := make(chan ws.ActiveAssetCtxMessage)
	sub, err := info.SubscribeActiveAssetCtx(context.Background(), "BTC", ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

func TestSubscribeUserEventsSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeUserEventsFunc: func(ctx context.Context, user string, ch chan ws.UserEventsMessage) (ws.Subscription, error) {
			if user != "0x123" {
				t.Errorf("expected user 0x123, got %s", user)
			}
			return &mockSubscription{}, nil
		},
	}

	info := &Info{ws: mockWS}

	ch := make(chan ws.UserEventsMessage)
	sub, err := info.SubscribeUserEvents(context.Background(), "0x123", ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

func TestSubscribeUserFillsSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeUserFillsFunc: func(ctx context.Context, user string, ch chan ws.UserFillsMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{ws: mockWS}

	ch := make(chan ws.UserFillsMessage)
	sub, err := info.SubscribeUserFills(context.Background(), "0x456", ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

func TestSubscribeOrderUpdatesSuccess(t *testing.T) {
	mockWS := &mockWsClient{
		subscribeOrderUpdatesFunc: func(ctx context.Context, user string, ch chan ws.OrderUpdatesMessage) (ws.Subscription, error) {
			return &mockSubscription{}, nil
		},
	}

	info := &Info{ws: mockWS}

	ch := make(chan ws.OrderUpdatesMessage)
	sub, err := info.SubscribeOrderUpdates(context.Background(), "0x789", ch)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sub == nil {
		t.Fatal("expected subscription to be non-nil")
	}
}

// ===== Coin/Asset Management Tests =====

func TestSetCoinMapping(t *testing.T) {
	info := &Info{
		nameToCoin: make(map[string]string),
	}

	coins := []string{"BTC", "ETH", "SOL"}
	info.SetCoinMapping(coins)

	for _, coin := range coins {
		if val, ok := info.nameToCoin[coin]; !ok || val != coin {
			t.Errorf("expected mapping %s->%s, not found or incorrect", coin, coin)
		}
	}
}

func TestGetCoinFromNameFound(t *testing.T) {
	info := &Info{
		nameToCoin: map[string]string{
			"Bitcoin":  "BTC",
			"Ethereum": "ETH",
		},
	}

	if coin := info.getCoinFromName("Bitcoin"); coin != "BTC" {
		t.Errorf("expected BTC, got %s", coin)
	}

	if coin := info.getCoinFromName("Ethereum"); coin != "ETH" {
		t.Errorf("expected ETH, got %s", coin)
	}
}

func TestGetCoinFromNameNotFound(t *testing.T) {
	info := &Info{
		nameToCoin: map[string]string{},
	}

	// When not found, should return the name as-is
	if coin := info.getCoinFromName("BTC"); coin != "BTC" {
		t.Errorf("expected BTC, got %s", coin)
	}
}

func TestGetAssetFound(t *testing.T) {
	info := &Info{
		coinToAsset: map[string]int{"BTC": 0, "ETH": 1},
		nameToCoin:  map[string]string{"Bitcoin": "BTC"},
	}

	asset, ok := info.GetAsset("Bitcoin")
	if !ok {
		t.Fatal("expected asset to be found")
	}
	if asset != 0 {
		t.Errorf("expected asset 0, got %d", asset)
	}
}

func TestGetAssetNotFound(t *testing.T) {
	info := &Info{
		coinToAsset: map[string]int{},
		nameToCoin:  map[string]string{},
	}

	_, ok := info.GetAsset("UNKNOWN")
	if ok {
		t.Fatal("expected asset to not be found")
	}
}

func TestPullRealData(t *testing.T) {
	// Manual test
	t.Skip()
	info, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}

	mids, err := info.AllMids(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}

	if len(mids) == 0 {
		t.Fatal("expected non-zero length of mids")
	}

	midsChan := make(chan ws.AllMidsMessage)
	sub, err := info.SubscribeAllMids(context.Background(), midsChan)
	if err != nil {
		t.Fatal(err)
	}
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
			t.Fatalf("timeout waiting for messages after 10s, got %d messages", messageCount)
		}
	}
}

// ===== Helper Functions =====

func strPtr(s string) *string {
	return &s
}

// Mock subscription for testing
type mockSubscription struct{}

func (m *mockSubscription) Unsubscribe() {}

func (m *mockSubscription) Err() <-chan error {
	return make(chan error)
}
