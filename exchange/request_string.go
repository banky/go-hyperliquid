package exchange

import (
	"fmt"
	"strings"
)

// String implements fmt.Stringer for orderRequest
func (o orderRequest) String() string {
	cloid := ""
	if c, ok := o.cloid.Get(); ok {
		cloid = c.String()
	}

	return fmt.Sprintf(
		"orderRequest{\n"+
			"  Coin:       %s\n"+
			"  IsBuy:      %v\n"+
			"  Sz:         %g\n"+
			"  LimitPx:    %g\n"+
			"  OrderType:  %s\n"+
			"  ReduceOnly: %v\n"+
			"  Cloid:      %s\n"+
			"}",
		o.coin, o.isBuy, o.sz, o.limitPx, indentString(o.orderType.String(), 2),
		o.reduceOnly, cloid,
	)
}

// String implements fmt.Stringer for OrderType
func (o OrderType) String() string {
	if o.Limit != nil {
		return fmt.Sprintf("OrderType{Limit: %s}", o.Limit)
	}
	if o.Trigger != nil {
		return fmt.Sprintf("OrderType{Trigger: %s}", o.Trigger)
	}
	return "OrderType{}"
}

// String implements fmt.Stringer for LimitOrder
func (l LimitOrder) String() string {
	return fmt.Sprintf(
		"LimitOrder{\n"+
			"  Tif: %s\n"+
			"}",
		l.Tif,
	)
}

// String implements fmt.Stringer for TriggerOrder
func (t TriggerOrder) String() string {
	return fmt.Sprintf(
		"TriggerOrder{\n"+
			"  IsMarket:  %v\n"+
			"  TriggerPx: %g\n"+
			"  TpSl:      %s\n"+
			"}",
		t.IsMarket, t.TriggerPx, t.TpSl,
	)
}

// String implements fmt.Stringer for BuilderInfo
func (b BuilderInfo) String() string {
	return fmt.Sprintf(
		"BuilderInfo{\n"+
			"  PublicAddress: %s\n"+
			"  FeeAmount:     %d\n"+
			"}",
		b.PublicAddress.Hex(), b.FeeAmount,
	)
}

// String implements fmt.Stringer for modifyRequest
func (m modifyRequest) String() string {
	oid := ""
	if id, ok := m.Oid.Get(); ok {
		oid = fmt.Sprintf("%d", id)
	}

	cloid := ""
	if c, ok := m.Cloid.Get(); ok {
		cloid = c.String()
	}

	return fmt.Sprintf(
		"modifyRequest{\n"+
			"  Oid:   %s\n"+
			"  Cloid: %s\n"+
			"  Order: %s\n"+
			"}",
		oid, cloid, indentString(m.Order.String(), 2),
	)
}

// String implements fmt.Stringer for CancelRequest
func (c cancelRequest) String() string {
	return fmt.Sprintf(
		"CancelRequest{\n"+
			"  Coin: %s\n"+
			"  Oid:  %d\n"+
			"}",
		c.Coin, c.Oid,
	)
}

// String implements fmt.Stringer for CancelRequestByCloid
func (c cancelByCloidRequest) String() string {
	return fmt.Sprintf(
		"CancelRequestByCloid{\n"+
			"  Coin:  %s\n"+
			"  Cloid: %s\n"+
			"}",
		c.Coin, c.Cloid.String(),
	)
}

// String implements fmt.Stringer for orderWire
func (o orderWire) String() string {
	cloid := ""
	if o.C != nil {
		cloid = o.C.String()
	}

	return fmt.Sprintf(
		"orderWire{\n"+
			"  A: %d\n"+
			"  B: %v\n"+
			"  P: %s\n"+
			"  S: %s\n"+
			"  R: %v\n"+
			"  T: %s\n"+
			"  C: %s\n"+
			"}",
		o.A, o.B, o.P, o.S, o.R, indentString(o.T.String(), 2), cloid,
	)
}

// String implements fmt.Stringer for orderTypeWire
func (o orderTypeWire) String() string {
	if o.Limit != nil {
		return fmt.Sprintf("orderTypeWire{Limit: %s}", o.Limit)
	}
	if o.Trigger != nil {
		return fmt.Sprintf("orderTypeWire{Trigger: %s}", o.Trigger)
	}
	return "orderTypeWire{}"
}

// String implements fmt.Stringer for triggerOrderWire
func (t triggerOrderWire) String() string {
	return fmt.Sprintf(
		"triggerOrderWire{\n"+
			"  IsMarket:  %v\n"+
			"  TriggerPx: %s\n"+
			"  TpSl:      %s\n"+
			"}",
		t.IsMarket, t.TriggerPx, t.TpSl,
	)
}

// String implements fmt.Stringer for modifyWire
func (m modifyWire) String() string {
	return fmt.Sprintf(
		"modifyWire{\n"+
			"  Oid:   %v\n"+
			"  Order: %s\n"+
			"}",
		m.Oid, indentString(m.Order.String(), 2),
	)
}

// String implements fmt.Stringer for cancelWire
func (c cancelWire) String() string {
	return fmt.Sprintf(
		"cancelWire{\n"+
			"  AssetId: %d\n"+
			"  Oid:     %d\n"+
			"}",
		c.AssetId, c.Oid,
	)
}

// String implements fmt.Stringer for cancelByCloidWire
func (c cancelByCloidWire) String() string {
	return fmt.Sprintf(
		"cancelByCloidWire{\n"+
			"  AssetId: %d\n"+
			"  Cloid:   %s\n"+
			"}",
		c.AssetId, c.Cloid.String(),
	)
}

// String implements fmt.Stringer for orderActionWire
func (o orderAction) String() string {
	orders := formatOrderWireSlice(o.Orders)
	builder := ""
	if o.Builder != nil {
		builder = indentString(o.Builder.String(), 2)
	}

	return fmt.Sprintf(
		"orderActionWire{\n"+
			"  Type:     %s\n"+
			"  Orders:   %s\n"+
			"  Grouping: %s\n"+
			"  Builder:  %s\n"+
			"}",
		o.Type, orders, o.Grouping, builder,
	)
}

// String implements fmt.Stringer for cancelActionWire
func (c cancelAction) String() string {
	return fmt.Sprintf(
		"cancelActionWire{\n"+
			"  Type:    %s\n"+
			"  Cancels: %s\n"+
			"}",
		c.Type, formatCancelWireSlice(c.Cancels),
	)
}

// String implements fmt.Stringer for cancelByCloidAction
func (c cancelByCloidAction) String() string {
	return fmt.Sprintf(
		"cancelByCloidAction{\n"+
			"  Type:    %s\n"+
			"  Cancels: %s\n"+
			"}",
		c.Type, formatCancelByCloidWireSlice(c.Cancels),
	)
}

// String implements fmt.Stringer for batchModifyAction
func (b batchModifyAction) String() string {
	return fmt.Sprintf(
		"batchModifyAction{\n"+
			"  Type:     %s\n"+
			"  Modifies: %s\n"+
			"}",
		b.Type, formatModifyWireSlice(b.Modifies),
	)
}

// Helper functions

func indentString(s string, spaces int) string {
	indent := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func formatOrderWireSlice(orders []orderWire) string {
	if len(orders) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, order := range orders {
		buf.WriteString(fmt.Sprintf("    %s", indentString(order.String(), 4)))
		if i < len(orders)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]")
	return buf.String()
}

func formatCancelWireSlice(cancels []cancelWire) string {
	if len(cancels) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, cancel := range cancels {
		buf.WriteString(fmt.Sprintf("    %s", indentString(cancel.String(), 4)))
		if i < len(cancels)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]")
	return buf.String()
}

func formatCancelByCloidWireSlice(cancels []cancelByCloidWire) string {
	if len(cancels) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, cancel := range cancels {
		buf.WriteString(fmt.Sprintf("    %s", indentString(cancel.String(), 4)))
		if i < len(cancels)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]")
	return buf.String()
}

func formatModifyWireSlice(modifies []modifyWire) string {
	if len(modifies) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, modify := range modifies {
		buf.WriteString(fmt.Sprintf("    %s", indentString(modify.String(), 4)))
		if i < len(modifies)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]")
	return buf.String()
}
