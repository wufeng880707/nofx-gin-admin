package biz

import (
	"context"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCompetitionParticipationRequiresConsentAndOwnerScope(t *testing.T) {
	service, db := competitionTestService(t)
	trader := competitionTrader("trader-1", "user-1", "1000")
	if err := db.Create(trader).Error; err != nil {
		t.Fatal(err)
	}

	ownerCtx := util.NewUserID(context.Background(), "user-1")
	if _, err := service.SetParticipation(ownerCtx, trader.ID, &schema.CompetitionParticipationForm{Participating: true}); err == nil {
		t.Fatal("expected explicit disclosure consent to be required")
	}
	state, err := service.SetParticipation(ownerCtx, trader.ID, &schema.CompetitionParticipationForm{
		Participating: true, ConfirmPublicDisclosure: true, PublicAlias: "Alpha",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Participating || state.PublicID == "" || state.PublicAlias != "Alpha" || state.PublicID == trader.ID {
		t.Fatalf("unexpected participation state: %#v", state)
	}
	if _, err := service.GetParticipation(util.NewUserID(context.Background(), "user-2"), trader.ID); err == nil {
		t.Fatal("expected cross-owner participation lookup to fail")
	}

	if _, err := service.SetParticipation(ownerCtx, trader.ID, &schema.CompetitionParticipationForm{Participating: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.GetEquityHistory(context.Background(), state.PublicID, 90); err == nil {
		t.Fatal("withdrawn trader must disappear from public APIs")
	}
}

func TestCompetitionLeaderboardRanksNormalizedReturnsWithoutLegacyDisclosure(t *testing.T) {
	service, db := competitionTestService(t)
	now := time.Now().UTC()
	first := competitionTrader("trader-1", "user-1", "100")
	first.ShowInCompetition = true
	first.CompetitionPublicID = "11111111-1111-4111-8111-111111111111"
	first.CompetitionAlias = "First"
	first.CompetitionJoinedAt = &now
	second := competitionTrader("trader-2", "user-2", "100")
	second.ShowInCompetition = true
	second.CompetitionPublicID = "22222222-2222-4222-8222-222222222222"
	second.CompetitionAlias = "Second"
	second.CompetitionJoinedAt = &now
	legacy := competitionTrader("legacy", "user-3", "100")
	legacy.ShowInCompetition = true
	if err := db.Create([]*schema.Trader{first, second, legacy}).Error; err != nil {
		t.Fatal(err)
	}
	closedAt := now.Add(-time.Hour)
	trades := []*schema.ClosedTrade{
		competitionTrade("one-a", first.ID, "10", closedAt.Add(-time.Hour)),
		competitionTrade("one-b", first.ID, "-5", closedAt),
		competitionTrade("two-a", second.ID, "20", closedAt),
	}
	if err := db.Create(trades).Error; err != nil {
		t.Fatal(err)
	}

	leaderboard, err := service.QueryLeaderboard(context.Background(), schema.CompetitionQueryParam{
		PaginationParam: util.PaginationParam{Current: 1, PageSize: 1}, Days: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leaderboard.PageResult.Total != 2 || len(leaderboard.Items) != 1 {
		t.Fatalf("legacy opt-in without disclosure identity must stay private: %#v", leaderboard)
	}
	winner := leaderboard.Items[0]
	if winner.PublicID != second.CompetitionPublicID || winner.Rank != 1 || winner.ReturnPct != 20 || winner.EquityIndex != 120 {
		t.Fatalf("unexpected leaderboard winner: %#v", winner)
	}
	if winner.PublicAlias != "Second" || winner.TotalTrades != 1 || winner.WinRate != 100 {
		t.Fatalf("unexpected public metric: %#v", winner)
	}

	history, err := service.GetEquityHistory(context.Background(), first.CompetitionPublicID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Points) != 3 || history.Trader.ReturnPct != 5 || history.Trader.MaxDrawdownPct <= 0 {
		t.Fatalf("unexpected normalized equity history: %#v", history)
	}
}

func competitionTestService(t *testing.T) (*Competition, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Trader), new(schema.ClosedTrade)); err != nil {
		t.Fatal(err)
	}
	return &Competition{CompetitionDAL: &dal.Competition{DB: db}, CatalogDAL: &dal.Catalog{DB: db}}, db
}

func competitionTrader(id, owner, initial string) *schema.Trader {
	return &schema.Trader{ID: id, OwnerUserID: owner, Name: id, AIModelID: "model", ExchangeID: "exchange",
		InitialBalance: initial, ScanIntervalMinutes: 15, IsSimulation: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
}

func competitionTrade(id, traderID, net string, closedAt time.Time) *schema.ClosedTrade {
	return &schema.ClosedTrade{ID: id, OwnerUserID: "owner", TraderID: traderID, ExchangeID: "exchange", Market: "futures",
		Symbol: "BTCUSDT", Side: "long", EntryQuantity: "1", ExitQuantity: "1", EntryPrice: "100", ExitPrice: "100",
		GrossPnL: net, Fee: "0", Funding: "0", NetPnL: net, OpenedAt: closedAt.Add(-time.Hour), ClosedAt: closedAt, RebuiltAt: closedAt}
}
