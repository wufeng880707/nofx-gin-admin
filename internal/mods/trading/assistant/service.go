package assistant

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/gorm"
)

const helpText = `XTrading 只读助手
/assets 资产概览
/positions 当前仓位
/traders Trader 状态
/strategies 策略状态
/diagnose_exchange 交易所诊断
/diagnose_model 模型诊断
/diagnose_trader Trader 诊断
/diagnose_order 订单诊断
/diagnose_strategy 策略诊断
/unbind 解除 Telegram 绑定

助手不会创建订单、审批交易或修改配置。`

type Response struct {
	Skill string
	Text  string
}

// Service is a deterministic, owner-scoped task skill dispatcher. It exposes
// no generic HTTP or write tool and therefore cannot submit trading commands.
type Service struct {
	DB *gorm.DB
}

func NewService(db *gorm.DB) *Service { return &Service{DB: db} }

func (a *Service) Execute(ctx context.Context, ownerUserID, input string) (Response, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return Response{}, fmt.Errorf("owner user id is required")
	}
	command := normalizeCommand(input)
	switch command {
	case "help":
		return Response{Skill: "help", Text: helpText}, nil
	case "assets":
		return a.assets(ctx, ownerUserID)
	case "positions":
		return a.positions(ctx, ownerUserID)
	case "traders":
		return a.traders(ctx, ownerUserID)
	case "strategies":
		return a.strategies(ctx, ownerUserID)
	case "diagnose_exchange":
		return a.diagnoseExchanges(ctx, ownerUserID)
	case "diagnose_model":
		return a.diagnoseModels(ctx, ownerUserID)
	case "diagnose_trader":
		return a.diagnoseTraders(ctx, ownerUserID)
	case "diagnose_order":
		return a.diagnoseOrders(ctx, ownerUserID)
	case "diagnose_strategy":
		return a.diagnoseStrategies(ctx, ownerUserID)
	default:
		return Response{Skill: "help", Text: "未识别的只读任务。\n\n" + helpText}, nil
	}
}

func normalizeCommand(input string) string {
	value := strings.TrimSpace(strings.ToLower(input))
	if index := strings.IndexAny(value, " \t\r\n"); index >= 0 {
		value = value[:index]
	}
	value = strings.TrimPrefix(value, "/")
	if index := strings.Index(value, "@"); index >= 0 {
		value = value[:index]
	}
	switch value {
	case "start", "帮助", "菜单":
		return "help"
	case "资产", "余额", "balance":
		return "assets"
	case "仓位", "持仓":
		return "positions"
	case "trader", "机器人":
		return "traders"
	case "策略":
		return "strategies"
	}
	return value
}

func (a *Service) assets(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.AssetBalance
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ?", owner).Order("value_usdt DESC").Limit(100).Find(&rows).Error; err != nil {
		return Response{}, err
	}
	total := 0.0
	lines := make([]string, 0, 7)
	for _, row := range rows {
		value := decimalFloat(row.ValueUSDT)
		if value <= 0 && decimalFloat(row.Total) == 0 {
			continue
		}
		total += value
		if len(lines) < 5 {
			lines = append(lines, fmt.Sprintf("%s: %s (%.2f USDT)", row.Asset, trimDecimal(row.Total), value))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "暂无非零资产；请先执行交易所同步。")
	}
	return Response{Skill: "assets", Text: fmt.Sprintf("资产概览\n总价值: %.2f USDT\n%s", total, strings.Join(lines, "\n"))}, nil
}

func (a *Service) positions(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.Position
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ? AND status = ?", owner, schema.PositionStatusOpen).
		Order("updated_at DESC").Limit(20).Find(&rows).Error; err != nil {
		return Response{}, err
	}
	if len(rows) == 0 {
		return Response{Skill: "positions", Text: "当前没有未平仓仓位。"}, nil
	}
	lines := []string{fmt.Sprintf("当前仓位: %d", len(rows))}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%s %s | 数量 %s | 标记 %s | 浮盈亏 %s USDT", row.Symbol,
			strings.ToUpper(row.Side), trimDecimal(row.Quantity), trimDecimal(row.MarkPrice), trimDecimal(row.UnrealizedPnL)))
	}
	return Response{Skill: "positions", Text: strings.Join(lines, "\n")}, nil
}

func (a *Service) traders(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.Trader
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ?", owner).Order("updated_at DESC").Limit(20).Find(&rows).Error; err != nil {
		return Response{}, err
	}
	if len(rows) == 0 {
		return Response{Skill: "traders", Text: "尚未创建 Trader。"}, nil
	}
	lines := []string{fmt.Sprintf("Trader: %d", len(rows))}
	for _, row := range rows {
		mode := "实盘"
		if row.IsSimulation {
			mode = "模拟"
		}
		lines = append(lines, fmt.Sprintf("%s | %s/%s | %s | 安全模式=%t", row.Name, row.DesiredState, row.RuntimeState, mode, row.SafeMode))
	}
	return Response{Skill: "traders", Text: strings.Join(lines, "\n")}, nil
}

func (a *Service) strategies(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.Strategy
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ?", owner).Order("updated_at DESC").Limit(20).Find(&rows).Error; err != nil {
		return Response{}, err
	}
	if len(rows) == 0 {
		return Response{Skill: "strategies", Text: "尚未创建策略。"}, nil
	}
	lines := []string{fmt.Sprintf("策略: %d", len(rows))}
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("%s | %s | active=%t | 当前版本=%d | 发布版本=%s", row.Name,
			row.StrategyType, row.IsActive, row.CurrentVersion, optionalInt(row.PublishedVersion)))
	}
	return Response{Skill: "strategies", Text: strings.Join(lines, "\n")}, nil
}

func (a *Service) diagnoseExchanges(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.Exchange
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ?", owner).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return Response{}, err
	}
	if len(rows) == 0 {
		return Response{Skill: "diagnose_exchange", Text: "交易所诊断: 未配置交易所账户。"}, nil
	}
	lines := []string{"交易所诊断"}
	for _, row := range rows {
		issues := make([]string, 0, 3)
		if !row.Enabled {
			issues = append(issues, "未启用")
		}
		if row.APIKeyCiphertext == "" || row.SecretKeyCiphertext == "" {
			issues = append(issues, "API 凭证不完整")
		}
		if row.ExchangeType == "okx" && row.PassphraseCiphertext == "" {
			issues = append(issues, "缺少 Passphrase")
		}
		if len(issues) == 0 {
			issues = append(issues, "基础配置完整；连接状态请执行账户测试")
		}
		lines = append(lines, fmt.Sprintf("%s/%s: %s", row.Name, row.AccountType, strings.Join(issues, "，")))
	}
	return Response{Skill: "diagnose_exchange", Text: strings.Join(lines, "\n")}, nil
}

func (a *Service) diagnoseModels(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.AIModel
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ?", owner).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return Response{}, err
	}
	if len(rows) == 0 {
		return Response{Skill: "diagnose_model", Text: "模型诊断: 未配置 AI 模型。"}, nil
	}
	lines := []string{"模型诊断"}
	for _, row := range rows {
		status := "基础配置完整；真实连通性需执行模型测试"
		if !row.Enabled {
			status = "未启用"
		} else if row.APIKeyCiphertext == "" {
			status = "缺少 API Key"
		} else if row.Provider == "custom" && (row.CustomAPIURL == "" || row.CustomModelName == "") {
			status = "自定义 URL 或模型名不完整"
		}
		lines = append(lines, fmt.Sprintf("%s/%s: %s", row.Name, row.Provider, status))
	}
	return Response{Skill: "diagnose_model", Text: strings.Join(lines, "\n")}, nil
}

func (a *Service) diagnoseTraders(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.Trader
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ?", owner).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return Response{}, err
	}
	if len(rows) == 0 {
		return Response{Skill: "diagnose_trader", Text: "Trader 诊断: 未创建 Trader。"}, nil
	}
	lines := []string{"Trader 诊断"}
	for _, row := range rows {
		issues := make([]string, 0, 4)
		if row.LastErrorCode != "" {
			issues = append(issues, row.LastErrorCode+": "+row.LastErrorMessage)
		}
		if row.SafeMode {
			issues = append(issues, "安全模式已开启")
		}
		if row.PauseReason != "" {
			issues = append(issues, "暂停原因="+row.PauseReason)
		}
		if row.ConsecutiveFailures > 0 {
			issues = append(issues, fmt.Sprintf("连续失败=%d", row.ConsecutiveFailures))
		}
		if len(issues) == 0 {
			issues = append(issues, "未发现持久化运行错误")
		}
		lines = append(lines, fmt.Sprintf("%s [%s/%s]: %s", row.Name, row.DesiredState, row.RuntimeState, strings.Join(issues, "；")))
	}
	return Response{Skill: "diagnose_trader", Text: strings.Join(lines, "\n")}, nil
}

func (a *Service) diagnoseOrders(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.ExchangeOrder
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ?", owner).Order("exchange_updated_at DESC").Limit(50).Find(&rows).Error; err != nil {
		return Response{}, err
	}
	if len(rows) == 0 {
		return Response{Skill: "diagnose_order", Text: "订单诊断: 暂无订单；请先执行历史回补或账户同步。"}, nil
	}
	counts := make(map[string]int)
	for _, row := range rows {
		counts[strings.ToLower(row.Status)]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return Response{Skill: "diagnose_order", Text: "订单诊断（最近 50 条）\n" + strings.Join(parts, "，") +
		"\n若订单缺失，检查账户类型、历史币种清单、同步游标和 API 读取权限。"}, nil
}

func (a *Service) diagnoseStrategies(ctx context.Context, owner string) (Response, error) {
	var rows []*schema.Strategy
	if err := a.DB.WithContext(ctx).Where("owner_user_id = ?", owner).Order("updated_at DESC").Find(&rows).Error; err != nil {
		return Response{}, err
	}
	if len(rows) == 0 {
		return Response{Skill: "diagnose_strategy", Text: "策略诊断: 未创建策略。"}, nil
	}
	lines := []string{"策略诊断"}
	for _, row := range rows {
		issues := make([]string, 0, 3)
		if !row.IsActive {
			issues = append(issues, "未激活")
		}
		if row.CurrentVersion == 0 {
			issues = append(issues, "尚无版本")
		}
		if row.PublishedVersion == nil {
			issues = append(issues, "尚未发布")
		}
		if strings.TrimSpace(row.ConfigJSON) == "" || strings.TrimSpace(row.ConfigJSON) == "{}" {
			issues = append(issues, "配置为空")
		}
		if len(issues) == 0 {
			issues = append(issues, "基础状态正常；Prompt 生效需查看最近决策版本")
		}
		lines = append(lines, fmt.Sprintf("%s: %s", row.Name, strings.Join(issues, "，")))
	}
	return Response{Skill: "diagnose_strategy", Text: strings.Join(lines, "\n")}, nil
}

func decimalFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func trimDecimal(value string) string {
	value = strings.TrimSpace(value)
	if !strings.Contains(value, ".") {
		if value == "" {
			return "0"
		}
		return value
	}
	value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	if value == "" || value == "-0" {
		return "0"
	}
	return value
}

func optionalInt(value *int) string {
	if value == nil {
		return "未发布"
	}
	return strconv.Itoa(*value)
}
