package info

import (
	"github.com/banky/go-hyperliquid/types"
	"github.com/ethereum/go-ethereum/common"
)

// L2Level represents a single level in the order book
type L2Level struct {
	Px types.FloatString `json:"px"`
	Sz types.FloatString `json:"sz"`
	N  types.FloatString `json:"n"`
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

// Position represents a user's position in a coin
type Position struct {
	Coin           string             `json:"coin"`
	EntryPx        *types.FloatString `json:"entryPx"`
	Leverage       Leverage           `json:"leverage"`
	LiquidationPx  *types.FloatString `json:"liquidationPx"`
	MarginUsed     types.FloatString  `json:"marginUsed"`
	PositionValue  types.FloatString  `json:"positionValue"`
	ReturnOnEquity types.FloatString  `json:"returnOnEquity"`
	Szi            types.FloatString  `json:"szi"`
	UnrealizedPnl  types.FloatString  `json:"unrealizedPnl"`
}

// AssetPosition represents a user's position in an asset
type AssetPosition struct {
	Position Position `json:"position"`
	Type     string   `json:"type"`
}

// Leverage represents leverage configuration
type Leverage struct {
	Type   string             `json:"type"` // "cross" or "isolated"
	Value  int64              `json:"value"`
	RawUsd *types.FloatString `json:"rawUsd,omitempty"` // Only for isolated
}

// MarginSummary contains margin information
type MarginSummary struct {
	AccountValue    types.FloatString `json:"accountValue"`
	TotalMarginUsed types.FloatString `json:"totalMarginUsed"`
	TotalNtlPos     types.FloatString `json:"totalNtlPos"`
	TotalRawUsd     types.FloatString `json:"totalRawUsd"`
}

// UserState contains detailed trading information about a user
type UserState struct {
	AssetPositions     []AssetPosition   `json:"assetPositions"`
	CrossMarginSummary MarginSummary     `json:"crossMarginSummary"`
	MarginSummary      MarginSummary     `json:"marginSummary"`
	Withdrawable       types.FloatString `json:"withdrawable"`
}

type Balance struct {
	Coin     string            `json:"coin"`
	Token    int64             `json:"token"`
	Total    types.FloatString `json:"total"`
	Hold     types.FloatString `json:"hold"`
	EntryNtl types.FloatString `json:"entryNtl"`
}

// SpotUserState contains the user’s token balances
// for spot trading on the Hyperliquid exchange
type SpotUserState struct {
	Balances []Balance `json:"balances"`
}

// OpenOrder represents an open order
type OpenOrder struct {
	Coin      string            `json:"coin"`
	LimitPx   types.FloatString `json:"limitPx"`
	Oid       int64             `json:"oid"`
	Side      string            `json:"side"`
	Sz        types.FloatString `json:"sz"`
	Timestamp int64             `json:"timestamp"`
}

// Fill represents a fill/executed trade
type Fill struct {
	Coin          string            `json:"coin"`
	Px            types.FloatString `json:"px"`
	Sz            types.FloatString `json:"sz"`
	Side          string            `json:"side"`
	Time          int64             `json:"time"`
	StartPosition types.FloatString `json:"startPosition"`
	Dir           string            `json:"dir"`
	ClosedPnl     types.FloatString `json:"closedPnl"`
	Hash          common.Hash       `json:"hash"`
	Oid           int64             `json:"oid"`
	Crossed       bool              `json:"crossed"`
	Fee           types.FloatString `json:"fee"`
	Tid           int64             `json:"tid"`
	FeeToken      string            `json:"feeToken"`
}

// FundingRecord represents a funding payment record
type FundingRecord struct {
	Coin        string            `json:"coin"`
	FundingRate types.FloatString `json:"fundingRate"`
	Premium     types.FloatString `json:"premium"`
	Time        int64             `json:"time"`
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

// OrderChild represents a child order (e.g., TP/SL orders)
type OrderChild struct {
}

// OrderData represents the detailed order information
type OrderData struct {
	Coin             string            `json:"coin"`
	Side             string            `json:"side"`
	LimitPx          types.FloatString `json:"limitPx"`
	Sz               types.FloatString `json:"sz"`
	Oid              int64             `json:"oid"`
	Timestamp        int64             `json:"timestamp"`
	TriggerCondition string            `json:"triggerCondition"`
	IsTrigger        bool              `json:"isTrigger"`
	TriggerPx        types.FloatString `json:"triggerPx"`
	Children         []OrderChild      `json:"children"`
	IsPositionTpsl   bool              `json:"isPositionTpsl"`
	ReduceOnly       bool              `json:"reduceOnly"`
	OrderType        string            `json:"orderType"`
	OrigSz           types.FloatString `json:"origSz"`
	Tif              string            `json:"tif"`
	Cloid            *types.Cloid      `json:"cloid"`
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

// FundingDelta represents the funding delta information
type FundingDelta struct {
	Coin        string            `json:"coin"`
	FundingRate types.FloatString `json:"fundingRate"`
	NSamples    int64             `json:"nSamples"`
	Szi         types.FloatString `json:"szi"`
	Type        string            `json:"type"`
	Usdc        types.FloatString `json:"usdc"`
}

// Funding represents a funding update event
type Funding struct {
	Delta FundingDelta  `json:"delta"`
	Hash  common.Hash   `json:"hash"`
	Time  int64         `json:"time"`
}

// DailyVolume represents daily user volume data
type DailyVolume struct {
	Date     string            `json:"date"`
	UserCross types.FloatString `json:"userCross"`
	UserAdd  types.FloatString `json:"userAdd"`
	Exchange types.FloatString `json:"exchange"`
}

// FeeTier represents a fee tier with notional cutoff
type FeeTier struct {
	NtlCutoff types.FloatString `json:"ntlCutoff"`
	Cross     types.FloatString `json:"cross"`
	Add       types.FloatString `json:"add"`
	SpotCross types.FloatString `json:"spotCross"`
	SpotAdd   types.FloatString `json:"spotAdd"`
}

// MakerFeeRebate represents a maker fee rebate tier
type MakerFeeRebate struct {
	MakerFractionCutoff types.FloatString `json:"makerFractionCutoff"`
	Add                 types.FloatString `json:"add"`
}

// FeeTiers contains VIP and market maker fee tiers
type FeeTiers struct {
	Vip []FeeTier        `json:"vip"`
	Mm  []MakerFeeRebate `json:"mm"`
}

// StakingDiscountTier represents a staking discount tier
type StakingDiscountTier struct {
	BpsOfMaxSupply types.FloatString `json:"bpsOfMaxSupply"`
	Discount       types.FloatString `json:"discount"`
}

// FeeSchedule contains fee information and tier structures
type FeeSchedule struct {
	Cross                types.FloatString     `json:"cross"`
	Add                  types.FloatString     `json:"add"`
	SpotCross            types.FloatString     `json:"spotCross"`
	SpotAdd              types.FloatString     `json:"spotAdd"`
	Tiers                FeeTiers              `json:"tiers"`
	ReferralDiscount     types.FloatString     `json:"referralDiscount"`
	StakingDiscountTiers []StakingDiscountTier `json:"stakingDiscountTiers"`
}

// UserFeeInfo contains comprehensive user fee information
type UserFeeInfo struct {
	DailyUserVlm              []DailyVolume         `json:"dailyUserVlm"`
	FeeSchedule               FeeSchedule           `json:"feeSchedule"`
	UserCrossRate             types.FloatString     `json:"userCrossRate"`
	UserAddRate               types.FloatString     `json:"userAddRate"`
	UserSpotCrossRate         types.FloatString     `json:"userSpotCrossRate"`
	UserSpotAddRate           types.FloatString     `json:"userSpotAddRate"`
	ActiveReferralDiscount    types.FloatString     `json:"activeReferralDiscount"`
	Trial                     *string               `json:"trial"`
	FeeTrialEscrow            types.FloatString     `json:"feeTrialEscrow"`
	NextTrialAvailableTimestamp *int64              `json:"nextTrialAvailableTimestamp"`
	StakingLink               *string               `json:"stakingLink"`
	ActiveStakingDiscount     StakingDiscountTier   `json:"activeStakingDiscount"`
}
