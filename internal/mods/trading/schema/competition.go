package schema

import (
	"strings"
	"time"
	"unicode"

	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

type CompetitionParticipationForm struct {
	Participating           bool   `json:"participating"`
	PublicAlias             string `json:"public_alias" binding:"max=32"`
	ConfirmPublicDisclosure bool   `json:"confirm_public_disclosure"`
}

type CompetitionParticipationState struct {
	Participating bool       `json:"participating"`
	PublicID      string     `json:"public_id"`
	PublicAlias   string     `json:"public_alias"`
	JoinedAt      *time.Time `json:"joined_at"`
}

func (a *CompetitionParticipationForm) Validate() error {
	a.PublicAlias = strings.TrimSpace(a.PublicAlias)
	if !a.Participating {
		return nil
	}
	if !a.ConfirmPublicDisclosure {
		return errors.BadRequest("trading_competition_consent_required", "Joining the competition requires explicit public disclosure consent")
	}
	if a.PublicAlias != "" {
		length := 0
		for _, value := range a.PublicAlias {
			if unicode.IsControl(value) {
				return errors.BadRequest("trading_competition_alias_invalid", "Public alias cannot contain control characters")
			}
			length++
		}
		if length < 2 || length > 32 {
			return errors.BadRequest("trading_competition_alias_invalid", "Public alias must contain 2 to 32 characters")
		}
	}
	return nil
}

type CompetitionQueryParam struct {
	util.PaginationParam
	Days int `form:"days" binding:"omitempty,min=1,max=365"`
}

type CompetitionTrader struct {
	PublicID       string     `json:"public_id"`
	PublicAlias    string     `json:"public_alias"`
	Rank           int        `json:"rank"`
	IsSimulation   bool       `json:"is_simulation"`
	TotalTrades    int64      `json:"total_trades"`
	WinningTrades  int64      `json:"winning_trades"`
	WinRate        float64    `json:"win_rate"`
	ReturnPct      float64    `json:"return_pct"`
	MaxDrawdownPct float64    `json:"max_drawdown_pct"`
	EquityIndex    float64    `json:"equity_index"`
	JoinedAt       *time.Time `json:"joined_at"`
	LastUpdatedAt  *time.Time `json:"last_updated_at"`
}

type CompetitionLeaderboard struct {
	PeriodStart time.Time              `json:"period_start"`
	PeriodEnd   time.Time              `json:"period_end"`
	Items       []*CompetitionTrader   `json:"items"`
	PageResult  *util.PaginationResult `json:"page"`
}

type CompetitionEquityPoint struct {
	CapturedAt  time.Time `json:"captured_at"`
	EquityIndex float64   `json:"equity_index"`
	ReturnPct   float64   `json:"return_pct"`
}

type CompetitionEquityHistory struct {
	Trader      *CompetitionTrader        `json:"trader"`
	PeriodStart time.Time                 `json:"period_start"`
	PeriodEnd   time.Time                 `json:"period_end"`
	Points      []*CompetitionEquityPoint `json:"points"`
}
