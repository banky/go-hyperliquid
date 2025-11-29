package exchange

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/mo"
)

/*//////////////////////////////////////////////////////////////
                             ORDER
//////////////////////////////////////////////////////////////*/

// CreateOrderOption is a functional option for Order operations
type CreateOrderOption func(*createOrderConfig)

type createOrderConfig struct {
	builder  mo.Option[BuilderInfo]
	grouping mo.Option[OrderGrouping]
}

// WithOrderBuilderInfo sets the builder info for the order
func WithOrderBuilderInfo(builder BuilderInfo) CreateOrderOption {
	return func(cfg *createOrderConfig) {
		cfg.builder = mo.Some(builder)
	}
}

func WithOrderGrouping(grouping OrderGrouping) CreateOrderOption {
	return func(cfg *createOrderConfig) {
		cfg.grouping = mo.Some(grouping)
	}
}

/*//////////////////////////////////////////////////////////////
                          MARKET ORDER
//////////////////////////////////////////////////////////////*/

// MarketOrderOption is a functional option for market orders
type MarketOrderOption func(*marketOrderConfig)

type marketOrderConfig struct {
	px       mo.Option[float64]
	cloid    mo.Option[common.Hash]
	slippage float64
}

// WithMarketOrderPrice sets the limit price for a market order
func WithMarketOrderPrice(px float64) MarketOrderOption {
	return func(cfg *marketOrderConfig) {
		cfg.px = mo.Some(px)
	}
}

// WithMarketOrderSlippage sets the slippage percentage
func WithMarketOrderSlippage(slippage float64) MarketOrderOption {
	return func(cfg *marketOrderConfig) {
		cfg.slippage = slippage
	}
}

// WithMarketOrderCLOID sets the client order ID
func WithMarketOrderCLOID(cloid common.Hash) MarketOrderOption {
	return func(cfg *marketOrderConfig) {
		cfg.cloid = mo.Some(cloid)
	}
}

/*//////////////////////////////////////////////////////////////
                          MARKET CLOSE
//////////////////////////////////////////////////////////////*/

// MarketCloseOption is a functional option for market close operations
type MarketCloseOption func(*marketCloseConfig)

type marketCloseConfig struct {
	sz       mo.Option[float64]
	px       mo.Option[float64]
	slippage float64
	cloid    mo.Option[common.Hash]
}

// WithMarketCloseSize sets the size to close
func WithMarketCloseSize(sz float64) MarketCloseOption {
	return func(cfg *marketCloseConfig) {
		cfg.sz = mo.Some(sz)
	}
}

// WithMarketClosePrice sets the close price
func WithMarketClosePrice(px float64) MarketCloseOption {
	return func(cfg *marketCloseConfig) {
		cfg.px = mo.Some(px)
	}
}

// WithMarketCloseSlippage sets the slippage for close
func WithMarketCloseSlippage(slippage float64) MarketCloseOption {
	return func(cfg *marketCloseConfig) {
		cfg.slippage = slippage
	}
}

func WithMarketCloseCLOID(cloid common.Hash) MarketCloseOption {
	return func(cfg *marketCloseConfig) {
		cfg.cloid = mo.Some(cloid)
	}
}

/*//////////////////////////////////////////////////////////////
                          MODIFY ORDER
//////////////////////////////////////////////////////////////*/

// ModifyOrderOption is a functional option for modify order operations
type ModifyOrderOption func(*modifyOrderConfig)

type modifyOrderConfig struct {
	reduceOnly bool
}

// WithModifyOrderReduceOnly sets the reduce-only flag
func WithModifyOrderReduceOnly(reduceOnly bool) ModifyOrderOption {
	return func(cfg *modifyOrderConfig) {
		cfg.reduceOnly = reduceOnly
	}
}

/*//////////////////////////////////////////////////////////////
                        SCHEDULE CANCEL
//////////////////////////////////////////////////////////////*/

// ScheduleCancelOption is a functional option for modifying scheduled cancel
type ScheduleCancelOption func(*scheduleCancelConfig)

type scheduleCancelConfig struct {
	time mo.Option[time.Duration]
}

func WithScheduleOptionTime(time time.Duration) ScheduleCancelOption {
	return func(cfg *scheduleCancelConfig) {
		cfg.time = mo.Some(time)
	}
}

/*//////////////////////////////////////////////////////////////
                         APPROVE AGENT
//////////////////////////////////////////////////////////////*/

// ApproveAgentOption is a functional option for approve agent operations
type ApproveAgentOption func(*approveAgentConfig)

type approveAgentConfig struct {
	name mo.Option[string]
}

// WithAgentName sets the name for the agent
func WithAgentName(name string) ApproveAgentOption {
	return func(cfg *approveAgentConfig) {
		cfg.name = mo.Some(name)
	}
}
