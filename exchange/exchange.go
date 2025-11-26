package exchange

import (
	"crypto/ecdsa"

	"github.com/banky/go-hyperliquid/rest"
)

type Exchange struct {
	privateKey *ecdsa.PrivateKey
	client     rest.Client
}

// func postAction(

// ) {
// 	asdf := common.HexToAddress("")

// }

// func postAction(

// )

// func (e *Exchange) Order(
// 	name string,
// 	isBuy bool,
// 	sz float64,
// 	limitPx float64,
// 	orderType OrderType,
// 	reduceOnly bool,
// 	cloid *common.Hash,
// 	builder BuilderInfo,
// ) {
// 	order := OrderRequest{
// 		coin:       name,
// 		isBuy:      isBuy,
// 		sz:         sz,
// 		limitPx:    limitPx,
// 		orderType:  orderType,
// 		reduceOnly: reduceOnly,
// 		cloid:      cloid,
// 	}
// 	return e.BulkOrders([]OrderRequest{order}, builder)
// }

// func (e *Exchange) BulkOrders(
// 	orderRequests []OrderRequest,
// 	builder *BuilderInfo,
// ) {
// 	orderWires := make([]OrderWire, 0, len(orderRequests))
// 	for orderRequest := range orderRequests {

// 	}
// }
