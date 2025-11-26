package ws

import (
	"fmt"
	"strings"
)

// ===== Subscription Types =====

// Subscription interface defines the contract for all subscription types
type Subscription interface {
	channelName() string
	identifier() string
	subscriptionPayload() any
}

// AllMidsSubscription subscribes to all mid-prices
type AllMidsSubscription struct{}

func (s AllMidsSubscription) channelName() string { return "allMids" }
func (s AllMidsSubscription) identifier() string  { return "allMids" }
func (s AllMidsSubscription) subscriptionPayload() any {
	return map[string]any{"type": "allMids"}
}

// L2BookSubscription subscribes to level 2 order book for a coin
type L2BookSubscription struct {
	Coin string
}

func (s L2BookSubscription) channelName() string { return "l2Book" }
func (s L2BookSubscription) identifier() string {
	return fmt.Sprintf("l2Book:%s", strings.ToLower(s.Coin))
}
func (s L2BookSubscription) subscriptionPayload() any {
	return map[string]any{"type": "l2Book", "coin": s.Coin}
}

// TradesSubscription subscribes to trades for a coin
type TradesSubscription struct {
	Coin string
}

func (s TradesSubscription) channelName() string { return "trades" }
func (s TradesSubscription) identifier() string {
	return fmt.Sprintf("trades:%s", strings.ToLower(s.Coin))
}
func (s TradesSubscription) subscriptionPayload() any {
	return map[string]any{"type": "trades", "coin": s.Coin}
}

// UserEventsSubscription subscribes to user events
type UserEventsSubscription struct {
	User string
}

func (s UserEventsSubscription) channelName() string { return "user" }
func (s UserEventsSubscription) identifier() string  { return "userEvents" }
func (s UserEventsSubscription) subscriptionPayload() any {
	return map[string]any{"type": "userEvents", "user": s.User}
}

// UserFillsSubscription subscribes to user fills
type UserFillsSubscription struct {
	User string
}

func (s UserFillsSubscription) channelName() string { return "userFills" }
func (s UserFillsSubscription) identifier() string {
	return fmt.Sprintf("userFills:%s", strings.ToLower(s.User))
}
func (s UserFillsSubscription) subscriptionPayload() any {
	return map[string]any{"type": "userFills", "user": s.User}
}

// CandleSubscription subscribes to candle data for a coin and interval
type CandleSubscription struct {
	Coin     string
	Interval string
}

func (s CandleSubscription) channelName() string { return "candle" }
func (s CandleSubscription) identifier() string {
	return fmt.Sprintf("candle:%s,%s", strings.ToLower(s.Coin), s.Interval)
}
func (s CandleSubscription) subscriptionPayload() any {
	return map[string]any{"type": "candle", "coin": s.Coin, "interval": s.Interval}
}

// OrderUpdatesSubscription subscribes to order updates
type OrderUpdatesSubscription struct {
	User string
}

func (s OrderUpdatesSubscription) channelName() string { return "orderUpdates" }
func (s OrderUpdatesSubscription) identifier() string  { return "orderUpdates" }
func (s OrderUpdatesSubscription) subscriptionPayload() any {
	return map[string]any{"type": "orderUpdates", "user": s.User}
}

// UserFundingsSubscription subscribes to user fundings
type UserFundingsSubscription struct {
	User string
}

func (s UserFundingsSubscription) channelName() string { return "userFundings" }
func (s UserFundingsSubscription) identifier() string {
	return fmt.Sprintf("userFundings:%s", strings.ToLower(s.User))
}
func (s UserFundingsSubscription) subscriptionPayload() any {
	return map[string]any{"type": "userFundings", "user": s.User}
}

// UserNonFundingLedgerUpdatesSubscription subscribes to user non-funding ledger updates
type UserNonFundingLedgerUpdatesSubscription struct {
	User string
}

func (s UserNonFundingLedgerUpdatesSubscription) channelName() string {
	return "userNonFundingLedgerUpdates"
}
func (s UserNonFundingLedgerUpdatesSubscription) identifier() string {
	return fmt.Sprintf("userNonFundingLedgerUpdates:%s", strings.ToLower(s.User))
}
func (s UserNonFundingLedgerUpdatesSubscription) subscriptionPayload() any {
	return map[string]any{"type": "userNonFundingLedgerUpdates", "user": s.User}
}

// WebData2Subscription subscribes to web data for a user
type WebData2Subscription struct {
	User string
}

func (s WebData2Subscription) channelName() string { return "webData2" }
func (s WebData2Subscription) identifier() string {
	return fmt.Sprintf("webData2:%s", strings.ToLower(s.User))
}
func (s WebData2Subscription) subscriptionPayload() any {
	return map[string]any{"type": "webData2", "user": s.User}
}

// BboSubscription subscribes to best bid/offer for a coin
type BboSubscription struct {
	Coin string
}

func (s BboSubscription) channelName() string { return "bbo" }
func (s BboSubscription) identifier() string  { return fmt.Sprintf("bbo:%s", strings.ToLower(s.Coin)) }
func (s BboSubscription) subscriptionPayload() any {
	return map[string]any{"type": "bbo", "coin": s.Coin}
}

// ActiveAssetCtxSubscription subscribes to active asset context
type ActiveAssetCtxSubscription struct {
	Coin string
}

func (s ActiveAssetCtxSubscription) channelName() string { return "activeAssetCtx" }
func (s ActiveAssetCtxSubscription) identifier() string {
	return fmt.Sprintf("activeAssetCtx:%s", strings.ToLower(s.Coin))
}
func (s ActiveAssetCtxSubscription) subscriptionPayload() any {
	return map[string]any{"type": "activeAssetCtx", "coin": s.Coin}
}

// ActiveAssetDataSubscription subscribes to active asset data for a user and coin
type ActiveAssetDataSubscription struct {
	User string
	Coin string
}

func (s ActiveAssetDataSubscription) channelName() string { return "activeAssetData" }
func (s ActiveAssetDataSubscription) identifier() string {
	return fmt.Sprintf("activeAssetData:%s,%s", strings.ToLower(s.Coin), strings.ToLower(s.User))
}
func (s ActiveAssetDataSubscription) subscriptionPayload() any {
	return map[string]any{"type": "activeAssetData", "user": s.User, "coin": s.Coin}
}

// ===== Message Types =====

// L2Level represents a single level in the order book
type L2Level struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}

// AllMidsMessage contains all mid-prices
type AllMidsMessage struct {
	Mids map[string]string `json:"mids"`
}

// L2BookMessage contains level 2 order book data
type L2BookMessage struct {
	Coin   string       `json:"coin"`
	Levels [2][]L2Level `json:"levels"`
	Time   int64        `json:"time"`
}

// Trade represents a single trade
type Trade struct {
	Coin string `json:"coin"`
	Side string `json:"side"` // "A" or "B"
	Px   string `json:"px"`
	Sz   int    `json:"sz"`
	Hash string `json:"hash"`
	Time int64  `json:"time"`
}

// TradesMessage contains a list of trades
type TradesMessage struct {
	Trades []Trade `json:"trades"`
}

// Fill represents a user fill/trade execution
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

// UserEventsMessage contains user event data (fills, etc.)
type UserEventsMessage struct {
	Fills []Fill `json:"fills"`
}

// UserFillsMessage contains user fill data
type UserFillsMessage struct {
	User       string `json:"user"`
	IsSnapshot bool   `json:"isSnapshot"`
	Fills      []Fill `json:"fills"`
}

// BboData represents best bid/offer
type BboData struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}

// BboMessage contains best bid/offer data
type BboMessage struct {
	Coin string      `json:"coin"`
	Time int64       `json:"time"`
	Bbo  [2]*BboData `json:"bbo"` // [bid, ask]
}

// CandleMessage contains candlestick data
type CandleMessage struct {
	S string `json:"s"` // Symbol (coin)
	I string `json:"i"` // Interval
	O string `json:"o"` // Open
	C string `json:"c"` // Close
	H string `json:"h"` // High
	L string `json:"l"` // Low
	V string `json:"v"` // Volume
	T int64  `json:"t"` // Timestamp
}

// OrderUpdatesMessage contains order update data
type OrderUpdatesMessage map[string]any

// UserFundingsMessage contains user funding data
type UserFundingsMessage map[string]any

// UserNonFundingLedgerUpdatesMessage contains non-funding ledger updates
type UserNonFundingLedgerUpdatesMessage map[string]any

// WebData2Message contains web data
type WebData2Message map[string]any

// PerpAssetCtx contains perp market context
type PerpAssetCtx struct {
	Funding      string     `json:"funding"`
	OpenInterest string     `json:"openInterest"`
	PrevDayPx    string     `json:"prevDayPx"`
	DayNtlVlm    string     `json:"dayNtlVlm"`
	Premium      string     `json:"premium"`
	OraclePx     string     `json:"oraclePx"`
	MarkPx       string     `json:"markPx"`
	MidPx        *string    `json:"midPx"`
	ImpactPxs    *[2]string `json:"impactPxs"`
	DayBaseVlm   string     `json:"dayBaseVlm"`
}

// ActiveAssetCtxMessage contains active asset context for perpetuals
type ActiveAssetCtxMessage struct {
	Coin string       `json:"coin"`
	Ctx  PerpAssetCtx `json:"ctx"`
}

// SpotAssetCtx contains spot asset context
type SpotAssetCtx struct {
	DayNtlVlm         string  `json:"dayNtlVlm"`
	MarkPx            string  `json:"markPx"`
	MidPx             *string `json:"midPx"`
	PrevDayPx         string  `json:"prevDayPx"`
	CirculatingSupply string  `json:"circulatingSupply"`
	Coin              string  `json:"coin"`
}

// ActiveSpotAssetCtxMessage contains active spot asset context
type ActiveSpotAssetCtxMessage struct {
	Coin string       `json:"coin"`
	Ctx  SpotAssetCtx `json:"ctx"`
}

// Leverage represents leverage info
type Leverage struct {
	Type   string  `json:"type"` // "cross" or "isolated"
	Value  int     `json:"value"`
	RawUsd *string `json:"rawUsd,omitempty"` // Only for isolated
}

// ActiveAssetDataMessage contains active asset data for a user and coin
type ActiveAssetDataMessage struct {
	User             string    `json:"user"`
	Coin             string    `json:"coin"`
	Leverage         Leverage  `json:"leverage"`
	MaxTradeSzs      [2]string `json:"maxTradeSzs"`
	AvailableToTrade [2]string `json:"availableToTrade"`
	MarkPx           string    `json:"markPx"`
}

// PongMessage is a ping/pong response
type PongMessage struct{}
