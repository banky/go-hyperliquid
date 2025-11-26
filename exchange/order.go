package exchange

import (
	"github.com/ethereum/go-ethereum/common"
)

type OrderRequest struct {
	coin       string
	isBuy      bool
	sz         float64
	limitPx    float64
	orderType  OrderType
	reduceOnly bool
	cloid      *common.Hash
}

type OrderWire struct {
	A int          `json:"a"`
	B bool         `json:"b"`
	P string       `json:"p"`
	S string       `json:"s"`
	R bool         `json:"r"`
	T OrderType    `json:"t"`
	C *common.Hash `json:"cloid,omitempty"`
}

type OrderType struct {
	Limit   *LimitOrder   `json:"limit,omitempty"`
	Trigger *TriggerOrder `json:"trigger,omitempty"`
}

type LimitOrder struct {
	Tif string `json:"tif"`
}

type TriggerOrder struct {
	IsMarket  bool   `json:"isMarket"`
	TriggerPx string `json:"triggerPx"`
	TpSl      string `json:"tpsl"`
}

type BuilderInfo struct {
	B string `json:"b"`
	F int    `json:"f"`
}
