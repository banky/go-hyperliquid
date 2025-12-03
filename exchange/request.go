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

	"github.com/banky/go-hyperliquid/info"
	"github.com/banky/go-hyperliquid/internal/utils"
	"github.com/banky/go-hyperliquid/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/samber/mo"
)

// ============================================================================
// Request and Action Interfaces
// ============================================================================

// action is an interface for all action types that can be signed and posted
type action interface {
	getType() string
}

// request is an interface for all request types that can be converted to actions
type request interface {
	toAction(ctx context.Context, e *Exchange, opts ...any) (action, error)
}

// ============================================================================
// Order Types
// ============================================================================

type OrderType struct {
	Limit   *LimitOrder
	Trigger *TriggerOrder
}

type LimitOrder struct {
	Tif string `json:"tif"`
}

type TriggerOrder struct {
	IsMarket  bool
	TriggerPx float64
	TpSl      string
}

type orderTypeWire struct {
	Limit   *LimitOrder       `json:"limit,omitempty"`
	Trigger *triggerOrderWire `json:"trigger,omitempty"`
}

type triggerOrderWire struct {
	IsMarket  bool   `json:"isMarket"`
	TriggerPx string `json:"triggerPx"`
	TpSl      string `json:"tpsl"`
}

type BuilderInfo struct {
	// Public address of the builder
	PublicAddress common.Address `json:"b"`
	// Amount of the fee in tenths of basis points.
	// eg. 10 means 1 basis point
	FeeAmount int64 `json:"f"`
}

// toOrderTypeWire converts OrderType to wire format
func (t OrderType) toOrderTypeWire() (orderTypeWire, error) {
	wire := orderTypeWire{}

	if t.Limit != nil {
		wire.Limit = &LimitOrder{
			Tif: t.Limit.Tif,
		}
	}

	if t.Trigger != nil {
		// Convert to wire format
		triggerPxStr, err := utils.FloatToWire(t.Trigger.TriggerPx)
		if err != nil {
			return orderTypeWire{}, fmt.Errorf(
				"failed to convert trigger price: %w",
				err,
			)
		}

		wire.Trigger = &triggerOrderWire{
			IsMarket:  t.Trigger.IsMarket,
			TriggerPx: triggerPxStr,
			TpSl:      t.Trigger.TpSl,
		}
	}

	return wire, nil
}

// ============================================================================
// Order Request
// ============================================================================

type orderRequest struct {
	coin       string
	isBuy      bool
	sz         float64
	limitPx    float64
	orderType  OrderType
	reduceOnly bool
	cloid      mo.Option[types.Cloid]
}

type orderRequestOption func(*orderRequestConfig)

type orderRequestConfig struct {
	reduceOnly   bool
	cloid        mo.Option[types.Cloid]
	limitOrder   mo.Option[LimitOrder]
	triggerOrder mo.Option[TriggerOrder]
}

func OrderRequest(
	coin string,
	isBuy bool,
	sz float64,
	limitPx float64,
	opts ...orderRequestOption,
) orderRequest {
	cfg := orderRequestConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	var orderType OrderType
	if l, ok := cfg.limitOrder.Get(); ok {
		orderType.Limit = &l
	} else if t, ok := cfg.triggerOrder.Get(); ok {
		orderType.Trigger = &t
	} else {
		panic("Failed to create OrderRequest. OrderType must be set")
	}

	return orderRequest{
		coin:       coin,
		isBuy:      isBuy,
		sz:         sz,
		limitPx:    limitPx,
		orderType:  orderType,
		reduceOnly: cfg.reduceOnly,
		cloid:      cfg.cloid,
	}
}

// WithReduceOnly sets the reduce-only flag
func WithReduceOnly(reduceOnly bool) orderRequestOption {
	return func(cfg *orderRequestConfig) {
		cfg.reduceOnly = reduceOnly
	}
}

// WithCloid sets the client order ID
func WithCloid(c types.Cloid) orderRequestOption {
	return func(cfg *orderRequestConfig) {
		cfg.cloid = mo.Some(c)
	}
}

func withCloid(c mo.Option[types.Cloid]) orderRequestOption {
	return func(cfg *orderRequestConfig) {
		cfg.cloid = c
	}
}

func WithLimitOrder(limitOrder LimitOrder) orderRequestOption {
	return func(cfg *orderRequestConfig) {
		cfg.limitOrder = mo.Some(limitOrder)
	}
}

func WithTriggerOrder(triggerOrder TriggerOrder) orderRequestOption {
	return func(cfg *orderRequestConfig) {
		cfg.triggerOrder = mo.Some(triggerOrder)
	}
}

// toAction converts an orderRequest to an orderAction
func (o orderRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract builder and grouping from opts
	var builder mo.Option[BuilderInfo]
	var grouping mo.Option[OrderGrouping]

	for _, opt := range opts {
		switch v := opt.(type) {
		case BuilderInfo:
			builder = mo.Some(v)
		case OrderGrouping:
			grouping = mo.Some(v)
		}
	}

	// Get asset ID for this order's coin
	assetId, ok := e.info.GetAsset(o.coin)
	if !ok {
		return nil, fmt.Errorf("unknown coin: %s", o.coin)
	}

	// Convert order to wire format
	wire, err := o.toOrderWire(assetId)
	if err != nil {
		return nil, fmt.Errorf("failed to convert order to wire: %w", err)
	}

	// Create action from the wire
	return ordersToAction([]orderWire{wire}, builder, grouping), nil
}

type orderWire struct {
	A int64         `json:"a"`
	B bool          `json:"b"`
	P string        `json:"p"`
	S string        `json:"s"`
	R bool          `json:"r"`
	T orderTypeWire `json:"t"`
	C *types.Cloid  `json:"c,omitempty"`
}

// toOrderWire converts OrderRequest to OrderWire
func (o orderRequest) toOrderWire(assetId int64) (orderWire, error) {
	// Convert sizes and prices to wire format
	sizeStr, err := utils.FloatToWire(o.sz)
	if err != nil {
		return orderWire{}, fmt.Errorf("failed to convert size: %w", err)
	}

	priceStr, err := utils.FloatToWire(o.limitPx)
	if err != nil {
		return orderWire{}, fmt.Errorf("failed to convert limit price: %w", err)
	}

	// Convert order type to wire format
	orderTypeWire, err := o.orderType.toOrderTypeWire()
	if err != nil {
		return orderWire{}, fmt.Errorf("failed to convert order type: %w", err)
	}

	return orderWire{
		A: int64(assetId),
		B: o.isBuy,
		P: priceStr,
		S: sizeStr,
		R: o.reduceOnly,
		T: orderTypeWire,
		C: o.cloid.ToPointer(),
	}, nil
}

type orderAction struct {
	Type     string        `json:"type"`
	Orders   []orderWire   `json:"orders"`
	Grouping OrderGrouping `json:"grouping"`
	Builder  *BuilderInfo  `json:"builder,omitempty"`
}

type OrderGrouping string

const (
	OrderGroupingNA           = "na"
	OrderGroupingNormalTpSl   = "normalTpsl"
	OrderGroupingPositionTpSl = "positionTpsl"
)

func (o orderAction) getType() string {
	return o.Type
}

func ordersToAction(
	orders []orderWire,
	builder mo.Option[BuilderInfo],
	grouping mo.Option[OrderGrouping],
) orderAction {
	action := orderAction{
		Type:   "order",
		Orders: orders,
	}

	if g, ok := grouping.Get(); ok {
		action.Grouping = g
	} else {
		action.Grouping = OrderGroupingNA
	}

	if b, ok := builder.Get(); ok {
		action.Builder = &b
	}

	return action
}

// ============================================================================
// Modify Request
// ============================================================================

type modifyRequest struct {
	Oid   mo.Option[int64]
	Cloid mo.Option[types.Cloid]
	Order orderRequest
}

type modifyRequestOption func(*modifyRequestConfig)

type modifyRequestConfig struct {
	oid   mo.Option[int64]
	cloid mo.Option[types.Cloid]
}

// ModifyRequest creates a new modify order request
func ModifyRequest(
	order orderRequest,
	opts ...modifyRequestOption,
) modifyRequest {
	cfg := modifyRequestConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.oid.IsNone() && cfg.cloid.IsNone() {
		panic(
			"failed to create modify request. either order ID or CLOID must be provided",
		)
	}

	return modifyRequest{
		Oid:   cfg.oid,
		Cloid: cfg.cloid,
		Order: order,
	}
}

func WithModifyOrderId(id int64) modifyRequestOption {
	return func(mrc *modifyRequestConfig) {
		mrc.oid = mo.Some(id)
	}
}

func WithModifyCloid(c types.Cloid) modifyRequestOption {
	return func(mrc *modifyRequestConfig) {
		mrc.cloid = mo.Some(c)
	}
}

// toAction converts a modifyRequest to a batchModifyAction
func (m modifyRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Get asset ID for this modify's coin
	assetId, ok := e.info.GetAsset(m.Order.coin)
	if !ok {
		return nil, fmt.Errorf("unknown coin: %s", m.Order.coin)
	}

	// Convert order to wire format
	wire, err := m.Order.toOrderWire(assetId)
	if err != nil {
		return nil, fmt.Errorf("failed to convert order to wire: %w", err)
	}

	// Extract OID or CLOID
	var oid any
	if o, ok := m.Oid.Get(); ok {
		oid = o
	} else if c, ok := m.Cloid.Get(); ok {
		oid = c
	} else {
		return nil, fmt.Errorf("invalid OID type for modify: either order ID or CLOID must be provided")
	}

	// Create modify wire and action
	mw := modifyWire{
		Oid:   oid,
		Order: wire,
	}

	return modifiesToAction([]modifyWire{mw}), nil
}

type modifyWire struct {
	Oid   any       `json:"oid"`
	Order orderWire `json:"order"`
}

type batchModifyAction struct {
	Type     string       `json:"type"`
	Modifies []modifyWire `json:"modifies"`
}

func (b batchModifyAction) getType() string {
	return b.Type
}

// modifiesToAction converts a list of ModifyWires to a batch modify action
func modifiesToAction(modifies []modifyWire) batchModifyAction {
	return batchModifyAction{
		Type:     "batchModify",
		Modifies: modifies,
	}
}

// ============================================================================
// Cancel Request
// ============================================================================

type cancelRequest struct {
	Coin string
	Oid  int64
}

// CancelRequest creates a new cancel request with an order ID
func CancelRequest(coin string, oid int64) cancelRequest {
	return cancelRequest{
		Coin: coin,
		Oid:  oid,
	}
}

// toAction converts a cancelRequest to a cancelAction
func (c cancelRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Get asset ID for this cancel's coin
	assetId, ok := e.info.GetAsset(c.Coin)
	if !ok {
		return nil, fmt.Errorf("unknown coin: %s", c.Coin)
	}

	// Convert cancel to wire format
	cw := c.toCancelWire(assetId)

	// Create action
	return cancelsToAction([]cancelWire{cw}), nil
}

type cancelWire struct {
	AssetId int64 `json:"a"`
	Oid     int64 `json:"o"`
}

// toCancelWire converts CancelRequest to CancelWire
func (c cancelRequest) toCancelWire(assetId int64) cancelWire {
	return cancelWire{
		AssetId: int64(assetId),
		Oid:     int64(c.Oid),
	}
}

type cancelAction struct {
	Type    string       `json:"type"`
	Cancels []cancelWire `json:"cancels"`
}

func (c cancelAction) getType() string {
	return c.Type
}

// cancelsToAction converts a list of CancelRequests to a cancel action
func cancelsToAction(cancels []cancelWire) cancelAction {
	return cancelAction{
		Type:    "cancel",
		Cancels: cancels,
	}
}

// ============================================================================
// Cancel by CLOID Request
// ============================================================================

type cancelByCloidRequest struct {
	Coin  string
	Cloid types.Cloid
}

// CancelByCloidRequest creates a new cancel request with a CLOID
func CancelByCloidRequest(
	coin string,
	cloid types.Cloid,
) cancelByCloidRequest {
	return cancelByCloidRequest{
		Coin:  coin,
		Cloid: cloid,
	}
}

// toAction converts a cancelByCloidRequest to a cancelByCloidAction
func (c cancelByCloidRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Get asset ID for this cancel's coin
	assetId, ok := e.info.GetAsset(c.Coin)
	if !ok {
		return nil, fmt.Errorf("unknown coin: %s", c.Coin)
	}

	// Convert cancel to wire format
	cw := c.toCancelByCloidWire(assetId)

	// Create action
	return cancelsByCloidToAction([]cancelByCloidWire{cw}), nil
}

type cancelByCloidWire struct {
	AssetId int64       `json:"asset"`
	Cloid   types.Cloid `json:"cloid"`
}

func (c cancelByCloidRequest) toCancelByCloidWire(
	assetId int64,
) cancelByCloidWire {
	return cancelByCloidWire{
		AssetId: int64(assetId),
		Cloid:   c.Cloid,
	}
}

type cancelByCloidAction struct {
	Type    string              `json:"type"`
	Cancels []cancelByCloidWire `json:"cancels"`
}

func (c cancelByCloidAction) getType() string {
	return c.Type
}

// cancelsByCloidToAction converts a list of CancelRequestsByCloid to a
// cancelByCloid action
func cancelsByCloidToAction(cancels []cancelByCloidWire) cancelByCloidAction {
	return cancelByCloidAction{
		Type:    "cancelByCloid",
		Cancels: cancels,
	}
}

// ============================================================================
// Market Open Request
// ============================================================================

type marketOpenRequest struct {
	coin     string
	isBuy    bool
	sz       float64
	px       mo.Option[float64]
	slippage mo.Option[float64]
	cloid    mo.Option[types.Cloid]
}

type marketOpenRequestOption func(*marketOpenRequestConfig)

type marketOpenRequestConfig struct {
	px       mo.Option[float64]
	slippage mo.Option[float64]
	cloid    mo.Option[types.Cloid]
}

// MarketOpenRequest creates a new market order request
func MarketOpenRequest(
	coin string,
	isBuy bool,
	sz float64,
	opts ...marketOpenRequestOption,
) marketOpenRequest {
	cfg := marketOpenRequestConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return marketOpenRequest{
		coin:     coin,
		isBuy:    isBuy,
		sz:       sz,
		px:       cfg.px,
		slippage: cfg.slippage,
		cloid:    cfg.cloid,
	}
}

// WithMarketPrice sets the price for a market order
func WithMarketPrice(px float64) marketOpenRequestOption {
	return func(cfg *marketOpenRequestConfig) {
		cfg.px = mo.Some(px)
	}
}

// WithMarketSlippage sets the slippage tolerance for a market order
func WithMarketSlippage(slippage float64) marketOpenRequestOption {
	return func(cfg *marketOpenRequestConfig) {
		cfg.slippage = mo.Some(slippage)
	}
}

// WithMarketCloid sets the client order ID for a market order
func WithMarketCloid(c types.Cloid) marketOpenRequestOption {
	return func(cfg *marketOpenRequestConfig) {
		cfg.cloid = mo.Some(c)
	}
}

// toAction converts a marketOpenRequest to an orderAction
// Note: This optionally accepts builder in opts
func (m marketOpenRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract builder from opts
	var builder mo.Option[BuilderInfo]
	for _, opt := range opts {
		if b, ok := opt.(BuilderInfo); ok {
			builder = mo.Some(b)
			break
		}
	}

	// Get slippage price
	px, err := e.getSlippagePrice(
		ctx,
		m.coin,
		m.isBuy,
		m.slippage.OrElse(DEFAULT_SLIPPAGE),
		m.px,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get slippage price: %w", err)
	}

	// Create an order request with IoC tif and reduceOnly=false
	orderReq := OrderRequest(
		m.coin,
		m.isBuy,
		m.sz,
		px,
		WithLimitOrder(LimitOrder{Tif: "Ioc"}),
		WithReduceOnly(false),
		withCloid(m.cloid),
	)

	// Convert order to action
	return orderReq.toAction(ctx, e, builder)
}

// ============================================================================
// Market Close Request
// ============================================================================

type marketCloseRequest struct {
	coin     string
	sz       mo.Option[float64]
	px       mo.Option[float64]
	slippage mo.Option[float64]
	cloid    mo.Option[types.Cloid]
}

type marketCloseRequestOption func(*marketCloseRequestConfig)

type marketCloseRequestConfig struct {
	sz       mo.Option[float64]
	px       mo.Option[float64]
	slippage mo.Option[float64]
	cloid    mo.Option[types.Cloid]
}

// MarketCloseRequest creates a new market close request
func MarketCloseRequest(
	coin string,
	opts ...marketCloseRequestOption,
) marketCloseRequest {
	cfg := marketCloseRequestConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return marketCloseRequest{
		coin:     coin,
		sz:       cfg.sz,
		px:       cfg.px,
		slippage: cfg.slippage,
		cloid:    cfg.cloid,
	}
}

// WithMarketCloseSize sets the size for a market close request
func WithMarketCloseSize(sz float64) marketCloseRequestOption {
	return func(cfg *marketCloseRequestConfig) {
		cfg.sz = mo.Some(sz)
	}
}

// WithMarketClosePrice sets the price for a market close request
func WithMarketClosePrice(px float64) marketCloseRequestOption {
	return func(cfg *marketCloseRequestConfig) {
		cfg.px = mo.Some(px)
	}
}

// WithMarketCloseSlippage sets the slippage tolerance for a market close
// request
func WithMarketCloseSlippage(slippage float64) marketCloseRequestOption {
	return func(cfg *marketCloseRequestConfig) {
		cfg.slippage = mo.Some(slippage)
	}
}

// WithMarketCloseCloid sets the client order ID for a market close request
func WithMarketCloseCloid(c types.Cloid) marketCloseRequestOption {
	return func(cfg *marketCloseRequestConfig) {
		cfg.cloid = mo.Some(c)
	}
}

// toAction converts a marketCloseRequest to an orderAction
// Note: This optionally accepts builder in opts
func (m marketCloseRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract builder from opts
	var builder mo.Option[BuilderInfo]
	for _, opt := range opts {
		if b, ok := opt.(BuilderInfo); ok {
			builder = mo.Some(b)
			break
		}
	}

	// Get user state to find the position
	address := crypto.PubkeyToAddress(e.privateKey.PublicKey)
	if a, ok := e.accountAddress.Get(); ok {
		address = a
	}
	if v, ok := e.vaultAddress.Get(); ok {
		address = v
	}

	dex := utils.GetDex(m.coin)
	userState, err := e.info.UserState(ctx, address, dex)
	if err != nil {
		return nil, fmt.Errorf("failed to get user state: %w", err)
	}

	// Find the position for this coin
	var position *info.Position
	var positionSize float64
	if userState.AssetPositions != nil {
		for _, assetPos := range userState.AssetPositions {
			if assetPos.Position.Coin == m.coin {
				position = &assetPos.Position
				positionSize = float64(assetPos.Position.Szi)
				break
			}
		}
	}

	if position == nil {
		return nil, fmt.Errorf("no position found for coin: %s", m.coin)
	}

	// Determine size to close
	var closeSz float64
	if sz, ok := m.sz.Get(); ok {
		closeSz = sz
	} else {
		// Close entire position
		closeSz = math.Abs(positionSize)
	}

	// Determine buy/sell direction (opposite of current position)
	isBuy := positionSize < 0

	// Get slippage price
	px, err := e.getSlippagePrice(
		ctx,
		m.coin,
		isBuy,
		m.slippage.OrElse(DEFAULT_SLIPPAGE),
		m.px,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get slippage price: %w", err)
	}

	// Create an order request with IoC tif and reduceOnly=false
	orderReq := OrderRequest(
		m.coin,
		isBuy,
		closeSz,
		px,
		WithLimitOrder(LimitOrder{Tif: "Ioc"}),
		WithReduceOnly(false),
		withCloid(m.cloid),
	)

	// Convert order to action
	return orderReq.toAction(ctx, e, builder)
}

// ============================================================================
// Update Leverage Request
// ============================================================================

type updateLeverageRequest struct {
	coin     string
	leverage int64
	isCross  mo.Option[bool]
}

type updateLeverageRequestOption func(*updateLeverageRequestConfig)

type updateLeverageRequestConfig struct {
	isCross mo.Option[bool]
}

// UpdateLeverageRequest creates a new update leverage request
func UpdateLeverageRequest(
	coin string,
	leverage int64,
	opts ...updateLeverageRequestOption,
) updateLeverageRequest {
	cfg := updateLeverageRequestConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	return updateLeverageRequest{
		coin:     coin,
		leverage: leverage,
		isCross:  cfg.isCross,
	}
}

// WithIsCross sets whether to use cross margin (default is true)
func WithIsCross(isCross bool) updateLeverageRequestOption {
	return func(cfg *updateLeverageRequestConfig) {
		cfg.isCross = mo.Some(isCross)
	}
}

// toAction converts an updateLeverageRequest to an updateLeverageAction
func (u updateLeverageRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Get asset ID for the leverage update
	assetId, ok := e.info.GetAsset(u.coin)
	if !ok {
		return nil, fmt.Errorf("unknown coin: %s", u.coin)
	}

	// Create action
	return updateLeverageToAction(u, assetId), nil
}

type updateLeverageAction struct {
	Type     string `json:"type"`
	Asset    int64  `json:"asset"`
	IsCross  bool   `json:"isCross"`
	Leverage int64  `json:"leverage"`
}

func (u updateLeverageAction) getType() string {
	return u.Type
}

// updateLeverageToAction converts an UpdateLeverageRequest to an
// updateLeverageAction
func updateLeverageToAction(
	u updateLeverageRequest,
	assetId int64,
) updateLeverageAction {
	return updateLeverageAction{
		Type:     "updateLeverage",
		Asset:    assetId,
		IsCross:  u.isCross.OrElse(true),
		Leverage: u.leverage,
	}
}

// ============================================================================
// Update Isolated Margin Request
// ============================================================================

type updateIsolatedMarginRequest struct {
	coin   string
	amount float64
}

// UpdateIsolatedMarginRequest creates a new update isolated margin request
func UpdateIsolatedMarginRequest(
	coin string,
	amount float64,
) updateIsolatedMarginRequest {
	return updateIsolatedMarginRequest{
		coin:   coin,
		amount: amount,
	}
}

// toAction converts an updateIsolatedMarginRequest to an updateIsolatedMarginAction
func (u updateIsolatedMarginRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Convert amount to USD int
	intAmount, err := utils.FloatToUsdInt(u.amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount to USD: %w", err)
	}

	// Get asset for this coin
	asset, ok := e.info.NameToAsset(u.coin)
	if !ok {
		return nil, fmt.Errorf("unknown asset for name: %s", u.coin)
	}

	// Create action
	return updateIsolatedMarginToAction(asset, intAmount), nil
}

type updateIsolatedMarginAction struct {
	Type  string `json:"type"`
	Asset int64  `json:"asset"`
	IsBuy bool   `json:"isBuy"`
	Ntli  int64  `json:"ntli"`
}

func (u updateIsolatedMarginAction) getType() string {
	return u.Type
}

// updateIsolatedMarginToAction converts an UpdateIsolatedMarginRequest to an
// updateIsolatedMarginAction
func updateIsolatedMarginToAction(
	assetId int64,
	ntli int64,
) updateIsolatedMarginAction {
	return updateIsolatedMarginAction{
		Type:  "updateIsolatedMargin",
		Asset: assetId,
		IsBuy: true,
		Ntli:  ntli,
	}
}

// ============================================================================
// Schedule Cancel Request
// ============================================================================

type scheduleCancelRequest struct {
	time mo.Option[time.Time]
}

func ScheduleCancelRequest(t *time.Time) scheduleCancelRequest {
	var optT mo.Option[time.Time]
	if t == nil {
		optT = mo.None[time.Time]()
	} else {
		optT = mo.Some(*t)
	}

	return scheduleCancelRequest{
		time: optT,
	}
}

// toAction converts a scheduleCancelRequest to a scheduleCancelAction
func (s scheduleCancelRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	return scheduleCancelToAction(s), nil
}

type scheduleCancelAction struct {
	Type string `json:"type"`
	Time *int64 `json:"time,omitempty"`
}

func (s scheduleCancelAction) getType() string {
	return s.Type
}

func scheduleCancelToAction(s scheduleCancelRequest) scheduleCancelAction {
	t := optionMap(s.time, func(value time.Time) int64 {
		return value.UnixMilli()
	})

	return scheduleCancelAction{
		Type: "scheduleCancel",
		Time: t.ToPointer(),
	}
}

// ============================================================================
// Set Referrer Request
// ============================================================================

type setReferrerRequest struct {
	code string
}

// SetReferrerRequest creates a new set referrer request
func SetReferrerRequest(code string) setReferrerRequest {
	return setReferrerRequest{
		code: code,
	}
}

// toAction converts a setReferrerRequest to a setReferrerAction
func (s setReferrerRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	return setReferrerAction{
		Type: "setReferrer",
		Code: s.code,
	}, nil
}

type setReferrerAction struct {
	Type string `json:"type"`
	Code string `json:"code"`
}

func (s setReferrerAction) getType() string {
	return s.Type
}

// ============================================================================
// Create Sub Account Request
// ============================================================================

type createSubAccountRequest struct {
	name string
}

// CreateSubAccountRequest creates a new create sub account request
func CreateSubAccountRequest(name string) createSubAccountRequest {
	return createSubAccountRequest{
		name: name,
	}
}

// toAction converts a createSubAccountRequest to a createSubAccountAction
func (c createSubAccountRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	return createSubAccountToAction(c.name), nil
}

type createSubAccountAction struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func (c createSubAccountAction) getType() string {
	return c.Type
}

func createSubAccountToAction(n string) createSubAccountAction {
	return createSubAccountAction{
		Type: "createSubAccount",
		Name: n,
	}
}

// ============================================================================
// USD Class Transfer Request
// ============================================================================

type usdClassTransferRequest struct {
	amount float64
	toPerp bool
}

// UsdClassTransferRequest creates a new USD class transfer request
func UsdClassTransferRequest(amount float64, toPerp bool) usdClassTransferRequest {
	return usdClassTransferRequest{
		amount: amount,
		toPerp: toPerp,
	}
}

// toAction converts a usdClassTransferRequest to a usdClassTransferAction
// Note: This requires timestamp (int64) in opts
func (u usdClassTransferRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract timestamp from opts
	var timestamp int64
	for _, opt := range opts {
		if ts, ok := opt.(int64); ok {
			timestamp = ts
			break
		}
	}

	if timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required in opts for usdClassTransferRequest")
	}

	// Convert amount to wire format
	strAmount, err := utils.FloatToWire(u.amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount to wire format: %w", err)
	}

	// Add vault address if present
	if v, ok := e.vaultAddress.Get(); ok {
		strAmount += fmt.Sprintf(" subaccount:%s", v.String())
	}

	return usdClassTransferAction{
		Type:             "usdClassTransfer",
		Amount:           strAmount,
		ToPerp:           u.toPerp,
		Nonce:            timestamp,
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

type usdClassTransferAction struct {
	Type             string `json:"type"`
	Amount           string `json:"amount"`
	ToPerp           bool   `json:"toPerp"`
	Nonce            int64  `json:"nonce"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (u usdClassTransferAction) getType() string {
	return u.Type
}

// ============================================================================
// USD Transfer Request
// ============================================================================

type usdTransferRequest struct {
	amount      float64
	destination common.Address
}

// UsdTransferRequest creates a new USD transfer request
func UsdTransferRequest(amount float64, destination common.Address) usdTransferRequest {
	return usdTransferRequest{
		amount:      amount,
		destination: destination,
	}
}

// toAction converts a usdTransferRequest to a usdTransferAction
// Note: This requires timestamp (int64) in opts
func (u usdTransferRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract timestamp from opts
	var timestamp int64
	for _, opt := range opts {
		if ts, ok := opt.(int64); ok {
			timestamp = ts
			break
		}
	}

	if timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required in opts for usdTransferRequest")
	}

	// Convert amount to wire format
	strAmount, err := utils.FloatToWire(u.amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount to wire format: %w", err)
	}

	return usdTransferAction{
		Type:             "usdSend",
		Amount:           strAmount,
		Destination:      strings.ToLower(u.destination.Hex()),
		Time:             timestamp,
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

type usdTransferAction struct {
	Type             string `json:"type"`
	Amount           string `json:"amount"`
	Destination      string `json:"destination"`
	Time             int64  `json:"time"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (u usdTransferAction) getType() string {
	return u.Type
}

// ============================================================================
// Send Asset Request
// ============================================================================

type sendAssetRequest struct {
	destination    common.Address
	sourceDex      string
	destinationDex string
	token          string
	amount         float64
}

// SendAssetRequest creates a new send asset request
func SendAssetRequest(
	destination common.Address,
	sourceDex string,
	destinationDex string,
	token string,
	amount float64,
) sendAssetRequest {
	return sendAssetRequest{
		destination:    destination,
		sourceDex:      sourceDex,
		destinationDex: destinationDex,
		token:          token,
		amount:         amount,
	}
}

// toAction converts a sendAssetRequest to a sendAssetAction
func (s sendAssetRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Convert amount to wire format
	amountStr, err := utils.FloatToWire(s.amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount: %w", err)
	}

	// Get vault address if present
	fromSubAccount := ""
	if v, ok := e.vaultAddress.Get(); ok {
		fromSubAccount = v.Hex()
	}

	return sendAssetAction{
		Type:             "sendAsset",
		Destination:      s.destination.Hex(),
		SourceDex:        s.sourceDex,
		DestinationDex:   s.destinationDex,
		Token:            s.token,
		Amount:           amountStr,
		FromSubAccount:   fromSubAccount,
		Nonce:            0, // Will be set by Exchange
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

type sendAssetAction struct {
	Type             string `json:"type"`
	Destination      string `json:"destination"`
	SourceDex        string `json:"sourceDex"`
	DestinationDex   string `json:"destinationDex"`
	Token            string `json:"token"`
	Amount           string `json:"amount"`
	FromSubAccount   string `json:"fromSubAccount"`
	Nonce            int64  `json:"nonce"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (s sendAssetAction) getType() string {
	return s.Type
}

// ============================================================================
// Sub Account Transfer Request
// ============================================================================

type subAccountTransferRequest struct {
	subAccount common.Address
	isDeposit  bool
	usd        int64
}

// SubAccountTransferRequest creates a new sub account transfer request
func SubAccountTransferRequest(subAccount common.Address, isDeposit bool, usd int64) subAccountTransferRequest {
	return subAccountTransferRequest{
		subAccount: subAccount,
		isDeposit:  isDeposit,
		usd:        usd,
	}
}

// toAction converts a subAccountTransferRequest to a subAccountTransferAction
func (s subAccountTransferRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	return subAccountTransferAction{
		Type:           "subAccountTransfer",
		SubAccountUser: strings.ToLower(s.subAccount.Hex()),
		IsDeposit:      s.isDeposit,
		Usd:            s.usd,
	}, nil
}

type subAccountTransferAction struct {
	Type           string `json:"type"`
	SubAccountUser string `json:"subAccountUser"`
	IsDeposit      bool   `json:"isDeposit"`
	Usd            int64  `json:"usd"`
}

func (s subAccountTransferAction) getType() string {
	return s.Type
}

// ============================================================================
// Sub Account Spot Transfer Request
// ============================================================================

type subAccountSpotTransferRequest struct {
	subAccountUser common.Address
	isDeposit      bool
	token          string
	amount         float64
}

// SubAccountSpotTransferRequest creates a new sub account spot transfer request
func SubAccountSpotTransferRequest(subAccountUser common.Address, isDeposit bool, token string, amount float64) subAccountSpotTransferRequest {
	return subAccountSpotTransferRequest{
		subAccountUser: subAccountUser,
		isDeposit:      isDeposit,
		token:          token,
		amount:         amount,
	}
}

// toAction converts a subAccountSpotTransferRequest to a subAccountSpotTransferAction
func (s subAccountSpotTransferRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Convert amount to wire format
	strAmount, err := utils.FloatToWire(s.amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount to wire format: %w", err)
	}

	return subAccountSpotTransferAction{
		Type:           "subAccountSpotTransfer",
		SubAccountUser: strings.ToLower(s.subAccountUser.Hex()),
		IsDeposit:      s.isDeposit,
		Token:          s.token,
		Amount:         strAmount,
	}, nil
}

type subAccountSpotTransferAction struct {
	Type           string `json:"type"`
	SubAccountUser string `json:"subAccountUser"`
	IsDeposit      bool   `json:"isDeposit"`
	Token          string `json:"token"`
	Amount         string `json:"amount"`
}

func (s subAccountSpotTransferAction) getType() string {
	return s.Type
}

// ============================================================================
// Vault Transfer Request
// ============================================================================

type vaultTransferRequest struct {
	vaultAddress common.Address
	isDeposit    bool
	usd          int64
}

// VaultTransferRequest creates a new vault transfer request
func VaultTransferRequest(vaultAddress common.Address, isDeposit bool, usd int64) vaultTransferRequest {
	return vaultTransferRequest{
		vaultAddress: vaultAddress,
		isDeposit:    isDeposit,
		usd:          usd,
	}
}

// toAction converts a vaultTransferRequest to a vaultTransferAction
func (v vaultTransferRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	return vaultTransferAction{
		Type:         "vaultTransfer",
		VaultAddress: strings.ToLower(v.vaultAddress.Hex()),
		IsDeposit:    v.isDeposit,
		Usd:          v.usd,
	}, nil
}

type vaultTransferAction struct {
	Type         string `json:"type"`
	VaultAddress string `json:"vaultAddress"`
	IsDeposit    bool   `json:"isDeposit"`
	Usd          int64  `json:"usd"`
}

func (v vaultTransferAction) getType() string {
	return v.Type
}

// ============================================================================
// Spot Transfer Request
// ============================================================================

type spotTransferRequest struct {
	amount      float64
	destination common.Address
	token       string
}

// SpotTransferRequest creates a new spot transfer request
func SpotTransferRequest(amount float64, destination common.Address, token string) spotTransferRequest {
	return spotTransferRequest{
		amount:      amount,
		destination: destination,
		token:       token,
	}
}

// toAction converts a spotTransferRequest to a spotTransferAction
// Note: This requires timestamp (int64) in opts
func (s spotTransferRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract timestamp from opts
	var timestamp int64
	for _, opt := range opts {
		if ts, ok := opt.(int64); ok {
			timestamp = ts
			break
		}
	}

	if timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required in opts for spotTransferRequest")
	}

	// Convert amount to wire format
	strAmount, err := utils.FloatToWire(s.amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount to wire format: %w", err)
	}

	return spotTransferAction{
		Type:             "spotSend",
		Destination:      strings.ToLower(s.destination.Hex()),
		Token:            s.token,
		Amount:           strAmount,
		Time:             timestamp,
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

type spotTransferAction struct {
	Type             string `json:"type"`
	Destination      string `json:"destination"`
	Token            string `json:"token"`
	Amount           string `json:"amount"`
	Time             int64  `json:"time"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (s spotTransferAction) getType() string {
	return s.Type
}

// ============================================================================
// Token Delegate Request
// ============================================================================

type tokenDelegateRequest struct {
	validator    common.Address
	wei          int64
	isUndelegate bool
}

// TokenDelegateRequest creates a new token delegate request
func TokenDelegateRequest(validator common.Address, wei int64, isUndelegate bool) tokenDelegateRequest {
	return tokenDelegateRequest{
		validator:    validator,
		wei:          wei,
		isUndelegate: isUndelegate,
	}
}

// toAction converts a tokenDelegateRequest to a tokenDelegateAction
// Note: This requires timestamp (int64) in opts
func (t tokenDelegateRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract timestamp from opts
	var timestamp int64
	for _, opt := range opts {
		if ts, ok := opt.(int64); ok {
			timestamp = ts
			break
		}
	}

	if timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required in opts for tokenDelegateRequest")
	}

	return tokenDelegateAction{
		Type:             "tokenDelegate",
		Validator:        strings.ToLower(t.validator.Hex()),
		Wei:              t.wei,
		IsUndelegate:     t.isUndelegate,
		Nonce:            timestamp,
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

type tokenDelegateAction struct {
	Type             string `json:"type"`
	Validator        string `json:"validator"`
	Wei              int64  `json:"wei"`
	IsUndelegate     bool   `json:"isUndelegate"`
	Nonce            int64  `json:"nonce"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (t tokenDelegateAction) getType() string {
	return t.Type
}

// ============================================================================
// Withdraw From Bridge Request
// ============================================================================

type withdrawFromBridgeRequest struct {
	amount      float64
	destination common.Address
}

func WithdrawFromBridgeRequest(
	amount float64,
	destination common.Address,
) withdrawFromBridgeRequest {
	return withdrawFromBridgeRequest{
		amount:      amount,
		destination: destination,
	}
}

// toAction converts a withdrawFromBridgeRequest to a withdrawFromBridgeAction
// Note: This requires timestamp (int64) in opts
func (w withdrawFromBridgeRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract timestamp from opts
	var timestamp int64
	for _, opt := range opts {
		if ts, ok := opt.(int64); ok {
			timestamp = ts
			break
		}
	}

	if timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required in opts for withdrawFromBridgeRequest")
	}

	strAmount, err := utils.FloatToWire(w.amount)
	if err != nil {
		return nil, fmt.Errorf("failed to convert amount to wire format: %w", err)
	}

	return withdrawFromBridgeAction{
		Type:             "withdraw3",
		Destination:      strings.ToLower(w.destination.Hex()),
		Amount:           strAmount,
		Time:             timestamp,
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

type withdrawFromBridgeAction struct {
	Type             string `json:"type"`
	Destination      string `json:"destination"`
	Amount           string `json:"amount"`
	Time             int64  `json:"time"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (w withdrawFromBridgeAction) getType() string {
	return w.Type
}

// ============================================================================
// Approve Agent Request
// ============================================================================

type approveAgentRequest struct {
	agentName mo.Option[string]
}

type approveAgentOption func(*approveAgentConfig)

type approveAgentConfig struct {
	agentName mo.Option[string]
}

func ApproveAgentRequest(opts ...approveAgentOption) approveAgentRequest {
	cfg := approveAgentConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return approveAgentRequest{
		agentName: cfg.agentName,
	}
}

func WithAgentName(name string) approveAgentOption {
	return func(cfg *approveAgentConfig) {
		cfg.agentName = mo.Some(name)
	}
}

// toAction converts an approveAgentRequest to an approveAgentAction
// Note: This requires agentPrivateKey (*ecdsa.PrivateKey) and timestamp (int64) in opts
func (a approveAgentRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract the agent private key and timestamp from opts
	var agentPrivateKey *ecdsa.PrivateKey
	var timestamp int64

	for _, opt := range opts {
		switch v := opt.(type) {
		case *ecdsa.PrivateKey:
			agentPrivateKey = v
		case int64:
			timestamp = v
		}
	}

	if agentPrivateKey == nil {
		return nil, fmt.Errorf("agent private key is required in opts for approveAgentRequest")
	}

	if timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required in opts for approveAgentRequest")
	}

	// Derive agent address from the key
	agentAddress := crypto.PubkeyToAddress(agentPrivateKey.PublicKey)

	// Extract agent name if provided
	agentName := ""
	if name, ok := a.agentName.Get(); ok {
		agentName = name
	}

	// Create action
	return approveAgentAction{
		Type:             "approveAgent",
		AgentAddress:     strings.ToLower(agentAddress.Hex()),
		AgentName:        agentName,
		Nonce:            timestamp,
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

type approveAgentAction struct {
	Type             string `json:"type"`
	AgentAddress     string `json:"agentAddress"`
	AgentName        string `json:"agentName"`
	Nonce            int64  `json:"nonce"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (a approveAgentAction) getType() string {
	return a.Type
}

// ============================================================================
// Approve Builder Fee Request
// ============================================================================

type approveBuilderFeeRequest struct {
	builder    common.Address
	maxFeeRate string
}

func ApproveBuilderFeeRequest(
	builder common.Address,
	maxFeeRate string,
) approveBuilderFeeRequest {
	return approveBuilderFeeRequest{
		builder:    builder,
		maxFeeRate: maxFeeRate,
	}
}

// toAction converts an approveBuilderFeeRequest to an approveBuilderFeeAction
// Note: This requires timestamp (int64) in opts
func (a approveBuilderFeeRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract timestamp from opts
	var timestamp int64
	for _, opt := range opts {
		if ts, ok := opt.(int64); ok {
			timestamp = ts
			break
		}
	}

	if timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required in opts for approveBuilderFeeRequest")
	}

	return approveBuilderFeeAction{
		Type:             "approveBuilderFee",
		MaxFeeRate:       a.maxFeeRate,
		Builder:          strings.ToLower(a.builder.Hex()),
		Nonce:            timestamp,
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

// ============================================================================
// Approve Builder Fee Action
// ============================================================================

type approveBuilderFeeAction struct {
	Type             string `json:"type"`
	MaxFeeRate       string `json:"maxFeeRate"`
	Builder          string `json:"builder"`
	Nonce            int64  `json:"nonce"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (a approveBuilderFeeAction) getType() string {
	return a.Type
}

// ============================================================================
// Convert To Multi Sig User Request
// ============================================================================

type convertToMultiSigUserRequest struct {
	authorizedUsers []common.Address
	threshold       int64
}

func ConvertToMultiSigUserRequest(
	authorizedUsers []common.Address,
	threshold int64,
) convertToMultiSigUserRequest {
	return convertToMultiSigUserRequest{
		authorizedUsers: authorizedUsers,
		threshold:       threshold,
	}
}

// toAction converts a convertToMultiSigUserRequest to a convertToMultiSigUserAction
// Note: This requires timestamp (int64) in opts
func (c convertToMultiSigUserRequest) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Extract timestamp from opts
	var timestamp int64
	for _, opt := range opts {
		if ts, ok := opt.(int64); ok {
			timestamp = ts
			break
		}
	}

	if timestamp == 0 {
		return nil, fmt.Errorf("timestamp is required in opts for convertToMultiSigUserRequest")
	}

	// Sort authorized users
	sortedUsers := make([]common.Address, len(c.authorizedUsers))
	copy(sortedUsers, c.authorizedUsers)
	slices.SortFunc(
		sortedUsers,
		func(a, z common.Address) int {
			return a.Cmp(z)
		},
	)

	// Create signers JSON
	signers := map[string]any{
		"authorizedUsers": sortedUsers,
		"threshold":       c.threshold,
	}
	signersJSON, err := json.Marshal(signers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signers: %w", err)
	}

	// Create action
	return convertToMultiSigUserAction{
		Type:             "convertToMultiSigUser",
		Signers:          string(signersJSON),
		Nonce:            timestamp,
		SignatureChainId: getSignatureChainId(),
		HyperliquidChain: e.rest.NetworkName(),
	}, nil
}

// ============================================================================
// Convert To Multi Sig User Action
// ============================================================================

type convertToMultiSigUserAction struct {
	Type             string `json:"type"`
	Signers          string `json:"signers"`
	Nonce            int64  `json:"nonce"`
	SignatureChainId string `json:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain"`
}

func (a convertToMultiSigUserAction) getType() string {
	return a.Type
}

// ============================================================================
// Multi Sig Request
// ============================================================================

type multiSigRequest[T request] struct {
	multiSigUser  common.Address
	innerRequest  T
	signatures    []signature
	nonce         int64
	vaultAddress  mo.Option[common.Address]
}

type multiSigOption[T request] func(*multiSigConfig[T])

type multiSigConfig[T request] struct {
	multiSigUser common.Address
	innerRequest T
	signatures   []signature
	nonce        int64
	vaultAddress mo.Option[common.Address]
}

func MultiSigRequest[T request](
	multiSigUser common.Address,
	innerRequest T,
	signatures []signature,
	nonce int64,
	opts ...multiSigOption[T],
) multiSigRequest[T] {
	cfg := multiSigConfig[T]{
		multiSigUser: multiSigUser,
		innerRequest: innerRequest,
		signatures:   signatures,
		nonce:        nonce,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return multiSigRequest[T]{
		multiSigUser: cfg.multiSigUser,
		innerRequest: cfg.innerRequest,
		signatures:   cfg.signatures,
		nonce:        cfg.nonce,
		vaultAddress: cfg.vaultAddress,
	}
}

func WithMultiSigVaultAddress[T request](vaultAddress common.Address) multiSigOption[T] {
	return func(cfg *multiSigConfig[T]) {
		cfg.vaultAddress = mo.Some(vaultAddress)
	}
}

// toAction converts a multiSigRequest to a multiSigAction
func (m multiSigRequest[T]) toAction(
	ctx context.Context,
	e *Exchange,
	opts ...any,
) (action, error) {
	// Get wallet address
	walletAddress := crypto.PubkeyToAddress(e.privateKey.PublicKey)

	// Convert inner request to action
	innerAction, err := m.innerRequest.toAction(ctx, e, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to convert inner request to action: %w", err)
	}

	// Create the multiSigAction
	return multiSigAction{
		Type:             "multiSig",
		SignatureChainId: getSignatureChainId(),
		Signatures:       m.signatures,
		Payload: multiSigPayload{
			MultiSigUser: strings.ToLower(m.multiSigUser.Hex()),
			OuterSigner:  strings.ToLower(walletAddress.Hex()),
			Action:       innerAction,
		},
	}, nil
}

// ============================================================================
// Multi Sig Action
// ============================================================================

type multiSigPayload struct {
	MultiSigUser string `json:"multiSigUser"`
	OuterSigner  string `json:"outerSigner"`
	Action       any    `json:"action"`
}

type multiSigAction struct {
	Type             string          `json:"type"`
	SignatureChainId string          `json:"signatureChainId"`
	Signatures       []signature     `json:"signatures"`
	Payload          multiSigPayload `json:"payload"`
}

func (a multiSigAction) getType() string {
	return a.Type
}

// ============================================================================
// Utility Functions
// ============================================================================

func optionMap[T, U any](o mo.Option[T], f func(T) U) mo.Option[U] {
	if v, ok := o.Get(); ok {
		return mo.Some(f(v))
	}
	return mo.None[U]()
}
