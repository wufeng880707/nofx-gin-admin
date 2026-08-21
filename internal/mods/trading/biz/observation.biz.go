package biz

import (
	"context"
	"encoding/json"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
)

type Observation struct {
	ObservationDAL *dal.Observation
}

func (a *Observation) QueryUserStreamStates(ctx context.Context) ([]*schema.UserStreamState, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	return a.ObservationDAL.QueryUserStreamStates(ctx, owner)
}

func (a *Observation) QueryUserStreamEvents(ctx context.Context, params schema.UserStreamEventQueryParam) (*schema.UserStreamEventQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID = owner
	return a.ObservationDAL.QueryUserStreamEvents(ctx, params)
}

func (a *Observation) GetDecision(ctx context.Context, id string) (*schema.DecisionDetailView, error) {
	owner, err := resolveOwnerScope(ctx, "")
	if err != nil {
		return nil, err
	}
	item, err := a.ObservationDAL.GetDecision(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.NotFound("trading_decision_not_found", "Trading decision not found")
	}
	return &schema.DecisionDetailView{
		DecisionView: buildDecisionView(item),
		InputPrompt:  item.InputPrompt,
		RawResponse:  item.RawResponse,
		Decision:     safeJSON(item.DecisionJSON, "{}"),
	}, nil
}

func (a *Observation) QueryAssetSummary(ctx context.Context, ownerUserID string) (*schema.AssetSummary, error) {
	owner, err := resolveOwnerScope(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	return a.ObservationDAL.QueryAssetSummary(ctx, owner)
}

func (a *Observation) QueryAIInvocationSummary(ctx context.Context, params schema.AIInvocationSummaryQueryParam) (*schema.AIInvocationSummary, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID = owner
	now := time.Now().UTC()
	if params.EndAt == nil {
		params.EndAt = &now
	}
	if params.StartAt == nil {
		start := params.EndAt.Add(-30 * 24 * time.Hour)
		params.StartAt = &start
	}
	if params.StartAt.After(*params.EndAt) || params.EndAt.Sub(*params.StartAt) > 366*24*time.Hour {
		return nil, errors.BadRequest("trading_ai_summary_range_invalid", "AI observation range must be ordered and no longer than 366 days")
	}
	items, err := a.ObservationDAL.ListAIInvocations(ctx, params)
	if err != nil {
		return nil, err
	}
	return buildAIInvocationSummary(items, *params.StartAt, *params.EndAt), nil
}

type aiMetricAccumulator struct {
	metric     *schema.AIInvocationMetric
	latencySum int64
	cost       *big.Rat
}

func buildAIInvocationSummary(items []*schema.AIInvocation, startAt, endAt time.Time) *schema.AIInvocationSummary {
	overall := newAIAccumulator("all")
	byModel, byTrader, byStrategy := map[string]*aiMetricAccumulator{}, map[string]*aiMetricAccumulator{}, map[string]*aiMetricAccumulator{}
	for _, item := range items {
		addAIInvocation(overall, item)
		addAIInvocation(groupAccumulator(byModel, item.AIModelID), item)
		addAIInvocation(groupAccumulator(byTrader, valueOrUnassigned(item.TraderID)), item)
		addAIInvocation(groupAccumulator(byStrategy, valueOrUnassigned(item.StrategyID)), item)
	}
	return &schema.AIInvocationSummary{StartAt: startAt, EndAt: endAt, Overall: finishAIAccumulator(overall),
		ByModel: finishAIGroups(byModel), ByTrader: finishAIGroups(byTrader), ByStrategy: finishAIGroups(byStrategy)}
}

func newAIAccumulator(key string) *aiMetricAccumulator {
	return &aiMetricAccumulator{metric: &schema.AIInvocationMetric{Key: key}, cost: new(big.Rat)}
}

func groupAccumulator(groups map[string]*aiMetricAccumulator, key string) *aiMetricAccumulator {
	if groups[key] == nil {
		groups[key] = newAIAccumulator(key)
	}
	return groups[key]
}

func addAIInvocation(target *aiMetricAccumulator, item *schema.AIInvocation) {
	target.metric.Calls++
	if item.Success {
		target.metric.Successes++
	} else {
		target.metric.Failures++
	}
	target.metric.InputTokens += int64(item.InputTokens)
	target.metric.OutputTokens += int64(item.OutputTokens)
	target.metric.TotalTokens += int64(item.TotalTokens)
	target.latencySum += item.DurationMS
	if cost, ok := new(big.Rat).SetString(strings.TrimSpace(item.EstimatedCostUSD)); ok {
		target.cost.Add(target.cost, cost)
	}
}

func finishAIAccumulator(target *aiMetricAccumulator) *schema.AIInvocationMetric {
	if target.metric.Calls > 0 {
		target.metric.SuccessRate = float64(target.metric.Successes) / float64(target.metric.Calls) * 100
		target.metric.AverageLatencyMS = float64(target.latencySum) / float64(target.metric.Calls)
	}
	target.metric.EstimatedCostUSD = strings.TrimRight(strings.TrimRight(target.cost.FloatString(8), "0"), ".")
	if target.metric.EstimatedCostUSD == "" {
		target.metric.EstimatedCostUSD = "0"
	}
	return target.metric
}

func finishAIGroups(groups map[string]*aiMetricAccumulator) []*schema.AIInvocationMetric {
	result := make([]*schema.AIInvocationMetric, 0, len(groups))
	for _, item := range groups {
		result = append(result, finishAIAccumulator(item))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].TotalTokens > result[j].TotalTokens })
	return result
}

func valueOrUnassigned(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "unassigned"
	}
	return *value
}

func (a *Observation) QueryAccountSnapshots(ctx context.Context, params schema.ObservationQueryParam) (*schema.AccountSnapshotQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.ObservationDAL.QueryAccountSnapshots(ctx, params)
}

func (a *Observation) QueryPositions(ctx context.Context, params schema.PositionQueryParam) (*schema.PositionQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.ObservationDAL.QueryPositions(ctx, params)
}

func (a *Observation) QueryAssetBalances(ctx context.Context, params schema.AssetBalanceQueryParam) (*schema.AssetBalanceQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.ObservationDAL.QueryAssetBalances(ctx, params)
}

func (a *Observation) QueryOrders(ctx context.Context, params schema.OrderQueryParam) (*schema.OrderQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.ObservationDAL.QueryOrders(ctx, params)
}

func (a *Observation) QueryFills(ctx context.Context, params schema.FillQueryParam) (*schema.FillQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.ObservationDAL.QueryFills(ctx, params)
}

func (a *Observation) QueryIncomeEvents(ctx context.Context, params schema.IncomeEventQueryParam) (*schema.IncomeEventQueryResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	return a.ObservationDAL.QueryIncomeEvents(ctx, params)
}

func (a *Observation) QueryDecisions(ctx context.Context, params schema.DecisionQueryParam) (*schema.DecisionViewResult, error) {
	owner, err := resolveOwnerScope(ctx, params.OwnerUserID)
	if err != nil {
		return nil, err
	}
	params.OwnerUserID, params.Pagination = owner, true
	result, err := a.ObservationDAL.QueryDecisions(ctx, params)
	if err != nil {
		return nil, err
	}
	data := make([]*schema.DecisionView, 0, len(result.Data))
	for _, item := range result.Data {
		data = append(data, buildDecisionView(item))
	}
	return &schema.DecisionViewResult{Data: data, PageResult: result.PageResult}, nil
}

func buildDecisionView(item *schema.Decision) *schema.DecisionView {
	return &schema.DecisionView{
		ID: item.ID, OwnerUserID: item.OwnerUserID, TraderID: item.TraderID,
		ExchangeID: item.ExchangeID, CycleNumber: item.CycleNumber, DecidedAt: item.DecidedAt, Success: item.Success,
		ErrorMessage: item.ErrorMessage, AIRequestDurationMs: item.AIRequestDurationMs,
		CandidateCoins: safeJSON(item.CandidateCoinsJSON, "[]"),
		ExecutionLog:   safeJSON(item.ExecutionLogJSON, "[]"), Actions: safeJSON(item.ActionsJSON, "[]"),
		CreatedAt: item.CreatedAt,
	}
}

func safeJSON(value, fallback string) json.RawMessage {
	if !json.Valid([]byte(value)) {
		value = fallback
	}
	return json.RawMessage(value)
}
