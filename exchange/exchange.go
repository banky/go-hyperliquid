package exchange

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"time"

	"github.com/banky/go-hyperliquid/constants"
	"github.com/banky/go-hyperliquid/info"
	"github.com/banky/go-hyperliquid/rest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/samber/mo"
)

// Config for initializing the Exchange client
type Config struct {
	BaseURL        string
	Timeout        time.Duration
	SkipWS         bool
	PrivateKey     *ecdsa.PrivateKey
	AccountAddress common.Address
	VaultAddress   common.Address
	Meta           *info.Meta
	SpotMeta       *info.SpotMeta
	PerpDexes      []string
}

// Exchange provides access to trading operations via REST API
type Exchange struct {
	rest           rest.ClientInterface
	info           *info.Info
	privateKey     *ecdsa.PrivateKey
	vaultAddress   mo.Option[common.Address]
	accountAddress mo.Option[common.Address]
	expiresAfter   mo.Option[time.Duration]
}

// New creates a new Exchange client
func New(cfg Config) (*Exchange, error) {
	if cfg.PrivateKey == nil {
		return nil, fmt.Errorf("private key is required")
	}

	// Create REST client
	restClient := rest.New(rest.Config{
		BaseUrl: cfg.BaseURL,
		Timeout: cfg.Timeout,
	})

	// Create Info client
	infoClient, err := info.New(info.Config{
		BaseURL:  cfg.BaseURL,
		Timeout:  cfg.Timeout,
		SkipWS:   true,
		Meta:     cfg.Meta,
		SpotMeta: cfg.SpotMeta,
		PerpDexs: cfg.PerpDexes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create info client: %w", err)
	}

	var vaultAddress mo.Option[common.Address]
	if cfg.VaultAddress != constants.ZERO_ADDRESS {
		vaultAddress = mo.Some(cfg.VaultAddress)
	}

	var accountAddress mo.Option[common.Address]
	if cfg.AccountAddress != constants.ZERO_ADDRESS {
		accountAddress = mo.Some(cfg.AccountAddress)
	}

	return &Exchange{
		rest:           restClient,
		info:           infoClient,
		privateKey:     cfg.PrivateKey,
		accountAddress: accountAddress,
		vaultAddress:   vaultAddress,
		expiresAfter:   mo.None[time.Duration](),
	}, nil
}

// Close cleans up the Exchange instance
func (e *Exchange) Close() {
	if e.info != nil {
		e.info.Close()
	}
}

// SetExpiresAfter sets the expiration time for actions (in milliseconds)
// This is not supported on user-signed actions and must be None for those to work
func (e *Exchange) SetExpiresAfter(expiresAfter time.Duration) {
	e.expiresAfter = mo.Some(expiresAfter)
}

// ClearExpiresAfter clears the expiration time
func (e *Exchange) ClearExpiresAfter() {
	e.expiresAfter = mo.None[time.Duration]()
}

// DEFAULT_SLIPPAGE is the default max slippage for market orders (5%)
const DEFAULT_SLIPPAGE = 0.05

// Order creates a single order
func (e *Exchange) Order(
	ctx context.Context,
	name string,
	isBuy bool,
	sz float64,
	limitPx float64,
	orderType OrderType,
	opts ...OrderOption,
) (any, error) {
	cfg := defaultOrderConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	order := OrderRequest{
		Coin:       name,
		IsBuy:      isBuy,
		Sz:         sz,
		LimitPx:    limitPx,
		OrderType:  orderType,
		ReduceOnly: cfg.reduceOnly,
		CLOID:      cfg.getCLOID(),
	}

	return e.BulkOrders(ctx, []OrderRequest{order}, cfg.getBuilderInfo())
}

// BulkOrders creates multiple orders in a single transaction
func (e *Exchange) BulkOrders(
	ctx context.Context,
	orders []OrderRequest,
	builder *BuilderInfo,
) (Response, error) {
	if len(orders) == 0 {
		return Response{}, fmt.Errorf("at least one order is required")
	}

	orderWires := make([]OrderWire, len(orders))
	for i, order := range orders {
		assetId, ok := e.info.GetAsset(order.Coin)
		if !ok {
			return Response{}, fmt.Errorf("unknown coin: %s", order.Coin)
		}

		wire, err := order.toOrderWire(assetId)
		if err != nil {
			return Response{}, fmt.Errorf("failed to convert order %d: %w", i, err)
		}
		orderWires[i] = wire
	}

	action := ordersToAction(orderWires, builder)

	return e.post(ctx, action)
}

// Cancel cancels a single order by order ID
func (e *Exchange) Cancel(
	ctx context.Context,
	name string,
	oid int64,
) (any, error) {
	return e.BulkCancel(ctx, []CancelRequest{{Coin: name, OID: oid}})
}

// BulkCancel cancels multiple orders in a single transaction
func (e *Exchange) BulkCancel(
	ctx context.Context,
	cancels []CancelRequest,
) (Response, error) {
	if len(cancels) == 0 {
		return Response{}, fmt.Errorf("at least one cancel is required")
	}

	cancelWires := make([]CancelWire, len(cancels))
	for i, cancel := range cancels {
		// Get asset ID for this cancel's coin
		assetId, ok := e.info.GetAsset(cancel.Coin)
		if !ok {
			return Response{}, fmt.Errorf("unknown coin: %s", cancel.Coin)
		}

		cancelWires[i] = cancel.toCancelWire(assetId)
	}

	action := cancelsToAction(cancelWires)

	return e.post(ctx, action)
}

// MarketOpen opens a market position
func (e *Exchange) MarketOpen(
	ctx context.Context,
	coin string,
	isBuy bool,
	sz float64,
	opts ...MarketOrderOption,
) (any, error) {
	cfg := defaultMarketOrderConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	px, err := e.getSlippagePrice(
		ctx,
		coin,
		isBuy,
		cfg.slippage,
		cfg.px,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get slippage price: %w", err)
	}

	// Market order is an aggressive limit order with IoC tif
	return e.Order(
		ctx,
		coin,
		isBuy,
		sz,
		px,
		OrderType{
			Limit: &LimitOrder{Tif: "Ioc"},
		},
		WithOrderReduceOnly(false),
		withOrderCLOID(cfg.cloid),
	)
}

// MarketClose closes a market position
func (e *Exchange) MarketClose(
	ctx context.Context,
	coin string,
	opts ...MarketCloseOption,
) (any, error) {
	cfg := defaultMarketCloseConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	publicKey := e.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error getting public key from private key")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	if a, ok := e.accountAddress.Get(); ok {
		address = a
	}
	if v, ok := e.vaultAddress.Get(); ok {
		address = v
	}

	// Get user state to find the position
	dex := getDex(coin)
	userState, err := e.info.UserState(ctx, address, dex)
	if err != nil {
		return nil, fmt.Errorf("failed to get user state: %w", err)
	}

	// Find the position for this coin
	var position *info.Position
	var positionSize float64
	if userState.AssetPositions != nil {
		for _, assetPos := range userState.AssetPositions {
			if assetPos.Position.Coin == coin {
				position = &assetPos.Position
				sz, err := stringToFloat(assetPos.Position.Szi)
				if err != nil {
					return nil, fmt.Errorf("invalid position size: %w", err)
				}
				positionSize = sz
				break
			}
		}
	}

	if position == nil {
		return nil, fmt.Errorf("no position found for coin: %s", coin)
	}

	// Determine size to close
	var closeSz float64
	if sz, ok := cfg.sz.Get(); ok {
		closeSz = sz
	} else {
		// Close entire position
		closeSz = math.Abs(positionSize)
	}

	// Determine buy/sell direction (opposite of current position)
	isBuy := positionSize < 0

	px, err := e.getSlippagePrice(
		ctx,
		coin,
		isBuy,
		cfg.slippage,
		cfg.px,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get slippage price: %w", err)
	}

	// Create market close order
	return e.Order(
		ctx,
		coin,
		isBuy,
		closeSz,
		px,
		OrderType{
			Limit: &LimitOrder{Tif: "Ioc"},
		},
		WithOrderReduceOnly(true),
		withOrderCLOID(cfg.cloid),
	)
}

// UpdateLeverage updates the leverage for an asset
func (e *Exchange) UpdateLeverage(
	ctx context.Context,
	leverage int,
	name string,
	isCross bool,
) (any, error) {
	// Get asset ID for the leverage update
	assetId, ok := e.info.GetAsset(name)
	if !ok {
		return nil, fmt.Errorf("unknown coin: %s", name)
	}

	action := map[string]any{
		"type":     "updateLeverage",
		"asset":    assetId,
		"isCross":  isCross,
		"leverage": leverage,
	}

	return e.post(
		ctx,
		action,
	)
}

func (e *Exchange) post(
	ctx context.Context,
	action map[string]any,
) (Response, error) {
	timestamp := time.Now().UnixMilli()

	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	payload := map[string]any{
		"action":    action,
		"signature": sig,
		"nonce":     timestamp,
	}

	actionType := action["type"]
	if actionType == "usdClassTransfer" || actionType == "sendAsset" {
		payload["vaultAddress"] = nil
	} else if v, ok := e.vaultAddress.Get(); ok {
		payload["vaultAddress"] = v
	} else {
		payload["vaultAddress"] = nil
	}

	if e, ok := e.expiresAfter.Get(); ok {
		payload["expiresAfter"] = e
	} else {
		payload["expiresAfter"] = nil
	}

	var result Response
	if err := e.rest.Post(ctx, "/exchange", payload, &result); err != nil {
		return Response{}, fmt.Errorf("failed to post to /exchange. Type: %v: %w", actionType, err)
	}

	return result, err
}

func (e *Exchange) getSlippagePrice(
	ctx context.Context,
	coin string,
	isBuy bool,
	slippage float64,
	pxOverride mo.Option[float64],
) (float64, error) {
	var px float64

	// Use override price if present, otherwise fetch midprice
	if override, ok := pxOverride.Get(); ok {
		px = override
	} else {
		dex := getDex(coin)

		mids, err := e.info.AllMids(ctx, dex)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch mid prices: %w", err)
		}

		midPriceStr, ok := mids[coin]
		if !ok {
			return 0, fmt.Errorf("mid price not found for coin: %s", coin)
		}

		midPrice, err := stringToFloat(midPriceStr)
		if err != nil {
			return 0, fmt.Errorf("invalid mid price for coin %s: %w", coin, err)
		}

		px = midPrice
	}

	// 2. Map coin -> asset
	asset, ok := e.info.CoinToAsset(coin)
	if !ok {
		return 0, fmt.Errorf("asset not found for coin: %s", coin)
	}

	// Spot assets start at 10000 (same logic as Python: asset >= 10_000)
	isSpot := asset >= 10_000

	// Apply slippage in the right direction
	if isBuy {
		px = px * (1 + slippage)
	} else {
		px = px * (1 - slippage)
	}

	// 4. Round to 5 significant figures (Python: f"{px:.5g}")
	px = roundToSigfig(px, 5)

	// 5. Final decimal rounding:
	//    Python: round(px_5sig, (6 if not is_spot else 8) - asset_to_sz_decimals[asset])
	baseDecimals := 6
	if isSpot {
		baseDecimals = 8
	}

	szDecimals, ok := e.info.AssetToSzDecimals(asset)
	if !ok {
		return 0, fmt.Errorf("asset sz decimals not found for asset: %d", asset)
	}

	decimals := baseDecimals - szDecimals
	px = roundToDecimals(px, decimals)

	return px, nil
}
