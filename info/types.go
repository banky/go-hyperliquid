package info

import (
	"encoding/json"
	"strconv"

	"github.com/banky/go-hyperliquid/types"
	"github.com/ethereum/go-ethereum/common"
)

// ===== Market Data Types =====

// L2Level represents a single level in the order book
type L2Level struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int64  `json:"n"`
}

// L2BookSnapshot contains level 2 order book data
type L2BookSnapshot struct {
	Coin   string       `json:"coin"`
	Levels [2][]L2Level `json:"levels"`
	Time   int64        `json:"time"`
}

// AssetInfo contains metadata about an asset
type AssetInfo struct {
	Name       string `json:"name"`
	SzDecimals int64  `json:"szDecimals"`
}

// Meta contains exchange metadata for perpetuals
type Meta struct {
	Universe []AssetInfo `json:"universe"`
}

// SpotAssetInfo contains spot asset metadata
type SpotAssetInfo struct {
	Name        string   `json:"name"`
	Tokens      [2]int64 `json:"tokens"`
	Index       int64    `json:"index"`
	IsCanonical bool     `json:"isCanonical"`
}

// SpotTokenInfo contains spot token metadata
type SpotTokenInfo struct {
	Name        string      `json:"name"`
	SzDecimals  int64       `json:"szDecimals"`
	WeiDecimals int64       `json:"weiDecimals"`
	Index       int64       `json:"index"`
	TokenId     string      `json:"tokenId"`
	IsCanonical bool        `json:"isCanonical"`
	EvmContract EvmContract `json:"evmContract"`
	FullName    *string     `json:"fullName"`
}

type EvmContract struct {
	Address             common.Address `json:"address"`
	EvmExtraWeiDecimals int64          `json:"evm_extra_wei_decimals"`
}

// SpotMeta contains exchange metadata for spot trading
type SpotMeta struct {
	Universe []SpotAssetInfo `json:"universe"`
	Tokens   []SpotTokenInfo `json:"tokens"`
}

// ===== User Account Types =====

// Position represents a user's position in a coin
type Position struct {
	Coin           string   `json:"coin"`
	EntryPx        *string  `json:"entryPx"`
	Leverage       Leverage `json:"leverage"`
	LiquidationPx  *string  `json:"liquidationPx"`
	MarginUsed     string   `json:"marginUsed"`
	PositionValue  string   `json:"positionValue"`
	ReturnOnEquity string   `json:"returnOnEquity"`
	Szi            string   `json:"szi"`
	UnrealizedPnl  string   `json:"unrealizedPnl"`
}

// AssetPosition represents a user's position in an asset
type AssetPosition struct {
	Position Position `json:"position"`
	Type     string   `json:"type"`
}

// Leverage represents leverage configuration
type Leverage struct {
	Type   string  `json:"type"` // "cross" or "isolated"
	Value  int64   `json:"value"`
	RawUsd *string `json:"rawUsd,omitempty"` // Only for isolated
}

// MarginSummary contains margin information
type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalMarginUsed string `json:"totalMarginUsed"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
}

// UserState contains detailed trading information about a user
type UserState struct {
	AssetPositions     []AssetPosition `json:"assetPositions"`
	CrossMarginSummary MarginSummary   `json:"crossMarginSummary"`
	MarginSummary      MarginSummary   `json:"marginSummary"`
	Withdrawable       string          `json:"withdrawable"`
}

// OpenOrder represents an open order
type OpenOrder struct {
	Coin      string `json:"coin"`
	LimitPx   string `json:"limitPx"`
	Oid       int64  `json:"oid"`
	Side      string `json:"side"`
	Sz        string `json:"sz"`
	Timestamp int64  `json:"timestamp"`
}

// Fill represents a fill/executed trade
type Fill struct {
	Coin          string `json:"coin"`
	Px            string `json:"px"`
	Sz            string `json:"sz"`
	Side          string `json:"side"`
	Time          int64  `json:"time"`
	StartPosition string `json:"startPosition"`
	Dir           string `json:"dir"`
	ClosedPnl     string `json:"closedPnl"`
	Hash          string `json:"hash"`
	Oid           int64  `json:"oid"`
	Crossed       bool   `json:"crossed"`
	Fee           string `json:"fee"`
	Tid           int64  `json:"tid"`
	FeeToken      string `json:"feeToken"`
}

// FundingRecord represents a funding payment record
type FundingRecord struct {
	Coin        string `json:"coin"`
	FundingRate string `json:"fundingRate"`
	Premium     string `json:"premium"`
	Time        int64  `json:"time"`
}

// Candle represents candlestick data
type Candle struct {
	T int64  `json:"t"` // Timestamp
	O string `json:"o"` // Open
	C string `json:"c"` // Close
	H string `json:"h"` // High
	L string `json:"l"` // Low
	V string `json:"v"` // Volume
	N int64  `json:"n"` // Number of trades
	S string `json:"s"` // Symbol
	I string `json:"i"` // Interval
}

// ===== Order Status Types =====

// OrderStatus represents the status of an order
type OrderStatus string

const (
	// Open represents a successfully placed order
	OrderStatusOpen OrderStatus = "open"

	// Filled represents a fully filled order
	OrderStatusFilled OrderStatus = "filled"

	// Canceled represents an order canceled by the user
	OrderStatusCanceled OrderStatus = "canceled"

	// Triggered represents a trigger order that has been triggered
	OrderStatusTriggered OrderStatus = "triggered"

	// Rejected represents an order rejected at placement
	OrderStatusRejected OrderStatus = "rejected"

	// MarginCanceled represents an order canceled due to insufficient margin
	OrderStatusMarginCanceled OrderStatus = "marginCanceled"

	// VaultWithdrawalCanceled represents an order canceled due to vault
	// withdrawal (vaults only)
	OrderStatusVaultWithdrawalCanceled OrderStatus = "vaultWithdrawalCanceled"

	// OpenInterestCapCanceled represents an order canceled due to being too
	// aggressive when open interest was at cap
	OrderStatusOpenInterestCapCanceled OrderStatus = "openInterestCapCanceled"

	// SelfTradeCanceled represents an order canceled due to self-trade
	// prevention
	OrderStatusSelfTradeCanceled OrderStatus = "selfTradeCanceled"

	// ReduceOnlyCanceled represents a reduce-only order canceled because it
	// does not reduce position
	OrderStatusReduceOnlyCanceled OrderStatus = "reduceOnlyCanceled"

	// SiblingFilledCanceled represents a TP/SL order canceled due to sibling
	// order being filled
	OrderStatusSiblingFilledCanceled OrderStatus = "siblingFilledCanceled"

	// DelistedCanceled represents an order canceled due to asset delisting
	OrderStatusDelistedCanceled OrderStatus = "delistedCanceled"

	// LiquidatedCanceled represents an order canceled due to liquidation
	OrderStatusLiquidatedCanceled OrderStatus = "liquidatedCanceled"

	// ScheduledCancel represents an API-only order canceled due to exceeding
	// scheduled cancel deadline (dead man's switch)
	OrderStatusScheduledCancel OrderStatus = "scheduledCancel"

	// TickRejected represents an order rejected due to invalid tick price
	OrderStatusTickRejected OrderStatus = "tickRejected"

	// MinTradeNtlRejected represents an order rejected due to order notional
	// below minimum
	OrderStatusMinTradeNtlRejected OrderStatus = "minTradeNtlRejected"

	// PerpMarginRejected represents an order rejected due to insufficient
	// margin
	OrderStatusPerpMarginRejected OrderStatus = "perpMarginRejected"

	// ReduceOnlyRejected represents an order rejected due to reduce only
	// constraint
	OrderStatusReduceOnlyRejected OrderStatus = "reduceOnlyRejected"

	// BadAloPxRejected represents an order rejected due to post-only immediate
	// match
	OrderStatusBadAloPxRejected OrderStatus = "badAloPxRejected"

	// IocCancelRejected represents an order rejected due to IOC not able to
	// match
	OrderStatusIocCancelRejected OrderStatus = "iocCancelRejected"

	// BadTriggerPxRejected represents an order rejected due to invalid TP/SL
	// price
	OrderStatusBadTriggerPxRejected OrderStatus = "badTriggerPxRejected"

	// MarketOrderNoLiquidityRejected represents an order rejected due to lack
	// of liquidity for market order
	OrderStatusMarketOrderNoLiquidityRejected OrderStatus = "marketOrderNoLiquidityRejected"

	// PositionIncreaseAtOpenInterestCapRejected represents an order rejected
	// due to open interest cap
	OrderStatusPositionIncreaseAtOpenInterestCapRejected OrderStatus = "positionIncreaseAtOpenInterestCapRejected"

	// PositionFlipAtOpenInterestCapRejected represents an order rejected due to
	// open interest cap
	OrderStatusPositionFlipAtOpenInterestCapRejected OrderStatus = "positionFlipAtOpenInterestCapRejected"

	// TooAggressiveAtOpenInterestCapRejected represents an order rejected due
	// to price too aggressive at open interest cap
	OrderStatusTooAggressiveAtOpenInterestCapRejected OrderStatus = "tooAggressiveAtOpenInterestCapRejected"

	// OpenInterestIncreaseRejected represents an order rejected due to open
	// interest cap
	OrderStatusOpenInterestIncreaseRejected OrderStatus = "openInterestIncreaseRejected"

	// InsufficientSpotBalanceRejected represents an order rejected due to
	// insufficient spot balance
	OrderStatusInsufficientSpotBalanceRejected OrderStatus = "insufficientSpotBalanceRejected"

	// OracleRejected represents an order rejected due to price too far from
	// oracle
	OrderStatusOracleRejected OrderStatus = "oracleRejected"

	// PerpMaxPositionRejected represents an order rejected due to exceeding
	// margin tier limit at current leverage
	OrderStatusPerpMaxPositionRejected OrderStatus = "perpMaxPositionRejected"
)

// FloatString represents a floating-point number that can be encoded as a JSON
// string or number
type FloatString float64

// UnmarshalJSON implements json.Unmarshaler for FloatString
func (f *FloatString) UnmarshalJSON(b []byte) error {
	// Handle "null"
	if string(b) == "null" {
		*f = 0
		return nil
	}

	// Remove quotes if needed and parse as string
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		*f = FloatString(v)
		return nil
	}

	// Otherwise fall back to normal float unmarshal
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = FloatString(v)
	return nil
}

// OrderChild represents a child order (e.g., TP/SL orders)
type OrderChild struct {
}

// OrderData represents the detailed order information
type OrderData struct {
	Coin             string       `json:"coin"`
	Side             string       `json:"side"`
	LimitPx          FloatString  `json:"limitPx"`
	Sz               FloatString  `json:"sz"`
	Oid              int64        `json:"oid"`
	Timestamp        int64        `json:"timestamp"`
	TriggerCondition string       `json:"triggerCondition"`
	IsTrigger        bool         `json:"isTrigger"`
	TriggerPx        FloatString  `json:"triggerPx"`
	Children         []OrderChild `json:"children"`
	IsPositionTpsl   bool         `json:"isPositionTpsl"`
	ReduceOnly       bool         `json:"reduceOnly"`
	OrderType        string       `json:"orderType"`
	OrigSz           FloatString  `json:"origSz"`
	Tif              string       `json:"tif"`
	Cloid            *types.Cloid `json:"cloid"`
}

// OrderResponse represents an order with its metadata
type OrderResponse struct {
	Order           OrderData   `json:"order"`
	Status          OrderStatus `json:"status"`
	StatusTimestamp int64       `json:"statusTimestamp"`
}

// QueryOrderResponse represents the complete response from
// queryOrderByOid/queryOrderByCloid
type QueryOrderResponse struct {
	Status string        `json:"status"` // "order" indicates this is an order query response
	Order  OrderResponse `json:"order"`
}
