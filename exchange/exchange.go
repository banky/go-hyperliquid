package exchange

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
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
// This is not supported on user-signed actions and must be None for those to
// work
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

	orderWires := make([]orderWire, len(orders))
	for i, order := range orders {
		assetId, ok := e.info.GetAsset(order.Coin)
		if !ok {
			return Response{}, fmt.Errorf("unknown coin: %s", order.Coin)
		}

		wire, err := order.toOrderWire(assetId)
		if err != nil {
			return Response{}, fmt.Errorf(
				"failed to convert order %d: %w",
				i,
				err,
			)
		}
		orderWires[i] = wire
	}

	action := ordersToAction(orderWires, builder)

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// ModifyOrder modifies a single order with Order ID
func (e *Exchange) ModifyOrder(
	ctx context.Context,
	oid int64,
	coin string,
	isBuy bool,
	sz float64,
	limitPx float64,
	orderType OrderType,
	opts ...ModifyOrderOption,
) (Response, error) {
	cfg := defaultModifyOrderConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	order := OrderRequest{
		Coin:       coin,
		IsBuy:      isBuy,
		Sz:         sz,
		LimitPx:    limitPx,
		OrderType:  orderType,
		ReduceOnly: cfg.reduceOnly,
	}

	modify := ModifyRequest{
		OID:   oid,
		Order: order,
	}

	return e.BulkModifyOrders(ctx, []ModifyRequest{modify})
}

// ModifyOrderWithCloid modifies a single order with Client Order ID
func (e *Exchange) ModifyOrderWithCloid(
	ctx context.Context,
	cloid common.Hash,
	coin string,
	isBuy bool,
	sz float64,
	limitPx float64,
	orderType OrderType,
	opts ...ModifyOrderOption,
) (Response, error) {
	cfg := defaultModifyOrderConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	order := OrderRequest{
		Coin:       coin,
		IsBuy:      isBuy,
		Sz:         sz,
		LimitPx:    limitPx,
		OrderType:  orderType,
		ReduceOnly: cfg.reduceOnly,
		CLOID:      &cloid,
	}

	modify := ModifyRequest{
		OID:   cloid,
		Order: order,
	}

	return e.BulkModifyOrders(ctx, []ModifyRequest{modify})
}

// BulkModifyOrders modifies multiple orders in a single transaction
func (e *Exchange) BulkModifyOrders(
	ctx context.Context,
	modifies []ModifyRequest,
) (Response, error) {
	if len(modifies) == 0 {
		return Response{}, fmt.Errorf("at least one modify request is required")
	}

	modifyWires := make([]modifyWire, len(modifies))
	for i, modify := range modifies {
		assetId, ok := e.info.GetAsset(modify.Order.Coin)
		if !ok {
			return Response{}, fmt.Errorf("unknown coin: %s", modify.Order.Coin)
		}

		wire, err := modify.Order.toOrderWire(assetId)
		if err != nil {
			return Response{}, fmt.Errorf(
				"failed to convert order %d: %w",
				i,
				err,
			)
		}

		// Handle OID conversion - if it's a Cloid (*common.Hash), use as-is,
		// otherwise convert int64
		var oid any
		if cloid, ok := modify.OID.(*common.Hash); ok {
			oid = cloid
		} else if intOid, ok := modify.OID.(int64); ok {
			oid = intOid
		} else {
			return Response{}, fmt.Errorf("invalid OID type for modify %d", i)
		}

		modifyWires[i] = modifyWire{
			OID:   oid,
			Order: wire,
		}
	}

	action := modifiesToAction(modifyWires)

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
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

// Cancel cancels a single order by order ID
func (e *Exchange) Cancel(
	ctx context.Context,
	oid int,
	coin string,
) (Response, error) {
	return e.BulkCancel(ctx, []CancelRequest{NewCancelRequest(coin, oid)})
}

// func (e *Exchange) CancelByCloid(
// 	ctx context.Context,
// 	cloid common.Hash,
// 	coin string,
// ) (Response, error) {

// }

// BulkCancel cancels multiple orders in a single transaction
func (e *Exchange) BulkCancel(
	ctx context.Context,
	cancels []CancelRequest,
) (Response, error) {
	if len(cancels) == 0 {
		return Response{}, fmt.Errorf("at least one cancel is required")
	}

	cancelWires := make([]cancelWire, len(cancels))
	for i, cancel := range cancels {
		// Get asset ID for this cancel's coin
		assetId, ok := e.info.GetAsset(cancel.Coin)
		if !ok {
			return Response{}, fmt.Errorf("unknown coin: %s", cancel.Coin)
		}

		cancelWires[i] = cancel.toCancelWire(assetId)
	}

	action := cancelsToAction(cancelWires)

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

func (e *Exchange) BulkCancelByCloid(
	ctx context.Context,
	cancels []CancelRequestByCloid,
) (Response, error) {
	if len(cancels) == 0 {
		return Response{}, fmt.Errorf("at least one cancel is required")
	}

	cancelWires := make([]cancelByCloidWire, len(cancels))
	for i, cancel := range cancels {
		// Get asset ID for this cancel's coin
		assetId, ok := e.info.GetAsset(cancel.Coin)
		if !ok {
			return Response{}, fmt.Errorf("unknown coin: %s", cancel.Coin)
		}

		cancelWires[i] = cancel.toCancelByCloidWire(assetId)
	}

	action := cancelsByCloidToAction(cancelWires)

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// Schedules a time to cancel all open orders. The time must be at least 5
// seconds. Once the duration elapses, all open orders will be canceled and a
// trigger count will be incremented. The max number of triggers per day is 10.
// This trigger count is reset at 00:00 UTC.
//
// if time is not nil, then set the cancel time in the future. If nil, then
// unsets any cancel time in the future.
func (e *Exchange) ScheduleCancel(
	ctx context.Context,
	opts ...ScheduleCancelOption,
) (Response, error) {
	cfg := defaultScheduleCancelConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	action := map[string]any{
		"type": "scheduleCancel",
	}

	if t, ok := cfg.time.Get(); ok {
		action["time"] = time.Now().Add(t).UnixMilli()
	}

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// UpdateLeverage updates the leverage for an asset
func (e *Exchange) UpdateLeverage(
	ctx context.Context,
	leverage int,
	name string,
	isCross bool,
) (Response, error) {
	// Get asset ID for the leverage update
	assetId, ok := e.info.GetAsset(name)
	if !ok {
		return Response{}, fmt.Errorf("unknown coin: %s", name)
	}

	action := map[string]any{
		"type":     "updateLeverage",
		"asset":    assetId,
		"isCross":  isCross,
		"leverage": leverage,
	}

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// UpdateIsolatedMargin updates the isolated margin for an asset
func (e *Exchange) UpdateIsolatedMargin(
	ctx context.Context,
	name string,
	amount float64,
) (Response, error) {
	intAmount, err := floatToUsdInt(amount)
	if err != nil {
		return Response{}, fmt.Errorf(
			"failed to convert amount to USD: %w",
			err,
		)
	}

	asset, ok := e.info.NameToAsset(name)
	if !ok {
		return Response{}, fmt.Errorf("unknown asset for name: %s", name)
	}

	action := map[string]any{
		"type":  "updateIsolatedMargin",
		"asset": int64(asset),
		"isBuy": true,
		"ntli":  int64(intAmount),
	}

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// SetReferrer sets the referrer code
func (e *Exchange) SetReferrer(
	ctx context.Context,
	code string,
) (Response, error) {
	action := map[string]any{
		"type": "setReferrer",
		"code": code,
	}

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

func (e *Exchange) CreateSubAccount(
	ctx context.Context,
	name string,
) (Response, error) {
	action := map[string]any{
		"type": "createSubAccount",
		"name": name,
	}

	timestamp := time.Now().UnixMilli()
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

func (e *Exchange) UsdClassTransfer(
	ctx context.Context,
	amount float64,
	toPerp bool,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	strAmount, err := floatToWire(amount)
	if err != nil {
		return Response{}, fmt.Errorf(
			"failed to convert amount to wire format: %w",
			err,
		)
	}

	action := map[string]any{
		"type":   "usdClassTransfer",
		"amount": strAmount,
		"toPerp": toPerp,
		"nonce":  timestamp,
	}

	sig, err := e.signUsdClassTransferAction(action)
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// SendAsset is used to transfer tokens between different perp
// DEXs, spot balance, users, and/or sub-accounts. Use "" to specify the default
// USDC perp DEX and "spot" to specify spot. Only the collateral token can be
// transferred to or from a perp DEX.
func (e *Exchange) SendAsset(
	ctx context.Context,
	destination string,
	sourceDex string,
	destinationDex string,
	token string,
	amount float64,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	strAmount, err := floatToWire(amount)
	if err != nil {
		return Response{}, fmt.Errorf(
			"failed to convert amount to wire format: %w",
			err,
		)
	}

	action := map[string]any{
		"type":           "sendAsset",
		"destination":    destination,
		"sourceDex":      sourceDex,
		"destinationDex": destinationDex,
		"token":          token,
		"amount":         strAmount,
		"nonce":          timestamp,
	}

	if v, ok := e.vaultAddress.Get(); ok {
		action["fromSubAccount"] = v.String()
	} else {
		action["fromSubAccount"] = ""
	}

	sig, err := e.signSendAssetAction(action)
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// SubAccountTransfer transfers assets between sub-accounts.
func (e *Exchange) SubAccountTransfer(
	ctx context.Context,
	subAccountUser common.Address,
	isDeposit bool,
	usd int,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	action := map[string]any{
		"type":           "subAccountTransfer",
		"subAccountUser": subAccountUser,
		"isDeposit":      isDeposit,
		"usd":            usd,
	}
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// SubAccountSpotTransfer transfers spot assets between sub-accounts.
func (e *Exchange) SubAccountSpotTransfer(
	ctx context.Context,
	subAccountUser common.Address,
	isDeposit bool,
	token string,
	amount float64,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	strAmount, err := floatToWire(amount)
	if err != nil {
		return Response{}, fmt.Errorf(
			"failed to convert amount to wire format: %w",
			err,
		)
	}

	action := map[string]any{
		"type":           "subAccountSpotTransfer",
		"subAccountUser": subAccountUser,
		"isDeposit":      isDeposit,
		"token":          token,
		"amount":         strAmount,
	}
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// VaultUsdTransfer transfers USD to or from a vault.
func (e *Exchange) VaultUsdTransfer(
	ctx context.Context,
	vaultAddress common.Address,
	isDeposit bool,
	usd int,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	action := map[string]any{
		"type":         "vaultTransfer",
		"vaultAddress": vaultAddress,
		"isDeposit":    isDeposit,
		"usd":          usd,
	}
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// UsdTransfer transfers USD to a destination address.
func (e *Exchange) UsdTransfer(
	ctx context.Context,
	amount float64,
	destination string,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	strAmount, err := floatToWire(amount)
	if err != nil {
		return Response{}, fmt.Errorf(
			"failed to convert amount to wire format: %w",
			err,
		)
	}

	action := map[string]any{
		"destination": destination,
		"amount":      strAmount,
		"time":        timestamp,
		"type":        "usdSend",
	}
	sig, err := e.signUsdTransferAction(action)
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// SpotTransfer transfers spot tokens to a destination address.
func (e *Exchange) SpotTransfer(
	ctx context.Context,
	amount float64,
	destination string,
	token string,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	strAmount, err := floatToWire(amount)
	if err != nil {
		return Response{}, fmt.Errorf(
			"failed to convert amount to wire format: %w",
			err,
		)
	}

	action := map[string]any{
		"destination": destination,
		"amount":      strAmount,
		"token":       token,
		"time":        timestamp,
		"type":        "spotSend",
	}
	sig, err := e.signSpotTransferAction(action)
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// TokenDelegate delegates tokens to a validator.
func (e *Exchange) TokenDelegate(
	ctx context.Context,
	validator string,
	wei int,
	isUndelegate bool,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	action := map[string]any{
		"validator":    validator,
		"wei":          wei,
		"isUndelegate": isUndelegate,
		"nonce":        timestamp,
		"type":         "tokenDelegate",
	}
	sig, err := e.signTokenDelegateAction(action)
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// WithdrawFromBridge withdraws tokens from the bridge.
func (e *Exchange) WithdrawFromBridge(
	ctx context.Context,
	amount float64,
	destination string,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	strAmount, err := floatToWire(amount)
	if err != nil {
		return Response{}, fmt.Errorf(
			"failed to convert amount to wire format: %w",
			err,
		)
	}

	action := map[string]any{
		"destination": destination,
		"amount":      strAmount,
		"time":        timestamp,
		"type":        "withdraw3",
	}
	sig, err := e.signWithdrawFromBridgeAction(action)
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// ApproveAgent approves an agent and returns the response and the agent's
// private key.
func (e *Exchange) ApproveAgent(
	ctx context.Context,
	opts ...ApproveAgentOption,
) (Response, *ecdsa.PrivateKey, error) {
	cfg := defaultApproveAgentConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	// Generate random agent private key
	agentPrivateKey, err := crypto.GenerateKey()
	if err != nil {
		return Response{}, nil, fmt.Errorf(
			"failed to generate agent key: %w",
			err,
		)
	}

	// Get agent address
	agentAddress := crypto.PubkeyToAddress(agentPrivateKey.PublicKey)

	timestamp := time.Now().UnixMilli()
	action := map[string]any{
		"type":         "approveAgent",
		"agentAddress": agentAddress,
		"nonce":        timestamp,
	}

	// Add agent name if provided
	if a, ok := cfg.name.Get(); ok {
		action["agentName"] = a
	} else {
		action["agentName"] = ""
	}

	sig, err := e.signAgentAction(action)
	if err != nil {
		return Response{}, nil, fmt.Errorf("failed to sign action: %w", err)
	}

	// Remove agentName from action if name was not provided
	if cfg.name.IsNone() {
		delete(action, "agentName")
	}

	result, err := e.post(ctx, action, timestamp, sig)
	if err != nil {
		return Response{}, nil, err
	}

	return result, agentPrivateKey, nil
}

// ApproveBuilderFee approves a maximum fee rate for a builder.
// maxFeeRate is a percentage, so maxFeeRate of 0.01 = 1%
func (e *Exchange) ApproveBuilderFee(
	ctx context.Context,
	builder common.Address,
	maxFeeRate float64,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	maxFeeRateStr, err := floatToWire(maxFeeRate * 100)
	if err != nil {
		return Response{}, fmt.Errorf(
			"failed to convert maxFeeRate to wire format: %w",
			err,
		)
	}
	action := map[string]any{
		"maxFeeRate": maxFeeRateStr + "%",
		"builder":    builder,
		"nonce":      timestamp,
		"type":       "approveBuilderFee",
	}
	sig, err := e.signApproveBuilderFeeAction(action)
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// ConvertToMultiSigUser converts the user account to a multi-sig account
func (e *Exchange) ConvertToMultiSigUser(
	ctx context.Context,
	authorizedUsers []common.Address,
	threshold int,
) (Response, error) {
	timestamp := time.Now().UnixMilli()

	// Sort authorized users
	sortedUsers := make([]common.Address, len(authorizedUsers))
	copy(sortedUsers, authorizedUsers)
	slices.SortFunc(
		authorizedUsers,
		func(a, z common.Address) int {
			return a.Cmp(z)
		},
	)

	// Create signers JSON
	signers := map[string]any{
		"authorizedUsers": sortedUsers,
		"threshold":       int64(threshold),
	}
	signersJSON, err := json.Marshal(signers)
	if err != nil {
		return Response{}, fmt.Errorf("failed to marshal signers: %w", err)
	}

	action := map[string]any{
		"type":    "convertToMultiSigUser",
		"signers": string(signersJSON),
		"nonce":   timestamp,
	}
	sig, err := e.signConvertToMultiSigUserAction(action)
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// SpotDeployRegisterToken registers a token for spot deployment
func (e *Exchange) SpotDeployRegisterToken(
	ctx context.Context,
	tokenName string,
	szDecimals int,
	weiDecimals int,
	maxGas int,
	fullName string,
) (Response, error) {
	timestamp := time.Now().UnixMilli()
	action := map[string]any{
		"type": "spotDeploy",
		"registerToken2": map[string]any{
			"spec": map[string]any{
				"name":        tokenName,
				"szDecimals":  szDecimals,
				"weiDecimals": weiDecimals,
			},
			"maxGas":   maxGas,
			"fullName": fullName,
		},
	}
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

// SpotDeployUserGenesis performs user genesis for spot deployment
func (e *Exchange) SpotDeployUserGenesis(
	ctx context.Context,
	token int,
	userAndWei []UserWeiPair,
	existingTokenAndWei []TokenWeiPair,
) (Response, error) {
	timestamp := time.Now().UnixMilli()

	// Convert userAndWei to lowercase addresses and string wei
	userAndWeiAction := make([][]string, len(userAndWei))
	for i, pair := range userAndWei {
		userAndWeiAction[i] = []string{
			strings.ToLower(pair.User.String()),
			pair.Wei.String(),
		}
	}

	// Convert existingTokenAndWei to action format
	existingTokenAndWeiAction := make([][]any, len(existingTokenAndWei))
	for i, pair := range existingTokenAndWei {
		existingTokenAndWeiAction[i] = []any{
			pair.Token,
			pair.Wei.String(),
		}
	}

	action := map[string]any{
		"type": "spotDeploy",
		"userGenesis": map[string]any{
			"token":               token,
			"userAndWei":          userAndWeiAction,
			"existingTokenAndWei": existingTokenAndWeiAction,
		},
	}
	sig, err := e.signL1Action(action, uint64(timestamp))
	if err != nil {
		return Response{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return e.post(ctx, action, timestamp, sig)
}

func (e *Exchange) post(
	ctx context.Context,
	action map[string]any,
	timestamp int64,
	sig signature,
) (Response, error) {
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
		return Response{}, fmt.Errorf(
			"failed to post to /exchange. Type: %v: %w",
			actionType,
			err,
		)
	}

	return result, nil
}

func (e *Exchange) getSlippagePrice(
	ctx context.Context,
	coin string,
	isBuy bool,
	slippage float64,
	pxOverride mo.Option[float64],
) (float64, error) {
	var px float64
	c, ok := e.info.NameToCoin(coin)
	if !ok {
		return 0, fmt.Errorf("coin not found: %s", coin)
	}
	coin = c

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
	// Python: round(px_5sig, (6 if not is_spot else 8) -
	// asset_to_sz_decimals[asset])
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
