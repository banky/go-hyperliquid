package exchange

import (
	"fmt"

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

type OrderWire struct {
	A int64         `json:"a"`
	B bool          `json:"b"`
	P string        `json:"p"`
	S string        `json:"s"`
	R bool          `json:"r"`
	T OrderTypeWire `json:"t"`
	C *common.Hash  `json:"cloid,omitempty"`
}

type OrderType struct {
	Limit   *LimitOrder
	Trigger *TriggerOrder
}

type OrderTypeWire struct {
	Limit   *LimitOrder       `json:"limit,omitempty"`
	Trigger *TriggerOrderWire `json:"trigger,omitempty"`
}

type LimitOrder struct {
	Tif string `json:"tif"`
}

type TriggerOrder struct {
	IsMarket  bool    `json:"isMarket"`
	TriggerPx float64 `json:"triggerPx"`
	TpSl      string  `json:"tpsl"`
}

type TriggerOrderWire struct {
	IsMarket  bool   `json:"isMarket"`
	TriggerPx string `json:"triggerPx"`
	TpSl      string `json:"tpsl"`
}

type BuilderInfo struct {
	// Public address of the builder
	B string `json:"b"`
	// Amount of the fee in tenths of basis points.
	// eg. 10 means 1 basis point
	F int `json:"f"`
}

// CancelRequest represents a cancel order request
type CancelRequest struct {
	Coin string
	OID  int64
}

// CancelWire represents a cancel order in wire format
type CancelWire struct {
	A int64 `json:"a"` // asset ID
	O int64 `json:"o"` // order ID
}

// toOrderWire converts OrderRequest to OrderWire
func (o OrderRequest) toOrderWire(assetId int) (OrderWire, error) {
	// Convert sizes and prices to wire format
	sizeStr, err := floatToWire(o.Sz)
	if err != nil {
		return OrderWire{}, fmt.Errorf("failed to convert size: %w", err)
	}

	priceStr, err := floatToWire(o.LimitPx)
	if err != nil {
		return OrderWire{}, fmt.Errorf("failed to convert limit price: %w", err)
	}

	// Convert order type to wire format
	orderTypeWire, err := o.OrderType.toOrderTypeWire()
	if err != nil {
		return OrderWire{}, fmt.Errorf("failed to convert order type: %w", err)
	}

	return OrderWire{
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
func (t OrderType) toOrderTypeWire() (OrderTypeWire, error) {
	wire := OrderTypeWire{}

	if t.Limit != nil {
		wire.Limit = &LimitOrder{
			Tif: t.Limit.Tif,
		}
	}

	if t.Trigger != nil {
		// Convert to wire format
		triggerPxStr, err := floatToWire(t.Trigger.TriggerPx)
		if err != nil {
			return OrderTypeWire{}, fmt.Errorf("failed to convert trigger price: %w", err)
		}

		wire.Trigger = &TriggerOrderWire{
			IsMarket:  t.Trigger.IsMarket,
			TriggerPx: triggerPxStr,
			TpSl:      t.Trigger.TpSl,
		}
	}

	return wire, nil
}

// toCancelWire converts CancelRequest to CancelWire
func (c CancelRequest) toCancelWire(assetId int) CancelWire {
	return CancelWire{
		A: int64(assetId),
		O: c.OID,
	}
}

// ordersToAction converts a list of OrderWires to an order action
func ordersToAction(orders []OrderWire, builder *BuilderInfo) map[string]any {
	action := map[string]any{
		"type":   "order",
		"orders": orders,
	}

	if builder != nil {
		action["grouping"] = map[string]any{
			"b": builder.B,
			"f": builder.F,
		}
	}

	return action
}

// cancelsToAction converts a list of CancelRequests to a cancel action
func cancelsToAction(cancels []CancelWire) map[string]any {
	return map[string]any{
		"type":    "cancel",
		"cancels": cancels,
	}
}
