package info

import (
	"context"
	"fmt"
	"sync"

	"github.com/banky/go-hyperliquid/rest"
	"github.com/banky/go-hyperliquid/ws"
)

// Info provides access to market data and user account information via REST and WebSocket APIs
type Info struct {
	rest rest.ClientInterface
	ws   ws.ClientInterface

	mu                sync.RWMutex
	coinToAsset       map[string]int
	nameToCoin        map[string]string
	assetToSzDecimals map[int]int
}

// Config for initializing the Info client
type Config struct {
	BaseURL string
	Timeout uint
	SkipWS  bool
}

// New creates a new Info client
func New(cfg Config) (*Info, error) {
	// Create REST client
	client := rest.New(rest.Config{
		BaseUrl: cfg.BaseURL,
		Timeout: cfg.Timeout,
	})

	// Create WebSocket manager if not skipped
	var wsManager *ws.Client
	if !cfg.SkipWS {
		wsManager = ws.New(cfg.BaseURL)
		wsManager.Start(context.Background())
	}

	info := &Info{
		rest:              client,
		ws:                wsManager,
		coinToAsset:       make(map[string]int),
		nameToCoin:        make(map[string]string),
		assetToSzDecimals: make(map[int]int),
	}

	return info, nil
}

// Start initializes the WebSocket connection
func (i *Info) Start(ctx context.Context) error {
	if i.ws != nil {
		return i.ws.Start(ctx)
	}
	return nil
}

// Stop closes the WebSocket connection
func (i *Info) Stop() {
	if i.ws != nil {
		i.ws.Stop()
	}
}

// ===== Market Data Queries =====

// AllMids retrieves mid-prices for all coins, with fallback to last trade price if book is empty.
func (i *Info) AllMids(ctx context.Context, dex string) (map[string]string, error) {
	var result map[string]string
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "allMids",
			"dex":  dex,
		},
		&result,
	)

	return result, err
}

// L2Snapshot retrieves up to 20 levels of the order book for a coin.
func (i *Info) L2Snapshot(ctx context.Context, name string) (*L2BookSnapshot, error) {
	coin := i.getCoinFromName(name)
	if coin == "" {
		return nil, fmt.Errorf("unknown coin name: %s", name)
	}

	var result L2BookSnapshot
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "l2Book",
			"coin": coin,
		},
		&result,
	)

	return &result, err
}

// Meta retrieves exchange metadata for perpetuals.
func (i *Info) Meta(ctx context.Context, dex string) (*Meta, error) {
	var result Meta
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "meta",
			"dex":  dex,
		},
		&result,
	)

	return &result, err
}

// SpotMeta retrieves exchange metadata for spot trading.
func (i *Info) SpotMeta(ctx context.Context) (*SpotMeta, error) {
	var result SpotMeta
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "spotMeta",
		},
		&result,
	)

	return &result, err
}

// ===== User Account Queries =====

// UserState retrieves account portfolio and position data.
func (i *Info) UserState(ctx context.Context, address string, dex string) (*UserState, error) {
	var result UserState
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "clearinghouseState",
			"user": address,
			"dex":  dex,
		},
		&result,
	)

	return &result, err
}

// SpotUserState retrieves account portfolio and position data for spot trading.
func (i *Info) SpotUserState(ctx context.Context, address string) (any, error) {
	var result any
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "spotClearinghouseState",
			"user": address,
		},
		&result,
	)

	return result, err
}

// OpenOrders retrieves a user's active orders.
func (i *Info) OpenOrders(ctx context.Context, address string, dex string) ([]OpenOrder, error) {
	var result []OpenOrder
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "openOrders",
			"user": address,
			"dex":  dex,
		},
		&result,
	)

	return result, err
}

// UserFills retrieves a user's fills/executed trades.
func (i *Info) UserFills(ctx context.Context, address string) ([]Fill, error) {
	var result []Fill
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "userFills",
			"user": address,
		},
		&result,
	)

	return result, err
}

// UserFillsByTime retrieves a user's fills within a time range.
func (i *Info) UserFillsByTime(
	ctx context.Context,
	address string,
	startTime int64,
	endTime *int64,
	aggregateByTime bool,
) ([]Fill, error) {
	req := map[string]any{
		"type":            "userFillsByTime",
		"user":            address,
		"startTime":       startTime,
		"aggregateByTime": aggregateByTime,
	}
	if endTime != nil {
		req["endTime"] = *endTime
	}

	var result []Fill
	err := i.rest.Post(
		ctx,
		"/info",
		req,
		&result,
	)

	return result, err
}

// FundingHistory retrieves funding history for a coin.
func (i *Info) FundingHistory(
	ctx context.Context,
	name string,
	startTime int64,
	endTime *int64,
) ([]FundingRecord, error) {
	coin := i.getCoinFromName(name)
	if coin == "" {
		return nil, fmt.Errorf("unknown coin name: %s", name)
	}

	req := map[string]any{
		"type":      "fundingHistory",
		"coin":      coin,
		"startTime": startTime,
	}
	if endTime != nil {
		req["endTime"] = *endTime
	}

	var result []FundingRecord
	err := i.rest.Post(
		ctx,
		"/info",
		req,
		&result,
	)

	return result, err
}

// UserFundingHistory retrieves a user's funding history.
func (i *Info) UserFundingHistory(
	ctx context.Context,
	user string,
	startTime int64,
	endTime *int64,
) (any, error) {
	req := map[string]any{
		"type":      "userFunding",
		"user":      user,
		"startTime": startTime,
	}
	if endTime != nil {
		req["endTime"] = *endTime
	}

	var result any
	err := i.rest.Post(
		ctx,
		"/info",
		req,
		&result,
	)

	return result, err
}

// CandlesSnapshot retrieves candlestick/OHLC data for a coin and interval.
func (i *Info) CandlesSnapshot(
	ctx context.Context,
	name string,
	interval string,
	startTime int64,
	endTime int64,
) ([]Candle, error) {
	coin := i.getCoinFromName(name)
	if coin == "" {
		return nil, fmt.Errorf("unknown coin name: %s", name)
	}

	req := map[string]any{
		"coin":      coin,
		"interval":  interval,
		"startTime": startTime,
		"endTime":   endTime,
	}

	var result []Candle
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "candleSnapshot",
			"req":  req,
		},
		&result,
	)

	return result, err
}

// UserFees retrieves a user's fee information and trading volume.
func (i *Info) UserFees(ctx context.Context, address string) (any, error) {
	var result any
	err := i.rest.Post(
		ctx,
		"/info",
		map[string]any{
			"type": "userFees",
			"user": address,
		},
		&result,
	)

	return result, err
}

// ===== WebSocket Subscriptions =====

// SubscribeAllMids subscribes to all mid-prices
func (i *Info) SubscribeAllMids(ctx context.Context, ch chan<- ws.AllMidsMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	return i.ws.SubscribeAllMids(ctx, ch)
}

// SubscribeL2Book subscribes to level 2 order book for a coin
func (i *Info) SubscribeL2Book(ctx context.Context, name string, ch chan<- ws.L2BookMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	coin := i.getCoinFromName(name)
	if coin == "" {
		return nil, fmt.Errorf("unknown coin name: %s", name)
	}
	return i.ws.SubscribeL2Book(ctx, coin, ch)
}

// SubscribeTrades subscribes to trades for a coin
func (i *Info) SubscribeTrades(ctx context.Context, name string, ch chan<- ws.TradesMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	coin := i.getCoinFromName(name)
	if coin == "" {
		return nil, fmt.Errorf("unknown coin name: %s", name)
	}
	return i.ws.SubscribeTrades(ctx, coin, ch)
}

// SubscribeCandle subscribes to candle data
func (i *Info) SubscribeCandle(ctx context.Context, name string, interval string, ch chan<- ws.CandleMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	coin := i.getCoinFromName(name)
	if coin == "" {
		return nil, fmt.Errorf("unknown coin name: %s", name)
	}
	return i.ws.SubscribeCandle(ctx, coin, interval, ch)
}

// SubscribeBbo subscribes to best bid/offer data
func (i *Info) SubscribeBbo(ctx context.Context, name string, ch chan<- ws.BboMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	coin := i.getCoinFromName(name)
	if coin == "" {
		return nil, fmt.Errorf("unknown coin name: %s", name)
	}
	return i.ws.SubscribeBbo(ctx, coin, ch)
}

// SubscribeActiveAssetCtx subscribes to active asset context
func (i *Info) SubscribeActiveAssetCtx(ctx context.Context, name string, ch chan<- ws.ActiveAssetCtxMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	coin := i.getCoinFromName(name)
	if coin == "" {
		return nil, fmt.Errorf("unknown coin name: %s", name)
	}
	return i.ws.SubscribeActiveAssetCtx(ctx, coin, ch)
}

// SubscribeUserEvents subscribes to user events
func (i *Info) SubscribeUserEvents(ctx context.Context, user string, ch chan<- ws.UserEventsMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	return i.ws.SubscribeUserEvents(ctx, user, ch)
}

// SubscribeUserFills subscribes to user fills
func (i *Info) SubscribeUserFills(ctx context.Context, user string, ch chan<- ws.UserFillsMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	return i.ws.SubscribeUserFills(ctx, user, ch)
}

// SubscribeOrderUpdates subscribes to order updates
func (i *Info) SubscribeOrderUpdates(ctx context.Context, user string, ch chan<- ws.OrderUpdatesMessage) (ws.Subscription, error) {
	if i.ws == nil {
		return nil, fmt.Errorf("websocket not initialized")
	}
	return i.ws.SubscribeOrderUpdates(ctx, user, ch)
}

// ===== Coin/Asset Management =====

// getCoinFromName retrieves the actual coin name from a user-friendly name
// Returns the coin as-is if it matches an entry in the mapping
func (i *Info) getCoinFromName(name string) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if coin, ok := i.nameToCoin[name]; ok {
		return coin
	}
	return name
}

// SetCoinMapping sets up the mapping between user-friendly names and actual coin names
// This should be called after retrieving metadata
func (i *Info) SetCoinMapping(coins []string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	for _, coin := range coins {
		i.nameToCoin[coin] = coin
	}
}

// GetAsset retrieves the asset ID for a given coin/name
func (i *Info) GetAsset(name string) (int, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	asset, ok := i.coinToAsset[i.nameToCoin[name]]
	return asset, ok
}
