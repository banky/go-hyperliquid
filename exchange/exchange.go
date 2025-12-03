package exchange

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"slices"
	"sync/atomic"
	"time"

	"github.com/banky/go-hyperliquid/constants"
	"github.com/banky/go-hyperliquid/info"
	"github.com/banky/go-hyperliquid/internal/utils"
	"github.com/banky/go-hyperliquid/rest"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/samber/mo"
)

// Config for initializing the Exchange client
type Config struct {
	BaseURL        string
	Timeout        time.Duration
	SkipInfo       bool
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
	prevNonce      *atomic.Int64
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

	var infoClient *info.Info
	if !cfg.SkipInfo {
		// Create Info client
		i, err := info.New(info.Config{
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

		infoClient = i
	}

	var vaultAddress mo.Option[common.Address]
	if cfg.VaultAddress != constants.ZERO_ADDRESS {
		vaultAddress = mo.Some(cfg.VaultAddress)
	}

	var accountAddress mo.Option[common.Address]
	if cfg.AccountAddress != constants.ZERO_ADDRESS {
		accountAddress = mo.Some(cfg.AccountAddress)
	}

	prevNonce := new(atomic.Int64)
	prevNonce.Store(time.Now().UnixMilli())

	return &Exchange{
		rest:           restClient,
		info:           infoClient,
		privateKey:     cfg.PrivateKey,
		accountAddress: accountAddress,
		vaultAddress:   vaultAddress,
		expiresAfter:   mo.None[time.Duration](),
		prevNonce:      prevNonce,
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
	request orderRequest,
	opts ...orderOption,
) (OrderResponse, error) {
	responses, err := e.BulkOrders(ctx, []orderRequest{request}, opts...)
	if err != nil {
		return OrderResponse{}, err
	}
	if len(responses) == 0 {
		return OrderResponse{}, fmt.Errorf("empty response from order")
	}
	return OrderResponse(responses[0]), nil
}

// BulkOrders creates multiple orders in a single transaction
func (e *Exchange) BulkOrders(
	ctx context.Context,
	requests []orderRequest,
	opts ...orderOption,
) (BulkOrdersResponse, error) {
	cfg := orderConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return e.bulkOrders(ctx, requests, cfg.builder, cfg.grouping)
}

func (e *Exchange) bulkOrders(
	ctx context.Context,
	requests []orderRequest,
	builder mo.Option[BuilderInfo],
	grouping mo.Option[OrderGrouping],
) (BulkOrdersResponse, error) {
	if len(requests) == 0 {
		return BulkOrdersResponse{}, fmt.Errorf(
			"at least one order is required",
		)
	}

	orderWires := make([]orderWire, len(requests))
	for i, order := range requests {
		assetId, ok := e.info.GetAsset(order.coin)
		if !ok {
			return BulkOrdersResponse{}, fmt.Errorf(
				"unknown coin: %s",
				order.coin,
			)
		}

		wire, err := order.toOrderWire(assetId)
		if err != nil {
			return BulkOrdersResponse{}, fmt.Errorf(
				"failed to convert order %d: %w",
				i,
				err,
			)
		}
		orderWires[i] = wire
	}

	action := ordersToAction(orderWires, builder, grouping)

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action,
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)
	if err != nil {
		return BulkOrdersResponse{}, fmt.Errorf(
			"failed to sign action: %w",
			err,
		)
	}

	return post[BulkOrdersResponse](ctx, e, action, timestamp, sig)
}

// ModifyOrder modifies a single order with Order ID
func (e *Exchange) ModifyOrder(
	ctx context.Context,
	request modifyRequest,
) (OrderResponse, error) {
	return e.modifySingleOrder(ctx, request)
}

// ModifyOrderWithCloid modifies a single order with Client Order ID
func (e *Exchange) ModifyOrderWithCloid(
	ctx context.Context,
	request modifyRequest,
) (OrderResponse, error) {
	return e.modifySingleOrder(ctx, request)
}

func (e *Exchange) modifySingleOrder(
	ctx context.Context,
	request modifyRequest,
) (OrderResponse, error) {
	responses, err := e.BulkModifyOrders(ctx, []modifyRequest{request})
	if err != nil {
		return OrderResponse{}, err
	}
	if len(responses) == 0 {
		return OrderResponse{}, fmt.Errorf("empty response from modify order")
	}
	return OrderResponse(responses[0]), nil
}

// BulkModifyOrders modifies multiple orders in a single transaction
func (e *Exchange) BulkModifyOrders(
	ctx context.Context,
	requests []modifyRequest,
) (BulkOrdersResponse, error) {
	if len(requests) == 0 {
		return BulkOrdersResponse{}, fmt.Errorf(
			"at least one modify request is required",
		)
	}

	modifyWires := make([]modifyWire, len(requests))
	for i, modify := range requests {
		assetId, ok := e.info.GetAsset(modify.Order.coin)
		if !ok {
			return BulkOrdersResponse{}, fmt.Errorf(
				"unknown coin: %s",
				modify.Order.coin,
			)
		}

		wire, err := modify.Order.toOrderWire(assetId)
		if err != nil {
			return BulkOrdersResponse{}, fmt.Errorf(
				"failed to convert order %d: %w",
				i,
				err,
			)
		}

		var oid any
		if o, ok := modify.Oid.Get(); ok {
			oid = o
		} else if c, ok := modify.Cloid.Get(); ok {
			oid = c
		} else {
			return BulkOrdersResponse{}, fmt.Errorf("invalid OID type for modify %d", i)
		}

		modifyWires[i] = modifyWire{
			Oid:   oid,
			Order: wire,
		}
	}

	action := modifiesToAction(modifyWires)

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action,
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)
	if err != nil {
		return BulkOrdersResponse{}, fmt.Errorf(
			"failed to sign action: %w",
			err,
		)
	}

	return post[BulkOrdersResponse](ctx, e, action, timestamp, sig)
}

// MarketOpen opens a market position
func (e *Exchange) MarketOpen(
	ctx context.Context,
	request marketOpenRequest,
	opts ...orderOption,
) (OrderResponse, error) {
	cfg := orderConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	px, err := e.getSlippagePrice(
		ctx,
		request.coin,
		request.isBuy,
		request.slippage.OrElse(DEFAULT_SLIPPAGE),
		request.px,
	)
	if err != nil {
		return OrderResponse{}, fmt.Errorf(
			"failed to get slippage price: %w",
			err,
		)
	}

	// Market order is an aggressive limit order with IoC tif
	return e.Order(
		ctx,
		OrderRequest(
			request.coin,
			request.isBuy,
			request.sz,
			px,
			WithLimitOrder(LimitOrder{Tif: "Ioc"}),
			WithReduceOnly(false),
			withCloid(request.cloid),
		),
		withBuilderInfo(cfg.builder),
	)
}

// MarketClose closes a market position
func (e *Exchange) MarketClose(
	ctx context.Context,
	request marketCloseRequest,
	opts ...orderOption,
) (OrderResponse, error) {
	cfg := orderConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	address := crypto.PubkeyToAddress(e.privateKey.PublicKey)

	if a, ok := e.accountAddress.Get(); ok {
		address = a
	}
	if v, ok := e.vaultAddress.Get(); ok {
		address = v
	}

	// Get user state to find the position
	dex := utils.GetDex(request.coin)
	userState, err := e.info.UserState(ctx, address, dex)
	if err != nil {
		return OrderResponse{}, fmt.Errorf("failed to get user state: %w", err)
	}

	// Find the position for this coin
	var position *info.Position
	var positionSize float64
	if userState.AssetPositions != nil {
		for _, assetPos := range userState.AssetPositions {
			if assetPos.Position.Coin == request.coin {
				position = &assetPos.Position
				positionSize = float64(assetPos.Position.Szi)
				break
			}
		}
	}

	if position == nil {
		return OrderResponse{}, fmt.Errorf(
			"no position found for coin: %s",
			request.coin,
		)
	}

	// Determine size to close
	var closeSz float64
	if sz, ok := request.sz.Get(); ok {
		closeSz = sz
	} else {
		// Close entire position
		closeSz = math.Abs(positionSize)
	}

	// Determine buy/sell direction (opposite of current position)
	isBuy := positionSize < 0

	px, err := e.getSlippagePrice(
		ctx,
		request.coin,
		isBuy,
		request.slippage.OrElse(DEFAULT_SLIPPAGE),
		request.px,
	)
	if err != nil {
		return OrderResponse{}, fmt.Errorf(
			"failed to get slippage price: %w",
			err,
		)
	}

	// Market order is an aggressive limit order with IoC tif
	return e.Order(
		ctx,
		OrderRequest(
			request.coin,
			isBuy,
			closeSz,
			px,
			WithLimitOrder(LimitOrder{Tif: "Ioc"}),
			WithReduceOnly(false),
			withCloid(request.cloid),
		),
		withBuilderInfo(cfg.builder),
	)
}

// Cancel cancels a single order by order ID
func (e *Exchange) Cancel(
	ctx context.Context,
	request cancelRequest,
) (CancelResponse, error) {
	responses, err := e.BulkCancel(ctx, []cancelRequest{request})
	if err != nil {
		return CancelResponse{}, err
	}
	if len(responses) == 0 {
		return CancelResponse{}, fmt.Errorf("empty response from modify order")
	}
	return CancelResponse(responses[0]), nil
}

// BulkCancel cancels multiple orders in a single transaction
func (e *Exchange) BulkCancel(
	ctx context.Context,
	cancels []cancelRequest,
) (BulkCancelResponse, error) {
	if len(cancels) == 0 {
		return BulkCancelResponse{}, fmt.Errorf(
			"at least one cancel is required",
		)
	}

	cancelWires := make([]cancelWire, len(cancels))
	for i, cancel := range cancels {
		// Get asset ID for this cancel's coin
		assetId, ok := e.info.GetAsset(cancel.Coin)
		if !ok {
			return BulkCancelResponse{}, fmt.Errorf(
				"unknown coin: %s",
				cancel.Coin,
			)
		}

		cancelWires[i] = cancel.toCancelWire(assetId)
	}

	action := cancelsToAction(cancelWires)

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action,
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)
	if err != nil {
		return BulkCancelResponse{}, fmt.Errorf(
			"failed to sign action: %w",
			err,
		)
	}

	return post[BulkCancelResponse](ctx, e, action, timestamp, sig)
}

// CancelByCloid cancels an order by its client order ID.
func (e *Exchange) CancelByCloid(
	ctx context.Context,
	request cancelByCloidRequest,
) (any, error) {
	responses, err := e.BulkCancelByCloid(ctx, []cancelByCloidRequest{request})
	if err != nil {
		return CancelResponse{}, err
	}
	if len(responses) == 0 {
		return CancelResponse{}, fmt.Errorf("empty response from modify order")
	}
	return CancelResponse(responses[0]), nil
}

func (e *Exchange) BulkCancelByCloid(
	ctx context.Context,
	cancels []cancelByCloidRequest,
) (BulkCancelResponse, error) {
	if len(cancels) == 0 {
		return BulkCancelResponse{}, fmt.Errorf(
			"at least one cancel is required",
		)
	}

	cancelWires := make([]cancelByCloidWire, len(cancels))
	for i, cancel := range cancels {
		// Get asset ID for this cancel's coin
		assetId, ok := e.info.GetAsset(cancel.Coin)
		if !ok {
			return BulkCancelResponse{}, fmt.Errorf(
				"unknown coin: %s",
				cancel.Coin,
			)
		}

		cancelWires[i] = cancel.toCancelByCloidWire(assetId)
	}

	action := cancelsByCloidToAction(cancelWires)

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action,
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)

	if err != nil {
		return BulkCancelResponse{}, fmt.Errorf(
			"failed to sign action: %w",
			err,
		)
	}

	return post[BulkCancelResponse](
		ctx,
		e,
		action,
		timestamp,
		sig,
	)
}

// Schedules a time to cancel all open orders. The time must be at least 5
// seconds. Once the duration elapses, all open orders will be canceled and a
// trigger count will be incremented. The max number of triggers per day is
// 10.
// This trigger count is reset at 00:00 UTC.
//
// if time is not nil, then set the cancel time in the future. If nil, then
// unsets any cancel time in the future.
func (e *Exchange) ScheduleCancel(
	ctx context.Context,
	request scheduleCancelRequest,
) (CancelResponse, error) {
	action, err := request.toAction(ctx, e)
	if err != nil {
		return CancelResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action.(scheduleCancelAction),
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)

	if err != nil {
		return CancelResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[CancelResponse](ctx, e, action, timestamp, sig)
}

// UpdateLeverage updates the leverage for an asset
func (e *Exchange) UpdateLeverage(
	ctx context.Context,
	request updateLeverageRequest,
) (UpdateResponse, error) {
	action, err := request.toAction(ctx, e)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action.(updateLeverageAction),
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)

	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// UpdateIsolatedMargin updates the isolated margin for an asset
func (e *Exchange) UpdateIsolatedMargin(
	ctx context.Context,
	request updateIsolatedMarginRequest,
) (UpdateResponse, error) {
	action, err := request.toAction(ctx, e)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action.(updateIsolatedMarginAction),
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)

	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// SetReferrer sets the referrer code
func (e *Exchange) SetReferrer(
	ctx context.Context,
	code string,
) (SetReferrerResponse, error) {
	req := SetReferrerRequest(code)
	action, err := req.toAction(ctx, e)
	if err != nil {
		return SetReferrerResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action.(setReferrerAction),
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)

	if err != nil {
		return SetReferrerResponse{}, fmt.Errorf(
			"failed to sign action: %w",
			err,
		)
	}

	return post[SetReferrerResponse](
		ctx,
		e,
		action,
		timestamp,
		sig,
	)
}

func (e *Exchange) CreateSubAccount(
	ctx context.Context,
	name string,
) (CreateSubAccountResponse, error) {
	req := CreateSubAccountRequest(name)
	action, err := req.toAction(ctx, e)
	if err != nil {
		return CreateSubAccountResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action.(createSubAccountAction),
		uint64(timestamp),
		e.privateKey,
		e.vaultAddress,
		e.expiresAfter,
		e.rest.IsMainnet(),
	)

	if err != nil {
		return CreateSubAccountResponse{}, fmt.Errorf(
			"failed to sign action: %w",
			err,
		)
	}

	return post[CreateSubAccountResponse](
		ctx,
		e,
		action,
		timestamp,
		sig,
	)
}

func (e *Exchange) UsdClassTransfer(
	ctx context.Context,
	amount float64,
	toPerp bool,
) (UpdateResponse, error) {
	timestamp := e.nextNonce()
	req := UsdClassTransferRequest(amount, toPerp)
	action, err := req.toAction(ctx, e, timestamp)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signUsdClassTransferAction(
		action.(usdClassTransferAction),
		e.privateKey,
	)

	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](
		ctx,
		e,
		action,
		timestamp,
		sig,
	)
}

// SendAsset is used to transfer tokens between different perp
// DEXs, spot balance, users, and/or sub-accounts. Use "" to specify the
// default
// USDC perp DEX and "spot" to specify spot. Only the collateral token can be
// transferred to or from a perp DEX.
func (e *Exchange) SendAsset(
	ctx context.Context,
	destination common.Address,
	sourceDex string,
	destinationDex string,
	token string,
	amount float64,
) (UpdateResponse, error) {
	timestamp := e.nextNonce()
	req := SendAssetRequest(
		destination,
		sourceDex,
		destinationDex,
		token,
		amount,
	)
	action, err := req.toAction(ctx, e, timestamp)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signSendAssetAction(action.(sendAssetAction), e.privateKey)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// SubAccountTransfer transfers assets between sub-accounts.
func (e *Exchange) SubAccountTransfer(
	ctx context.Context,
	subAccount common.Address,
	isDeposit bool,
	usd int64,
) (UpdateResponse, error) {
	req := SubAccountTransferRequest(subAccount, isDeposit, usd)
	action, err := req.toAction(ctx, e)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action.(subAccountTransferAction),
		uint64(timestamp),
		e.privateKey,
		mo.None[common.Address](),
		e.expiresAfter,
		e.rest.IsMainnet(),
	)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// SubAccountSpotTransfer transfers spot assets between sub-accounts.
func (e *Exchange) SubAccountSpotTransfer(
	ctx context.Context,
	subAccountUser common.Address,
	isDeposit bool,
	token string,
	amount float64,
) (UpdateResponse, error) {
	req := SubAccountSpotTransferRequest(
		subAccountUser,
		isDeposit,
		token,
		amount,
	)
	action, err := req.toAction(ctx, e)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action.(subAccountSpotTransferAction),
		uint64(timestamp),
		e.privateKey,
		mo.None[common.Address](),
		e.expiresAfter,
		e.rest.IsMainnet(),
	)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// VaultUsdTransfer transfers USD to or from a vault.
func (e *Exchange) VaultUsdTransfer(
	ctx context.Context,
	vaultAddress common.Address,
	isDeposit bool,
	usd int64,
) (UpdateResponse, error) {
	req := VaultTransferRequest(vaultAddress, isDeposit, usd)
	action, err := req.toAction(ctx, e)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signL1Action(
		action.(vaultTransferAction),
		uint64(timestamp),
		e.privateKey,
		mo.None[common.Address](),
		e.expiresAfter,
		e.rest.IsMainnet(),
	)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// UsdTransfer transfers USD to a destination perp account on
// Hyperliquid L1
func (e *Exchange) UsdTransfer(
	ctx context.Context,
	amount float64,
	destination common.Address,
) (UpdateResponse, error) {
	timestamp := e.nextNonce()
	req := UsdTransferRequest(amount, destination)
	action, err := req.toAction(ctx, e, timestamp)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signUsdTransferAction(action.(usdTransferAction), e.privateKey)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// SpotTransfer transfers spot tokens to a destination address.
func (e *Exchange) SpotTransfer(
	ctx context.Context,
	amount float64,
	destination common.Address,
	token string,
) (UpdateResponse, error) {
	timestamp := e.nextNonce()
	req := SpotTransferRequest(amount, destination, token)
	action, err := req.toAction(ctx, e, timestamp)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signSpotTransferAction(
		action.(spotTransferAction),
		e.privateKey,
	)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// TokenDelegate delegates tokens to a validator.
func (e *Exchange) TokenDelegate(
	ctx context.Context,
	validator common.Address,
	wei int64,
	isUndelegate bool,
) (UpdateResponse, error) {
	timestamp := e.nextNonce()
	req := TokenDelegateRequest(validator, wei, isUndelegate)
	action, err := req.toAction(ctx, e, timestamp)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signTokenDelegateAction(
		action.(tokenDelegateAction),
		e.privateKey,
	)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// WithdrawFromBridge withdraws tokens from the bridge.
func (e *Exchange) WithdrawFromBridge(
	ctx context.Context,
	amount float64,
	destination common.Address,
) (UpdateResponse, error) {
	timestamp := e.nextNonce()
	req := WithdrawFromBridgeRequest(amount, destination)
	action, err := req.toAction(ctx, e, timestamp)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signWithdrawFromBridgeAction(
		action.(withdrawFromBridgeAction),
		e.privateKey,
	)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// ApproveAgent approves an agent and returns the response and the agent's
// private key.
func (e *Exchange) ApproveAgent(
	ctx context.Context,
	request approveAgentRequest,
) (UpdateResponse, *ecdsa.PrivateKey, error) {
	// Generate random agent private key
	agentPrivateKey, err := crypto.GenerateKey()
	if err != nil {
		return UpdateResponse{}, nil, fmt.Errorf(
			"failed to generate agent key: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	action, err := request.toAction(ctx, e, agentPrivateKey, timestamp)
	if err != nil {
		return UpdateResponse{}, nil, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signAgentAction(action.(approveAgentAction), e.privateKey)
	if err != nil {
		return UpdateResponse{}, nil, fmt.Errorf(
			"failed to sign action: %w",
			err,
		)
	}

	result, err := post[UpdateResponse](ctx, e, action, timestamp, sig)
	if err != nil {
		return UpdateResponse{}, nil, err
	}

	return result, agentPrivateKey, nil
}

// ApproveBuilderFee approves a maximum fee rate for a builder.
// maxFeeRate should be a percent string; e.g. "0.001%"
func (e *Exchange) ApproveBuilderFee(
	ctx context.Context,
	builder common.Address,
	maxFeeRate string,
) (UpdateResponse, error) {
	timestamp := e.nextNonce()
	req := ApproveBuilderFeeRequest(builder, maxFeeRate)
	action, err := req.toAction(ctx, e, timestamp)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signApproveBuilderFeeAction(
		action.(approveBuilderFeeAction),
		e.privateKey,
	)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	result, err := post[UpdateResponse](ctx, e, action, timestamp, sig)
	if err != nil {
		return UpdateResponse{}, err
	}

	return result, nil
}

// ConvertToMultiSigUser converts the user account to a multi-sig account
func (e *Exchange) ConvertToMultiSigUser(
	ctx context.Context,
	request convertToMultiSigUserRequest,
) (UpdateResponse, error) {
	action, err := request.toAction(ctx, e)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	timestamp := e.nextNonce()
	sig, err := signConvertToMultiSigUserAction(
		action.(convertToMultiSigUserAction),
		e.privateKey,
	)
	if err != nil {
		return UpdateResponse{}, fmt.Errorf("failed to sign action: %w", err)
	}

	return post[UpdateResponse](ctx, e, action, timestamp, sig)
}

// // SpotDeployRegisterToken registers a token for spot deployment
// func (e *Exchange) SpotDeployRegisterToken(
// 	ctx context.Context,
// 	tokenName string,
// 	szDecimals int64,
// 	weiDecimals int64,
// 	maxGas int64,
// 	fullName string,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type": "spotDeploy",
// 		"registerToken2": map[string]any{
// 			"spec": map[string]any{
// 				"name":        tokenName,
// 				"szDecimals":  szDecimals,
// 				"weiDecimals": weiDecimals,
// 			},
// 			"maxGas":   maxGas,
// 			"fullName": fullName,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}

// 	return e.post(ctx, action, timestamp, sig)
// }

// // SpotDeployUserGenesis performs user genesis for spot deployment
// func (e *Exchange) SpotDeployUserGenesis(
// 	ctx context.Context,
// 	token int64,
// 	userAndWei []UserWeiPair,
// 	existingTokenAndWei []TokenWeiPair,
// ) (Response, error) {
//  timestamp := e.nextNonce()

// 	// Convert userAndWei to lowercase addresses and string wei
// 	userAndWeiAction := make([][]string, len(userAndWei))
// 	for i, pair := range userAndWei {
// 		userAndWeiAction[i] = []string{
// 			strings.ToLower(pair.User.String()),
// 			pair.Wei.String(),
// 		}
// 	}

// 	// Convert existingTokenAndWei to action format
// 	existingTokenAndWeiAction := make([][]any, len(existingTokenAndWei))
// 	for i, pair := range existingTokenAndWei {
// 		existingTokenAndWeiAction[i] = []any{
// 			pair.Token,
// 			pair.Wei.String(),
// 		}
// 	}

// 	action := map[string]any{
// 		"type": "spotDeploy",
// 		"userGenesis": map[string]any{
// 			"token":               token,
// 			"userAndWei":          userAndWeiAction,
// 			"existingTokenAndWei": existingTokenAndWeiAction,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}

// 	return e.post(ctx, action, timestamp, sig)
// }

// // spotDeployTokenActionInner is a helper for simple spot deploy token
// actions
// func (e *Exchange) spotDeployTokenActionInner(
// 	ctx context.Context,
// 	variant string,
// 	token int64,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type": "spotDeploy",
// 		variant: map[string]any{
// 			"token": token,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // SpotDeployEnableFreezePrivilege enables freeze privilege for a token
// func (e *Exchange) SpotDeployEnableFreezePrivilege(
// 	ctx context.Context,
// 	token int64,
// ) (Response, error) {
// 	return e.spotDeployTokenActionInner(ctx, "enableFreezePrivilege", token)
// }

// // SpotDeployRevokeFreezePrivilege revokes freeze privilege for a token
// func (e *Exchange) SpotDeployRevokeFreezePrivilege(
// 	ctx context.Context,
// 	token int64,
// ) (Response, error) {
// 	return e.spotDeployTokenActionInner(ctx, "revokeFreezePrivilege", token)
// }

// // SpotDeployEnableQuoteToken enables a token as a quote asset
// func (e *Exchange) SpotDeployEnableQuoteToken(
// 	ctx context.Context,
// 	token int64,
// ) (Response, error) {
// 	return e.spotDeployTokenActionInner(ctx, "enableQuoteToken", token)
// }

// // SpotDeployFreezeUser freezes or unfreezes a user
// func (e *Exchange) SpotDeployFreezeUser(
// 	ctx context.Context,
// 	token int64,
// 	user common.Address,
// 	freeze bool,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type": "spotDeploy",
// 		"freezeUser": map[string]any{
// 			"token":  token,
// 			"user":   strings.ToLower(user.String()),
// 			"freeze": freeze,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // SpotDeployGenesis sets up genesis configuration for a token
// func (e *Exchange) SpotDeployGenesis(
// 	ctx context.Context,
// 	token int64,
// 	maxSupply string,
// 	noHyperliquidity bool,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	genesis := map[string]any{
// 		"token":     token,
// 		"maxSupply": maxSupply,
// 	}
// 	if noHyperliquidity {
// 		genesis["noHyperliquidity"] = true
// 	}
// 	action := map[string]any{
// 		"type":    "spotDeploy",
// 		"genesis": genesis,
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // SpotDeployRegisterSpot registers a spot trading pair
// func (e *Exchange) SpotDeployRegisterSpot(
// 	ctx context.Context,
// 	baseToken int64,
// 	quoteToken int64,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type": "spotDeploy",
// 		"registerSpot": map[string]any{
// 			"tokens": []int64{baseToken, quoteToken},
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // SpotDeployRegisterHyperliquidity registers hyperliquidity market maker
// func (e *Exchange) SpotDeployRegisterHyperliquidity(
// 	ctx context.Context,
// 	spot int64,
// 	startPx float64,
// 	orderSz float64,
// 	nOrders int64,
// 	nSeededLevels *int64,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	registerHyperliquidity := map[string]any{
// 		"spot":    spot,
// 		"startPx": fmt.Sprintf("%v", startPx),
// 		"orderSz": fmt.Sprintf("%v", orderSz),
// 		"nOrders": nOrders,
// 	}
// 	if nSeededLevels != nil {
// 		registerHyperliquidity["nSeededLevels"] = *nSeededLevels
// 	}
// 	action := map[string]any{
// 		"type":                   "spotDeploy",
// 		"registerHyperliquidity": registerHyperliquidity,
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // SpotDeploySetDeployerTradingFeeShare sets the deployer trading fee share
// func (e *Exchange) SpotDeploySetDeployerTradingFeeShare(
// 	ctx context.Context,
// 	token int64,
// 	share string,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type": "spotDeploy",
// 		"setDeployerTradingFeeShare": map[string]any{
// 			"token": token,
// 			"share": share,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // PerpDeploySchemaInput represents schema input for perp deployment
// type PerpDeploySchemaInput struct {
// 	FullName        string
// 	CollateralToken string
// 	OracleUpdater   *common.Address
// }

// // PerpDeployRegisterAsset registers a new perpetual asset
// func (e *Exchange) PerpDeployRegisterAsset(
// 	ctx context.Context,
// 	dex string,
// 	maxGas *int64,
// 	coin string,
// 	szDecimals int64,
// 	oraclePx string,
// 	marginTableID int64,
// 	onlyIsolated bool,
// 	schema *PerpDeploySchemaInput,
// ) (Response, error) {
//  timestamp := e.nextNonce()

// 	var schemaWire map[string]any
// 	if schema != nil {
// 		schemaWire = map[string]any{
// 			"fullName":        schema.FullName,
// 			"collateralToken": schema.CollateralToken,
// 		}
// 		if schema.OracleUpdater != nil {
// 			schemaWire["oracleUpdater"] = strings.ToLower(
// 				schema.OracleUpdater.String(),
// 			)
// 		} else {
// 			schemaWire["oracleUpdater"] = nil
// 		}
// 	}

// 	action := map[string]any{
// 		"type": "perpDeploy",
// 		"registerAsset": map[string]any{
// 			"maxGas": maxGas,
// 			"assetRequest": map[string]any{
// 				"coin":          coin,
// 				"szDecimals":    szDecimals,
// 				"oraclePx":      oraclePx,
// 				"marginTableId": marginTableID,
// 				"onlyIsolated":  onlyIsolated,
// 			},
// 			"dex":    dex,
// 			"schema": schemaWire,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // PerpDeploySetOracle sets oracle prices for a DEX
// func (e *Exchange) PerpDeploySetOracle(
// 	ctx context.Context,
// 	dex string,
// 	oraclePxs map[string]string,
// 	allMarkPxs []map[string]string,
// 	externalPerpPxs map[string]string,
// ) (Response, error) {
//  timestamp := e.nextNonce()

// 	// Convert maps to sorted key-value pairs
// 	oraclePxsWire := sortStringMap(oraclePxs)
// 	markPxsWire := make([][][]string, len(allMarkPxs))
// 	for i, markPxs := range allMarkPxs {
// 		markPxsWire[i] = sortStringMap(markPxs)
// 	}
// 	externalPerpPxsWire := sortStringMap(externalPerpPxs)

// 	action := map[string]any{
// 		"type": "perpDeploy",
// 		"setOracle": map[string]any{
// 			"dex":             dex,
// 			"oraclePxs":       oraclePxsWire,
// 			"markPxs":         markPxsWire,
// 			"externalPerpPxs": externalPerpPxsWire,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // cSignerInner is a helper for c signer actions
// func (e *Exchange) cSignerInner(
// 	ctx context.Context,
// 	variant string,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type":  "CSignerAction",
// 		variant: nil,
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // CSignerUnjailSelf unjails the signer
// func (e *Exchange) CSignerUnjailSelf(ctx context.Context) (Response, error) {
// 	return e.cSignerInner(ctx, "unjailSelf")
// }

// // CSignerJailSelf jails the signer
// func (e *Exchange) CSignerJailSelf(ctx context.Context) (Response, error) {
// 	return e.cSignerInner(ctx, "jailSelf")
// }

// // CValidatorRegisterProfile represents validator profile configuration
// type CValidatorRegisterProfile struct {
// 	NodeIP              string
// 	Name                string
// 	Description         string
// 	DelegationsDisabled bool
// 	CommissionBps       int64
// 	Signer              string
// }

// // CValidatorRegister registers a new validator
// func (e *Exchange) CValidatorRegister(
// 	ctx context.Context,
// 	profile CValidatorRegisterProfile,
// 	unjailed bool,
// 	initialWei int64,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type": "CValidatorAction",
// 		"register": map[string]any{
// 			"profile": map[string]any{
// 				"node_ip": map[string]any{
// 					"Ip": profile.NodeIP,
// 				},
// 				"name":                 profile.Name,
// 				"description":          profile.Description,
// 				"delegations_disabled": profile.DelegationsDisabled,
// 				"commission_bps":       profile.CommissionBps,
// 				"signer":               profile.Signer,
// 			},
// 			"unjailed":    unjailed,
// 			"initial_wei": initialWei,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // CValidatorChangeProfileOptions represents optional changes to validator
// // profile
// type CValidatorChangeProfileOptions struct {
// 	NodeIP             *string
// 	Name               *string
// 	Description        *string
// 	DisableDelegations *bool
// 	CommissionBps      *int64
// 	Signer             *string
// }

// // CValidatorChangeProfile updates validator profile
// func (e *Exchange) CValidatorChangeProfile(
// 	ctx context.Context,
// 	unjailed bool,
// 	options CValidatorChangeProfileOptions,
// ) (Response, error) {
//  timestamp := e.nextNonce()

// 	var nodeIP any
// 	if options.NodeIP != nil {
// 		nodeIP = map[string]any{"Ip": *options.NodeIP}
// 	}

// 	action := map[string]any{
// 		"type": "CValidatorAction",
// 		"changeProfile": map[string]any{
// 			"node_ip":             nodeIP,
// 			"name":                options.Name,
// 			"description":         options.Description,
// 			"unjailed":            unjailed,
// 			"disable_delegations": options.DisableDelegations,
// 			"commission_bps":      options.CommissionBps,
// 			"signer":              options.Signer,
// 		},
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // CValidatorUnregister unregisters the validator
// func (e *Exchange) CValidatorUnregister(ctx context.Context) (Response,
// error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type":       "CValidatorAction",
// 		"unregister": nil,
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // UseBigBlocks enables or disables big blocks for EVM user modifications
// func (e *Exchange) UseBigBlocks(
// 	ctx context.Context,
// 	enable bool,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type":           "evmUserModify",
// 		"usingBigBlocks": enable,
// 	}
// 	sig, err := e.signL1Action(action, uint64(timestamp))
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // AgentEnableDexAbstraction enables DEX abstraction for the agent
// func (e *Exchange) AgentEnableDexAbstraction(
// 	ctx context.Context,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type": "agentEnableDexAbstraction",
// 	}
// 	sig, err := e.signL1ActionWithVault(
// 		action,
// 		uint64(timestamp),
// 		e.vaultAddress,
// 	)
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// // UserDexAbstraction enables or disables DEX abstraction for a user
// func (e *Exchange) UserDexAbstraction(
// 	ctx context.Context,
// 	user common.Address,
// 	enabled bool,
// ) (Response, error) {
//  timestamp := e.nextNonce()
// 	action := map[string]any{
// 		"type":    "userDexAbstraction",
// 		"user":    strings.ToLower(user.String()),
// 		"enabled": enabled,
// 		"nonce":   timestamp,
// 	}
// 	sig, err := e.signUserDexAbstractionAction(action)
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return e.post(ctx, action, timestamp, sig)
// }

// MultiSig executes a multi-signature transaction
// Use the generic Resp to specify the response type of the action
// and T to specify the type of the inner request
func MultiSig[Resp any, T request](
	ctx context.Context,
	e *Exchange,
	request multiSigRequest[T],
) (Resp, error) {
	action, err := request.toAction(ctx, e)
	if err != nil {
		var noResp Resp
		return noResp, fmt.Errorf(
			"failed to convert request to action: %w",
			err,
		)
	}

	sig, err := signMultiSigAction(
		action.(multiSigAction),
		uint64(request.nonce),
		e.privateKey,
		request.vaultAddress,
		mo.None[time.Duration](),
		e.rest.IsMainnet(),
	)

	var noResp Resp
	if err != nil {
		return noResp, fmt.Errorf("failed to sign action: %w", err)
	}

	result, err := post[Resp](ctx, e, action, request.nonce, sig)
	if err != nil {
		return noResp, err
	}

	return result, nil
}

// Noop sends a no-operation action
// func (e *Exchange) Noop(ctx context.Context, nonce int64) (Response, error) {
// 	action := map[string]any{
// 		"type": "noop",
// 	}
// 	sig, err := e.signL1ActionWithVault(action, uint64(nonce), e.vaultAddress)
// 	if err != nil {
// 		return Response{}, fmt.Errorf("failed to sign action: %w", err)
// 	}
// 	return post(ctx, action, nonce, sig)
// }

// sortStringMap converts a map to sorted key-value pairs
func sortStringMap(m map[string]string) [][]string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	result := make([][]string, len(keys))
	for i, k := range keys {
		result[i] = []string{k, m[k]}
	}
	return result
}

func post[T any, U action](
	ctx context.Context,
	exchange *Exchange,
	action U,
	timestamp int64,
	sig signature,
) (T, error) {
	payload := map[string]any{
		"action":    action,
		"signature": sig,
		"nonce":     timestamp,
	}

	actionType := action.getType()
	if actionType == "usdClassTransfer" || actionType == "sendAsset" {
		payload["vaultAddress"] = nil
	} else if v, ok := exchange.vaultAddress.Get(); ok {
		payload["vaultAddress"] = v
	} else {
		payload["vaultAddress"] = nil
	}

	if e, ok := exchange.expiresAfter.Get(); ok {
		payload["expiresAfter"] = e
	} else {
		payload["expiresAfter"] = nil
	}

	var zero T
	var response response[T]
	if err := exchange.rest.Post(ctx, "/exchange", payload, &response); err != nil {
		return zero, fmt.Errorf(
			"failed to post to /exchange. Type: %v: %w",
			actionType,
			err,
		)
	}

	if response.IsErr() {
		return zero, fmt.Errorf(
			"exchange error (action: %v): %s",
			actionType,
			response.ErrorMessage,
		)
	}

	return *response.Data, nil
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
		dex := utils.GetDex(coin)

		mids, err := e.info.AllMids(ctx, dex)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch mid prices: %w", err)
		}

		midPrice, ok := mids[coin]
		if !ok {
			return 0, fmt.Errorf("mid price not found for coin: %s", coin)
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
	px = utils.RoundToSigfig(px, 5)

	// 5. Final decimal rounding:
	// Python: round(px_5sig, (6 if not is_spot else 8) -
	// asset_to_sz_decimals[asset])
	baseDecimals := int64(6)
	if isSpot {
		baseDecimals = 8
	}

	szDecimals, ok := e.info.AssetToSzDecimals(asset)
	if !ok {
		return 0, fmt.Errorf("asset sz decimals not found for asset: %d", asset)
	}

	decimals := baseDecimals - szDecimals
	px = utils.RoundToDecimals(px, decimals)

	return px, nil
}

// nextNonce returns a strictly increasing nonce suitable for Hyperliquid.
// Hyperliquid requires each transaction’s nonce to be unique, unused, and
// greater than the smallest of the last 100 nonces, while remaining close to
// the current unix millisecond timestamp. This method uses an atomic CAS loop
// to ensure monotonic, time-based nonces safe for high-throughput order flow.
func (e *Exchange) nextNonce() int64 {
	for {
		prev := e.prevNonce.Load()
		curr := time.Now().UnixMilli()

		if curr <= prev {
			curr = prev + 1
		}

		if e.prevNonce.CompareAndSwap(prev, curr) {
			return curr
		}
	}
}

func getSignatureChainId() string {
	return fmt.Sprintf("0x%x", constants.SIGNATURE_CHAIN_ID)
}
