package exchange

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

type OrderRequest struct {
	Coin       string
	IsBuy      bool
	Sz         float64
	LimitPx    float64
	OrderType  OrderType
	ReduceOnly bool
	CLOID      *common.Hash
}

type orderWire struct {
	A int64         `json:"a"`
	B bool          `json:"b"`
	P string        `json:"p"`
	S string        `json:"s"`
	R bool          `json:"r"`
	T orderTypeWire `json:"t"`
	C *common.Hash  `json:"cloid,omitempty"`
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
	IsMarket  bool    `json:"isMarket"`
	TriggerPx float64 `json:"triggerPx"`
	TpSl      string  `json:"tpsl"`
}

type triggerOrderWire struct {
	IsMarket  bool   `json:"isMarket"`
	TriggerPx string `json:"triggerPx"`
	TpSl      string `json:"tpsl"`
}

type BuilderInfo struct {
	// Public address of the builder
	PublicAddress common.Address
	// Amount of the fee in tenths of basis points.
	// eg. 10 means 1 basis point
	FeeAmount int
}

type Oid int64
type Cloid common.Hash

// OidOrCloid represents either an order ID or a CLOID
type OidOrCloid any

// ModifyRequest represents a modify order request
type ModifyRequest struct {
	OID   OidOrCloid
	Order OrderRequest
}

// modifyWire represents a modify order in wire format
type modifyWire struct {
	OID   any       `json:"oid"`
	Order orderWire `json:"order"`
}

// CancelRequest represents a cancel order request
type CancelRequest struct {
	Coin string
	Oid  int
}

// cancelWire represents a cancel order in wire format
type cancelWire struct {
	AssetId int64 `json:"a"`
	Oid     int64 `json:"o"`
}

// NewCancelRequest creates a new cancel request with an order ID
func NewCancelRequest(coin string, oid int) CancelRequest {
	return CancelRequest{
		Coin: coin,
		Oid:  oid,
	}
}

// CancelRequestByCloid represents a cancel order request by CLOID
type CancelRequestByCloid struct {
	Coin  string
	Cloid common.Hash
}

// cancelByCloidWire represents a cancel order request by CLOID in wire format
type cancelByCloidWire struct {
	AssetId int64       `json:"asset"`
	Cloid   common.Hash `json:"cloid"`
}

// NewCancelRequestWithCloid creates a new cancel request with a CLOID
func NewCancelRequestByCloid(coin string, cloid common.Hash) CancelRequestByCloid {
	return CancelRequestByCloid{
		Coin:  coin,
		Cloid: cloid,
	}
}

// toOrderWire converts OrderRequest to OrderWire
func (o OrderRequest) toOrderWire(assetId int) (orderWire, error) {
	// Convert sizes and prices to wire format
	sizeStr, err := floatToWire(o.Sz)
	if err != nil {
		return orderWire{}, fmt.Errorf("failed to convert size: %w", err)
	}

	priceStr, err := floatToWire(o.LimitPx)
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
		C: o.CLOID,
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
		triggerPxStr, err := floatToWire(t.Trigger.TriggerPx)
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
func (c CancelRequest) toCancelWire(assetId int) cancelWire {
	return cancelWire{
		AssetId: int64(assetId),
		Oid:     int64(c.Oid),
	}
}

func (c CancelRequestByCloid) toCancelByCloidWire(assetId int) cancelByCloidWire {
	return cancelByCloidWire{
		AssetId: int64(assetId),
		Cloid:   c.Cloid,
	}
}

// ordersToAction converts a list of OrderWires to an order action
func ordersToAction(orders []orderWire, builder *BuilderInfo) map[string]any {
	action := map[string]any{
		"type":   "order",
		"orders": orders,
	}

	if builder != nil {
		action["grouping"] = map[string]any{
			"b": strings.ToLower(builder.PublicAddress.String()),
			"f": builder.FeeAmount,
		}
	}

	return action
}

// cancelsToAction converts a list of CancelRequests to a cancel action
func cancelsToAction(cancels []cancelWire) map[string]any {
	return map[string]any{
		"type":    "cancel",
		"cancels": cancels,
	}
}

// cancelsByCloidToAction converts a list of CancelRequestsByCloid to a cancelByCloid action
func cancelsByCloidToAction(cancels []cancelByCloidWire) map[string]any {
	return map[string]any{
		"type":    "cancelByCloid",
		"cancels": cancels,
	}
}

// modifiesToAction converts a list of ModifyWires to a batch modify action
func modifiesToAction(modifies []modifyWire) map[string]any {
	return map[string]any{
		"type":     "batchModify",
		"modifies": modifies,
	}
}
