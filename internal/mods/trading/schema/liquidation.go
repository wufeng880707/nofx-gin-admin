package schema

import (
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
)

type MarketLiquidationEvent struct {
	ID           uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	Exchange     string    `json:"exchange" gorm:"size:16;not null;index:idx_market_liquidation_scope,priority:1"`
	Market       string    `json:"market" gorm:"size:16;not null;index:idx_market_liquidation_scope,priority:2"`
	Symbol       string    `json:"symbol" gorm:"size:64;not null;index:idx_market_liquidation_scope,priority:3"`
	Side         string    `json:"side" gorm:"size:8;not null;index"`
	Price        string    `json:"price" gorm:"type:decimal(36,18);not null"`
	Quantity     string    `json:"quantity" gorm:"type:decimal(36,18);not null"`
	NotionalUSDT string    `json:"notional_usdt" gorm:"type:decimal(36,18);not null"`
	EventAt      time.Time `json:"event_at" gorm:"not null;index:idx_market_liquidation_scope,priority:4"`
	PayloadHash  string    `json:"-" gorm:"size:64;not null;uniqueIndex"`
	Source       string    `json:"source" gorm:"size:32;not null"`
	CreatedAt    time.Time `json:"created_at"`
}

func (*MarketLiquidationEvent) TableName() string {
	return config.C.FormatTableName("market_liquidation_events")
}

type LiquidationHeatmapQuery struct {
	Symbol       string `form:"symbol"`
	Hours        int    `form:"hours"`
	TimeBuckets  int    `form:"timeBuckets"`
	PriceBuckets int    `form:"priceBuckets"`
}

func (a *LiquidationHeatmapQuery) Validate() error {
	a.Symbol = strings.ToUpper(strings.TrimSpace(a.Symbol))
	if a.Symbol == "" || len(a.Symbol) > 40 {
		return errors.BadRequest("market_liquidation_symbol_invalid", "Liquidation symbol is required")
	}
	if a.Hours == 0 {
		a.Hours = 24
	}
	if a.TimeBuckets == 0 {
		a.TimeBuckets = 24
	}
	if a.PriceBuckets == 0 {
		a.PriceBuckets = 20
	}
	if a.Hours < 1 || a.Hours > 168 || a.TimeBuckets < 4 || a.TimeBuckets > 96 || a.PriceBuckets < 5 || a.PriceBuckets > 60 {
		return errors.BadRequest("market_liquidation_range_invalid", "Liquidation heatmap range or bucket count is invalid")
	}
	return nil
}

type LiquidationHeatmapCell struct {
	TimeIndex     int     `json:"time_index"`
	PriceIndex    int     `json:"price_index"`
	LongNotional  float64 `json:"long_notional_usdt"`
	ShortNotional float64 `json:"short_notional_usdt"`
	EventCount    int     `json:"event_count"`
}

type LiquidationHeatmap struct {
	Exchange       string                   `json:"exchange"`
	Market         string                   `json:"market"`
	Symbol         string                   `json:"symbol"`
	From           time.Time                `json:"from"`
	To             time.Time                `json:"to"`
	MinPrice       float64                  `json:"min_price"`
	MaxPrice       float64                  `json:"max_price"`
	TimeBuckets    int                      `json:"time_buckets"`
	PriceBuckets   int                      `json:"price_buckets"`
	Cells          []LiquidationHeatmapCell `json:"cells"`
	TotalNotional  float64                  `json:"total_notional_usdt"`
	EventCount     int                      `json:"event_count"`
	DataAvailable  bool                     `json:"data_available"`
	Source         string                   `json:"source"`
	CoverageNotice string                   `json:"coverage_notice"`
}
