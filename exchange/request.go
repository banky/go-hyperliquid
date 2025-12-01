package exchange

import (
	"fmt"
	"time"

	"github.com/banky/go-hyperliquid/internal/utils"
	"github.com/banky/go-hyperliquid/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/mo"
)

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

type usdClassTransferAction struct {
	Type string `json:"type"`
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
