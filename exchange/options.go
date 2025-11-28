package exchange

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/samber/mo"
)

// ===== Order Options =====

// OrderOption is a functional option for Order operations
type OrderOption func(*orderConfig)

type orderConfig struct {
	reduceOnly bool
	cloid      mo.Option[common.Hash]
	builder    mo.Option[BuilderInfo]
}

// WithOrderReduceOnly sets the reduce-only flag
func WithOrderReduceOnly(reduceOnly bool) OrderOption {
	return func(cfg *orderConfig) {
		cfg.reduceOnly = reduceOnly
	}
}

// WithOrderCLOID sets the client order ID
func WithOrderCLOID(cloid common.Hash) OrderOption {
	return func(cfg *orderConfig) {
		cfg.cloid = mo.Some(cloid)
	}
}

// withOrderCLOID is internally used for setting CLOID with an optional
func withOrderCLOID(cloid mo.Option[common.Hash]) OrderOption {
	return func(cfg *orderConfig) {
		cfg.cloid = cloid
	}
}

func defaultOrderConfig() orderConfig {
	return orderConfig{
		reduceOnly: false,
	}
}

func (o orderConfig) getCLOID() *common.Hash {
	if cloid, ok := o.cloid.Get(); ok {
		return &cloid
	}
	return nil
}

func (o orderConfig) getBuilderInfo() *BuilderInfo {
	if builderInfo, ok := o.builder.Get(); ok {
		return &builderInfo
	}
	return nil
}

// ===== Market Order Options =====

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

func defaultMarketOrderConfig() marketOrderConfig {
	return marketOrderConfig{
		slippage: DEFAULT_SLIPPAGE,
	}
}

func (m marketOrderConfig) getCLOID() *common.Hash {
	if cloid, ok := m.cloid.Get(); ok {
		return &cloid
	}
	return nil
}

// ===== Market Close Options =====

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

func defaultMarketCloseConfig() marketCloseConfig {
	return marketCloseConfig{
		slippage: DEFAULT_SLIPPAGE,
	}
}

// ===== Modify Order Options =====

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

func defaultModifyOrderConfig() modifyOrderConfig {
	return modifyOrderConfig{
		reduceOnly: false,
	}
}

// ===== Schedule Cancel Options =====

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

func defaultScheduleCancelConfig() scheduleCancelConfig {
	return scheduleCancelConfig{}
}

// ===== Approve Agent Options =====

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

func defaultApproveAgentConfig() approveAgentConfig {
	return approveAgentConfig{}
}
