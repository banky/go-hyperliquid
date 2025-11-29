package info

import (
	"fmt"
	"strings"
)

// String implements fmt.Stringer for L2Level
func (l L2Level) String() string {
	return fmt.Sprintf(
		"L2Level{\n"+
			"  Px: %s\n"+
			"  Sz: %s\n"+
			"  N:  %s\n"+
			"}",
		l.Px, l.Sz, l.N,
	)
}

// String implements fmt.Stringer for L2BookSnapshot
func (l L2BookSnapshot) String() string {
	bidLevels := formatLevels(l.Levels[0])
	askLevels := formatLevels(l.Levels[1])

	return fmt.Sprintf(
		"L2BookSnapshot{\n"+
			"  Coin:  %s\n"+
			"  Time:  %d\n"+
			"  Levels: {\n"+
			"    Bids: %s\n"+
			"    Asks: %s\n"+
			"  }\n"+
			"}",
		l.Coin, l.Time, bidLevels, askLevels,
	)
}

// String implements fmt.Stringer for AssetInfo
func (a AssetInfo) String() string {
	return fmt.Sprintf(
		"AssetInfo{\n"+
			"  Name:       %s\n"+
			"  SzDecimals: %d\n"+
			"}",
		a.Name, a.SzDecimals,
	)
}

// String implements fmt.Stringer for Meta
func (m Meta) String() string {
	return fmt.Sprintf(
		"Meta{\n"+
			"  Universe: %s\n"+
			"}",
		formatAssetInfoSlice(m.Universe),
	)
}

// String implements fmt.Stringer for SpotAssetInfo
func (s SpotAssetInfo) String() string {
	return fmt.Sprintf(
		"SpotAssetInfo{\n"+
			"  Name:        %s\n"+
			"  Tokens:      [%d, %d]\n"+
			"  Index:       %d\n"+
			"  IsCanonical: %v\n"+
			"}",
		s.Name, s.Tokens[0], s.Tokens[1], s.Index, s.IsCanonical,
	)
}

// String implements fmt.Stringer for SpotTokenInfo
func (s SpotTokenInfo) String() string {
	fullName := ""
	if s.FullName != nil {
		fullName = *s.FullName
	}

	return fmt.Sprintf(
		"SpotTokenInfo{\n"+
			"  Name:             %s\n"+
			"  SzDecimals:       %d\n"+
			"  WeiDecimals:      %d\n"+
			"  Index:            %d\n"+
			"  TokenId:          %s\n"+
			"  IsCanonical:      %v\n"+
			"  EvmContract:      %s\n"+
			"  FullName:         %s\n"+
			"}",
		s.Name, s.SzDecimals, s.WeiDecimals, s.Index, s.TokenId,
		s.IsCanonical, indentString(s.EvmContract.String(), 2), fullName,
	)
}

// String implements fmt.Stringer for EvmContract
func (e EvmContract) String() string {
	return fmt.Sprintf(
		"EvmContract{\n"+
			"  Address:             %s\n"+
			"  EvmExtraWeiDecimals: %d\n"+
			"}",
		e.Address.Hex(), e.EvmExtraWeiDecimals,
	)
}

// String implements fmt.Stringer for SpotMeta
func (s SpotMeta) String() string {
	return fmt.Sprintf(
		"SpotMeta{\n"+
			"  Universe: %s\n"+
			"  Tokens:   %s\n"+
			"}",
		formatSpotAssetInfoSlice(s.Universe),
		formatSpotTokenInfoSlice(s.Tokens),
	)
}

// String implements fmt.Stringer for Position
func (p Position) String() string {
	entryPx := ""
	if p.EntryPx != nil {
		entryPx = p.EntryPx.String()
	}

	liquidationPx := ""
	if p.LiquidationPx != nil {
		liquidationPx = p.LiquidationPx.String()
	}

	return fmt.Sprintf(
		"Position{\n"+
			"  Coin:           %s\n"+
			"  EntryPx:        %s\n"+
			"  Leverage:       %s\n"+
			"  LiquidationPx:  %s\n"+
			"  MarginUsed:     %s\n"+
			"  PositionValue:  %s\n"+
			"  ReturnOnEquity: %s\n"+
			"  Szi:            %s\n"+
			"  UnrealizedPnl:  %s\n"+
			"}",
		p.Coin, entryPx, indentString(p.Leverage.String(), 2), liquidationPx,
		p.MarginUsed, p.PositionValue, p.ReturnOnEquity, p.Szi, p.UnrealizedPnl,
	)
}

// String implements fmt.Stringer for AssetPosition
func (a AssetPosition) String() string {
	return fmt.Sprintf(
		"AssetPosition{\n"+
			"  Position: %s\n"+
			"  Type:     %s\n"+
			"}",
		indentString(a.Position.String(), 2), a.Type,
	)
}

// String implements fmt.Stringer for Leverage
func (l Leverage) String() string {
	rawUsd := ""
	if l.RawUsd != nil {
		rawUsd = l.RawUsd.String()
	}

	return fmt.Sprintf(
		"Leverage{\n"+
			"  Type:   %s\n"+
			"  Value:  %d\n"+
			"  RawUsd: %s\n"+
			"}",
		l.Type, l.Value, rawUsd,
	)
}

// String implements fmt.Stringer for MarginSummary
func (m MarginSummary) String() string {
	return fmt.Sprintf(
		"MarginSummary{\n"+
			"  AccountValue:    %s\n"+
			"  TotalMarginUsed: %s\n"+
			"  TotalNtlPos:     %s\n"+
			"  TotalRawUsd:     %s\n"+
			"}",
		m.AccountValue, m.TotalMarginUsed, m.TotalNtlPos, m.TotalRawUsd,
	)
}

// String implements fmt.Stringer for UserState
func (u UserState) String() string {
	return fmt.Sprintf(
		"UserState{\n"+
			"  AssetPositions:     %s\n"+
			"  CrossMarginSummary: %s\n"+
			"  MarginSummary:      %s\n"+
			"  Withdrawable:       %s\n"+
			"}",
		formatAssetPositionSlice(u.AssetPositions),
		indentString(u.CrossMarginSummary.String(), 2),
		indentString(u.MarginSummary.String(), 2),
		u.Withdrawable,
	)
}

// String implements fmt.Stringer for OpenOrder
func (o OpenOrder) String() string {
	return fmt.Sprintf(
		"OpenOrder{\n"+
			"  Coin:      %s\n"+
			"  LimitPx:   %s\n"+
			"  Oid:       %d\n"+
			"  Side:      %s\n"+
			"  Sz:        %s\n"+
			"  Timestamp: %d\n"+
			"}",
		o.Coin, o.LimitPx, o.Oid, o.Side, o.Sz, o.Timestamp,
	)
}

// String implements fmt.Stringer for Fill
func (f Fill) String() string {
	return fmt.Sprintf(
		"Fill{\n"+
			"  Coin:          %s\n"+
			"  Px:            %s\n"+
			"  Sz:            %s\n"+
			"  Side:          %s\n"+
			"  Time:          %d\n"+
			"  StartPosition: %s\n"+
			"  Dir:           %s\n"+
			"  ClosedPnl:     %s\n"+
			"  Hash:          %s\n"+
			"  Oid:           %d\n"+
			"  Crossed:       %v\n"+
			"  Fee:           %s\n"+
			"  Tid:           %d\n"+
			"  FeeToken:      %s\n"+
			"}",
		f.Coin, f.Px, f.Sz, f.Side, f.Time, f.StartPosition, f.Dir,
		f.ClosedPnl, f.Hash, f.Oid, f.Crossed, f.Fee, f.Tid, f.FeeToken,
	)
}

// String implements fmt.Stringer for FundingRecord
func (f FundingRecord) String() string {
	return fmt.Sprintf(
		"FundingRecord{\n"+
			"  Coin:        %s\n"+
			"  FundingRate: %s\n"+
			"  Premium:     %s\n"+
			"  Time:        %d\n"+
			"}",
		f.Coin, f.FundingRate, f.Premium, f.Time,
	)
}

// String implements fmt.Stringer for Candle
func (c Candle) String() string {
	return fmt.Sprintf(
		"Candle{\n"+
			"  T: %d\n"+
			"  O: %s\n"+
			"  C: %s\n"+
			"  H: %s\n"+
			"  L: %s\n"+
			"  V: %s\n"+
			"  N: %d\n"+
			"  S: %s\n"+
			"  I: %s\n"+
			"}",
		c.T, c.O, c.C, c.H, c.L, c.V, c.N, c.S, c.I,
	)
}

// String implements fmt.Stringer for OrderChild
func (oc OrderChild) String() string {
	return "OrderChild{}"
}

// String implements fmt.Stringer for OrderData
func (od OrderData) String() string {
	cloid := ""
	if od.Cloid != nil {
		cloid = od.Cloid.String()
	}

	return fmt.Sprintf(
		"OrderData{\n"+
			"  Coin:               %s\n"+
			"  Side:               %s\n"+
			"  LimitPx:            %s\n"+
			"  Sz:                 %s\n"+
			"  Oid:                %d\n"+
			"  Timestamp:          %d\n"+
			"  TriggerCondition:   %s\n"+
			"  IsTrigger:          %v\n"+
			"  TriggerPx:          %s\n"+
			"  Children:           %s\n"+
			"  IsPositionTpsl:     %v\n"+
			"  ReduceOnly:         %v\n"+
			"  OrderType:          %s\n"+
			"  OrigSz:             %s\n"+
			"  Tif:                %s\n"+
			"  Cloid:              %s\n"+
			"}",
		od.Coin,
		od.Side,
		od.LimitPx,
		od.Sz,
		od.Oid,
		od.Timestamp,
		od.TriggerCondition,
		od.IsTrigger,
		od.TriggerPx,
		formatOrderChildSlice(od.Children),
		od.IsPositionTpsl,
		od.ReduceOnly,
		od.OrderType,
		od.OrigSz,
		od.Tif,
		cloid,
	)
}

// String implements fmt.Stringer for OrderResponse
func (or OrderResponse) String() string {
	return fmt.Sprintf(
		"OrderResponse{\n"+
			"  Order:           %s\n"+
			"  Status:          %s\n"+
			"  StatusTimestamp: %d\n"+
			"}",
		indentString(or.Order.String(), 2), or.Status, or.StatusTimestamp,
	)
}

// String implements fmt.Stringer for QueryOrderResponse
func (qor QueryOrderResponse) String() string {
	return fmt.Sprintf(
		"QueryOrderResponse{\n"+
			"  Status: %s\n"+
			"  Order:  %s\n"+
			"}",
		qor.Status, indentString(qor.Order.String(), 2),
	)
}

// Helper functions

func indentString(s string, spaces int64) string {
	indent := strings.Repeat(" ", int(spaces))
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = indent + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func formatLevels(levels []L2Level) string {
	if len(levels) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, level := range levels {
		buf.WriteString(
			fmt.Sprintf("      %s", indentString(level.String(), 6)),
		)
		if i < len(levels)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("    ]")
	return buf.String()
}

func formatAssetInfoSlice(assets []AssetInfo) string {
	if len(assets) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, asset := range assets {
		buf.WriteString(fmt.Sprintf("    %s", indentString(asset.String(), 4)))
		if i < len(assets)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]")
	return buf.String()
}

func formatSpotAssetInfoSlice(assets []SpotAssetInfo) string {
	if len(assets) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, asset := range assets {
		buf.WriteString(fmt.Sprintf("    %s", indentString(asset.String(), 4)))
		if i < len(assets)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]")
	return buf.String()
}

func formatSpotTokenInfoSlice(tokens []SpotTokenInfo) string {
	if len(tokens) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, token := range tokens {
		buf.WriteString(fmt.Sprintf("    %s", indentString(token.String(), 4)))
		if i < len(tokens)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]")
	return buf.String()
}

func formatAssetPositionSlice(positions []AssetPosition) string {
	if len(positions) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, pos := range positions {
		buf.WriteString(fmt.Sprintf("    %s", indentString(pos.String(), 4)))
		if i < len(positions)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("  ]")
	return buf.String()
}

func formatOrderChildSlice(children []OrderChild) string {
	if len(children) == 0 {
		return "[]"
	}
	var buf strings.Builder
	buf.WriteString("[\n")
	for i, child := range children {
		buf.WriteString(
			fmt.Sprintf("      %s", indentString(child.String(), 6)),
		)
		if i < len(children)-1 {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("    ]")
	return buf.String()
}
