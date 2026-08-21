package biz

import (
	"context"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
)

const competitionParticipantLimit = 1000

type Competition struct {
	CompetitionDAL *dal.Competition
	CatalogDAL     *dal.Catalog
}

func (a *Competition) GetParticipation(ctx context.Context, traderID string) (*schema.CompetitionParticipationState, error) {
	item, err := a.getOwnedTrader(ctx, traderID)
	if err != nil {
		return nil, err
	}
	return competitionParticipationState(item), nil
}

func (a *Competition) SetParticipation(ctx context.Context, traderID string, form *schema.CompetitionParticipationForm) (*schema.CompetitionParticipationState, error) {
	if form == nil {
		return nil, errors.BadRequest("trading_competition_form_required", "Competition participation form is required")
	}
	if err := form.Validate(); err != nil {
		return nil, err
	}
	item, err := a.getOwnedTrader(ctx, traderID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if form.Participating {
		initial, ok := new(big.Rat).SetString(strings.TrimSpace(item.InitialBalance))
		if !ok || initial.Sign() <= 0 {
			return nil, errors.BadRequest("trading_competition_initial_balance_required", "Competition requires a positive initial balance")
		}
		if item.CompetitionPublicID == "" {
			item.CompetitionPublicID = util.MustNewUUID()
		}
		if form.PublicAlias != "" {
			item.CompetitionAlias = form.PublicAlias
		}
		if item.CompetitionAlias == "" {
			item.CompetitionAlias = competitionDefaultAlias(item.CompetitionPublicID)
		}
		if !item.ShowInCompetition || item.CompetitionJoinedAt == nil {
			item.CompetitionJoinedAt = &now
		}
		item.ShowInCompetition = true
	} else {
		item.ShowInCompetition = false
	}
	item.UpdatedAt = now
	if err := a.CompetitionDAL.UpdateParticipation(ctx, item); err != nil {
		return nil, err
	}
	return competitionParticipationState(item), nil
}

func (a *Competition) QueryLeaderboard(ctx context.Context, params schema.CompetitionQueryParam) (*schema.CompetitionLeaderboard, error) {
	params, startAt, endAt := normalizeCompetitionQuery(params)
	participants, err := a.CompetitionDAL.ListParticipants(ctx, competitionParticipantLimit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(participants))
	for _, item := range participants {
		ids = append(ids, item.ID)
	}
	trades, err := a.CompetitionDAL.ListClosedTrades(ctx, ids, startAt, endAt)
	if err != nil {
		return nil, err
	}
	byTrader := groupCompetitionTrades(trades)
	items := make([]*schema.CompetitionTrader, 0, len(participants))
	for _, participant := range participants {
		metric, _ := buildCompetitionMetric(participant, byTrader[participant.ID], startAt, false)
		items = append(items, metric)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ReturnPct != items[j].ReturnPct {
			return items[i].ReturnPct > items[j].ReturnPct
		}
		if items[i].MaxDrawdownPct != items[j].MaxDrawdownPct {
			return items[i].MaxDrawdownPct < items[j].MaxDrawdownPct
		}
		if items[i].TotalTrades != items[j].TotalTrades {
			return items[i].TotalTrades > items[j].TotalTrades
		}
		return items[i].PublicID < items[j].PublicID
	})
	for index := range items {
		items[index].Rank = index + 1
	}
	total := len(items)
	from := (params.Current - 1) * params.PageSize
	if from > total {
		from = total
	}
	to := from + params.PageSize
	if to > total {
		to = total
	}
	return &schema.CompetitionLeaderboard{
		PeriodStart: startAt,
		PeriodEnd:   endAt,
		Items:       items[from:to],
		PageResult:  &util.PaginationResult{Total: int64(total), Current: params.Current, PageSize: params.PageSize},
	}, nil
}

func (a *Competition) GetEquityHistory(ctx context.Context, publicID string, days int) (*schema.CompetitionEquityHistory, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" || len(publicID) > 36 {
		return nil, errors.NotFound("trading_competition_trader_not_found", "Competition trader not found")
	}
	_, startAt, endAt := normalizeCompetitionQuery(schema.CompetitionQueryParam{Days: days})
	participant, err := a.CompetitionDAL.GetParticipantByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	if participant == nil {
		return nil, errors.NotFound("trading_competition_trader_not_found", "Competition trader not found")
	}
	trades, err := a.CompetitionDAL.ListClosedTrades(ctx, []string{participant.ID}, startAt, endAt)
	if err != nil {
		return nil, err
	}
	metric, points := buildCompetitionMetric(participant, trades, startAt, true)
	return &schema.CompetitionEquityHistory{Trader: metric, PeriodStart: startAt, PeriodEnd: endAt, Points: points}, nil
}

func (a *Competition) getOwnedTrader(ctx context.Context, traderID string) (*schema.Trader, error) {
	owner, err := mutationOwnerFilter(ctx)
	if err != nil {
		return nil, err
	}
	item, err := a.CatalogDAL.GetTrader(ctx, traderID, owner)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.NotFound("trading_trader_not_found", "Trader not found")
	}
	return item, nil
}

func normalizeCompetitionQuery(params schema.CompetitionQueryParam) (schema.CompetitionQueryParam, time.Time, time.Time) {
	if params.Current < 1 {
		params.Current = 1
	}
	if params.PageSize < 1 || params.PageSize > 50 {
		params.PageSize = 20
	}
	if params.Days < 1 || params.Days > 365 {
		params.Days = 90
	}
	endAt := time.Now().UTC()
	startAt := endAt.AddDate(0, 0, -params.Days)
	return params, startAt, endAt
}

func groupCompetitionTrades(trades []*schema.ClosedTrade) map[string][]*schema.ClosedTrade {
	result := make(map[string][]*schema.ClosedTrade)
	for _, trade := range trades {
		result[trade.TraderID] = append(result[trade.TraderID], trade)
	}
	return result
}

func buildCompetitionMetric(participant *schema.Trader, trades []*schema.ClosedTrade, startAt time.Time, includePoints bool) (*schema.CompetitionTrader, []*schema.CompetitionEquityPoint) {
	initial, _ := new(big.Rat).SetString(strings.TrimSpace(participant.InitialBalance))
	if initial == nil || initial.Sign() <= 0 {
		initial = big.NewRat(1, 1)
	}
	equity := new(big.Rat).Set(initial)
	peak := new(big.Rat).Set(initial)
	metric := &schema.CompetitionTrader{
		PublicID: participant.CompetitionPublicID, PublicAlias: participant.CompetitionAlias,
		IsSimulation: participant.IsSimulation, EquityIndex: 100, JoinedAt: participant.CompetitionJoinedAt,
	}
	points := make([]*schema.CompetitionEquityPoint, 0, len(trades)+1)
	if includePoints {
		points = append(points, &schema.CompetitionEquityPoint{CapturedAt: startAt, EquityIndex: 100, ReturnPct: 0})
	}
	for _, trade := range trades {
		net := ratOrZero(trade.NetPnL)
		equity.Add(equity, net)
		metric.TotalTrades++
		if net.Sign() > 0 {
			metric.WinningTrades++
		}
		if equity.Cmp(peak) > 0 {
			peak.Set(equity)
		}
		if peak.Sign() > 0 {
			currentDrawdown, _ := new(big.Rat).Quo(new(big.Rat).Sub(peak, equity), peak).Float64()
			metric.MaxDrawdownPct = math.Max(metric.MaxDrawdownPct, currentDrawdown*100)
		}
		index, _ := new(big.Rat).Mul(new(big.Rat).Quo(equity, initial), big.NewRat(100, 1)).Float64()
		metric.EquityIndex = index
		metric.ReturnPct = index - 100
		closedAt := trade.ClosedAt
		metric.LastUpdatedAt = &closedAt
		if includePoints {
			points = append(points, &schema.CompetitionEquityPoint{CapturedAt: trade.ClosedAt, EquityIndex: index, ReturnPct: index - 100})
		}
	}
	if metric.TotalTrades > 0 {
		metric.WinRate = float64(metric.WinningTrades) / float64(metric.TotalTrades) * 100
	}
	if metric.LastUpdatedAt == nil {
		metric.LastUpdatedAt = participant.CompetitionJoinedAt
	}
	return metric, points
}

func competitionParticipationState(item *schema.Trader) *schema.CompetitionParticipationState {
	return &schema.CompetitionParticipationState{Participating: item.ShowInCompetition, PublicID: item.CompetitionPublicID,
		PublicAlias: item.CompetitionAlias, JoinedAt: item.CompetitionJoinedAt}
}

func competitionDefaultAlias(publicID string) string {
	compact := strings.ReplaceAll(publicID, "-", "")
	if len(compact) > 8 {
		compact = compact[len(compact)-8:]
	}
	return "Trader-" + strings.ToUpper(compact)
}
