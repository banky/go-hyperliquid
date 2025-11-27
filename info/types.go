package info

// ===== Market Data Types =====

// L2Level represents a single level in the order book
type L2Level struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
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
	SzDecimals int    `json:"szDecimals"`
}

// Meta contains exchange metadata for perpetuals
type Meta struct {
	Universe []AssetInfo `json:"universe"`
}

// SpotAssetInfo contains spot asset metadata
type SpotAssetInfo struct {
	Name        string `json:"name"`
	Tokens      [2]int `json:"tokens"`
	Index       int    `json:"index"`
	IsCanonical bool   `json:"isCanonical"`
}

// SpotTokenInfo contains spot token metadata
type SpotTokenInfo struct {
	Name        string  `json:"name"`
	SzDecimals  int     `json:"szDecimals"`
	WeiDecimals int     `json:"weiDecimals"`
	Index       int     `json:"index"`
	TokenId     string  `json:"tokenId"`
	IsCanonical bool    `json:"isCanonical"`
	EvmContract *string `json:"evmContract"`
	FullName    *string `json:"fullName"`
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
	Value  int     `json:"value"`
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
	Oid       int    `json:"oid"`
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
	Oid           int    `json:"oid"`
	Crossed       bool   `json:"crossed"`
	Fee           string `json:"fee"`
	Tid           int    `json:"tid"`
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
	N int    `json:"n"` // Number of trades
	S string `json:"s"` // Symbol
	I string `json:"i"` // Interval
}
