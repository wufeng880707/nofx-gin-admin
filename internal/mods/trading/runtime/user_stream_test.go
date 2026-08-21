package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/provider"
	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeUserDataStream struct{ event provider.UserDataEvent }

func (a *fakeUserDataStream) Run(ctx context.Context, handle func(provider.UserDataEvent) error) error {
	if err := handle(a.event); err != nil {
		return err
	}
	// Duplicate delivery must remain idempotent by exchange and payload hash.
	if err := handle(a.event); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestUserDataStreamPersistsMetadataAndStops(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.UserStreamState), new(schema.UserStreamEvent)); err != nil {
		t.Fatal(err)
	}
	exchange := &schema.Exchange{ID: "exchange-stream", OwnerUserID: "owner-a", ExchangeType: "binance",
		AccountName: "main", Name: "Main", AccountType: "futures", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(exchange).Error; err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(db, &provider.Factory{}, nil)
	registry.userStreamBuildHook = func(*schema.Exchange) (provider.UserDataStream, error) {
		return &fakeUserDataStream{event: provider.UserDataEvent{Type: "ORDER_TRADE_UPDATE",
			EventTime: time.UnixMilli(1728972148778).UTC(), PayloadHash: "hash-a"}}, nil
	}
	if err := registry.StartUserDataStreams(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var count int64
		if err := db.Model(new(schema.UserStreamEvent)).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for user stream event, count=%d", count)
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := registry.StopUserDataStreams(ctx); err != nil {
		t.Fatal(err)
	}
	var state schema.UserStreamState
	if err := db.First(&state, "exchange_id = ?", exchange.ID).Error; err != nil {
		t.Fatal(err)
	}
	if state.Status != schema.UserStreamStatusStopped || state.LastEventAt == nil {
		t.Fatalf("unexpected final stream state: %#v", state)
	}
}

func TestUserDataStreamSupervisorStartsAndStopsUpdatedAccounts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Exchange), new(schema.UserStreamState), new(schema.UserStreamEvent)); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(db, &provider.Factory{}, nil)
	registry.userStreamRefresh = 20 * time.Millisecond
	started := make(chan string, 4)
	registry.userStreamBuildHook = func(exchange *schema.Exchange) (provider.UserDataStream, error) {
		started <- exchange.ID
		return &fakeUserDataStream{event: provider.UserDataEvent{Type: "ACCOUNT_UPDATE", EventTime: time.Now().UTC(),
			PayloadHash: exchange.ID + "-" + exchange.UpdatedAt.Format(time.RFC3339Nano)}}, nil
	}
	if err := registry.StartUserDataStreams(); err != nil {
		t.Fatal(err)
	}
	exchange := &schema.Exchange{ID: "dynamic-exchange", OwnerUserID: "owner-a", ExchangeType: "binance",
		AccountName: "dynamic", Name: "Dynamic", AccountType: "spot", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.Create(exchange).Error; err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-started:
		if id != exchange.ID {
			t.Fatalf("unexpected started exchange %q", id)
		}
	case <-time.After(time.Second):
		t.Fatal("dynamic exchange stream did not start")
	}
	var updateErr error
	for attempt := 0; attempt < 100; attempt++ {
		updateErr = db.Model(exchange).Updates(map[string]interface{}{"enabled": false, "updated_at": time.Now().Add(time.Second)}).Error
		if updateErr == nil || !strings.Contains(strings.ToLower(updateErr.Error()), "locked") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if updateErr != nil {
		t.Fatal(updateErr)
	}
	deadline := time.Now().Add(time.Second)
	for {
		var state schema.UserStreamState
		if err := db.First(&state, "exchange_id = ?", exchange.ID).Error; err == nil && state.Status == schema.UserStreamStatusStopped {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disabled exchange stream did not stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := registry.StopUserDataStreams(ctx); err != nil {
		t.Fatal(err)
	}
}
