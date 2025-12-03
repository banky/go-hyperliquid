package exchange

import (
	"github.com/samber/mo"
)

/*//////////////////////////////////////////////////////////////
                             ORDER
//////////////////////////////////////////////////////////////*/

// orderOption is an optional config creating
// an order
type orderOption func(*orderConfig)

type orderConfig struct {
	builder  mo.Option[BuilderInfo]
	grouping mo.Option[OrderGrouping]
}

// WithBuilderInfo sets the builder info for the order
func WithBuilderInfo(builder BuilderInfo) orderOption {
	return func(cfg *orderConfig) {
		cfg.builder = mo.Some(builder)
	}
}

func withBuilderInfo(builder mo.Option[BuilderInfo]) orderOption {
	return func(cfg *orderConfig) {
		cfg.builder = builder
	}
}

func WithGrouping(grouping OrderGrouping) orderOption {
	return func(cfg *orderConfig) {
		cfg.grouping = mo.Some(grouping)
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
