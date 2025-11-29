package exchange

import (
	"fmt"
	"math/big"

	"github.com/banky/go-hyperliquid/internal/utils"
	"github.com/banky/go-hyperliquid/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/mo"
)

type orderRequest struct {
	Coin       string
	IsBuy      bool
	Sz         float64
	LimitPx    float64
	OrderType  OrderType
	ReduceOnly bool
	Cloid      mo.Option[types.Cloid]
}

type NewOrderRequestOption func(*newOrderRequestConfig)

type newOrderRequestConfig struct {
	reduceOnly   bool
	cloid        mo.Option[types.Cloid]
	limitOrder   mo.Option[LimitOrder]
	triggerOrder mo.Option[TriggerOrder]
}

func NewOrderRequest(
	coin string,
	isBuy bool,
	sz float64,
	limitPx float64,
	opts ...NewOrderRequestOption,
) orderRequest {
	cfg := newOrderRequestConfig{}
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
		Coin:       coin,
		IsBuy:      isBuy,
		Sz:         sz,
		LimitPx:    limitPx,
		OrderType:  orderType,
		ReduceOnly: cfg.reduceOnly,
		Cloid:      cfg.cloid,
	}
}

// WithReduceOnly sets the reduce-only flag
func WithReduceOnly(reduceOnly bool) NewOrderRequestOption {
	return func(cfg *newOrderRequestConfig) {
		cfg.reduceOnly = reduceOnly
	}
}

// WithCloid sets the client order ID
func WithCloid(c types.Cloid) NewOrderRequestOption {
	return func(cfg *newOrderRequestConfig) {
		cfg.cloid = mo.Some(c)
	}
}

func WithLimitOrder(limitOrder LimitOrder) NewOrderRequestOption {
	return func(cfg *newOrderRequestConfig) {
		cfg.limitOrder = mo.Some(limitOrder)
	}
}

func WithTriggerOrder(triggerOrder TriggerOrder) NewOrderRequestOption {
	return func(cfg *newOrderRequestConfig) {
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

type OrderType struct {
	Limit   *LimitOrder
	Trigger *TriggerOrder
}

type orderTypeWire struct {
	Limit   *LimitOrder       `json:"limit,omitempty"`
	Trigger *triggerOrderWire `json:"trigger,omitempty"`
}

type LimitOrder struct {
	Tif string `json:"tif"`
}

type TriggerOrder struct {
	IsMarket  bool
	TriggerPx float64
	TpSl      string
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

type Oid int64

// OidOrCloid represents either an order ID or a CLOID
type OidOrCloid any

// UserWeiPair represents a user address and wei amount
type UserWeiPair struct {
	User common.Address
	Wei  *big.Int
}

// TokenWeiPair represents a token ID and wei amount
type TokenWeiPair struct {
	Token int64
	Wei   *big.Int
}

// modifyRequest represents a modify order request
type modifyRequest struct {
	Oid   mo.Option[int64]
	Cloid mo.Option[types.Cloid]
	Order orderRequest
}

type ModifyRequestOption func(*ModifyRequestConfig)

type ModifyRequestConfig struct {
	oid   mo.Option[int64]
	cloid mo.Option[types.Cloid]
}

// NewModifyRequest creates a new modify order request
func NewModifyRequest(
	order orderRequest,
	opts ...ModifyRequestOption,
) modifyRequest {
	cfg := ModifyRequestConfig{}
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

func WithModifyOrderId(id int64) ModifyRequestOption {
	return func(mrc *ModifyRequestConfig) {
		mrc.oid = mo.Some(id)
	}
}

func WithModifyCloid(c types.Cloid) ModifyRequestOption {
	return func(mrc *ModifyRequestConfig) {
		mrc.cloid = mo.Some(c)
	}
}

// func

// modifyWire represents a modify order in wire format
type modifyWire struct {
	Oid   any       `json:"oid"`
	Order orderWire `json:"order"`
}

// CancelRequest represents a cancel order request
type CancelRequest struct {
	Coin string
	Oid  int64
}

// cancelWire represents a cancel order in wire format
type cancelWire struct {
	AssetId int64 `json:"a"`
	Oid     int64 `json:"o"`
}

// NewCancelRequest creates a new cancel request with an order ID
func NewCancelRequest(coin string, oid int64) CancelRequest {
	return CancelRequest{
		Coin: coin,
		Oid:  oid,
	}
}

// CancelRequestByCloid represents a cancel order request by CLOID
type CancelRequestByCloid struct {
	Coin  string
	Cloid types.Cloid
}

// cancelByCloidWire represents a cancel order request by CLOID in wire format
type cancelByCloidWire struct {
	AssetId int64       `json:"asset"`
	Cloid   types.Cloid `json:"cloid"`
}

// NewCancelRequestWithCloid creates a new cancel request with a CLOID
func NewCancelRequestByCloid(
	coin string,
	cloid types.Cloid,
) CancelRequestByCloid {
	return CancelRequestByCloid{
		Coin:  coin,
		Cloid: cloid,
	}
}

// toOrderWire converts OrderRequest to OrderWire
func (o orderRequest) toOrderWire(assetId int64) (orderWire, error) {
	// Convert sizes and prices to wire format
	sizeStr, err := utils.FloatToWire(o.Sz)
	if err != nil {
		return orderWire{}, fmt.Errorf("failed to convert size: %w", err)
	}

	priceStr, err := utils.FloatToWire(o.LimitPx)
	if err != nil {
		return orderWire{}, fmt.Errorf("failed to convert limit price: %w", err)
	}

	// Convert order type to wire format
	orderTypeWire, err := o.OrderType.toOrderTypeWire()
	if err != nil {
		return orderWire{}, fmt.Errorf("failed to convert order type: %w", err)
	}

	return orderWire{
		A: int64(assetId),
		B: o.IsBuy,
		P: priceStr,
		S: sizeStr,
		R: o.ReduceOnly,
		T: orderTypeWire,
		C: o.Cloid.ToPointer(),
	}, nil
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

// toCancelWire converts CancelRequest to CancelWire
func (c CancelRequest) toCancelWire(assetId int64) cancelWire {
	return cancelWire{
		AssetId: int64(assetId),
		Oid:     int64(c.Oid),
	}
}

func (c CancelRequestByCloid) toCancelByCloidWire(
	assetId int64,
) cancelByCloidWire {
	return cancelByCloidWire{
		AssetId: int64(assetId),
		Cloid:   c.Cloid,
	}
}

type orderActionWire struct {
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

func (o orderActionWire) getType() string {
	return o.Type
}

func ordersToAction(
	orders []orderWire,
	builder mo.Option[BuilderInfo],
	grouping mo.Option[OrderGrouping],
) orderActionWire {
	action := orderActionWire{
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

type cancelActionWire struct {
	Type    string       `json:"type"`
	Cancels []cancelWire `json:"cancels"`
}

func (c cancelActionWire) getType() string {
	return c.Type
}

// cancelsToAction converts a list of CancelRequests to a cancel action
func cancelsToAction(cancels []cancelWire) cancelActionWire {
	return cancelActionWire{
		Type:    "cancel",
		Cancels: cancels,
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
