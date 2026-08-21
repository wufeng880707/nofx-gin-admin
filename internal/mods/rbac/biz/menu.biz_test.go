package biz

import (
	"context"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/rbac/dal"
	"github.com/LyricTian/gin-admin/v10/internal/mods/rbac/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateLegacyHome(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:rbac-menu-migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(new(schema.Menu), new(schema.MenuResource), new(schema.RoleMenu)); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	now := time.Now()
	legacy := &schema.Menu{ID: "legacy-home", Code: "home", Name: "Home", Path: "/home", Status: schema.MenuStatusEnabled, CreatedAt: now}
	dashboard := &schema.Menu{ID: "dashboard", Code: "dashboard", Name: "Dashboard", Path: "/dashboard", Status: schema.MenuStatusEnabled, CreatedAt: now}
	workplace := &schema.Menu{ID: "workplace", Code: "workplace", Name: "Workplace", Path: "/dashboard/workplace", ParentID: dashboard.ID, ParentPath: dashboard.ID + ".", Status: schema.MenuStatusEnabled, CreatedAt: now}
	if err := db.Create([]*schema.Menu{legacy, dashboard, workplace}).Error; err != nil {
		t.Fatalf("seed menus: %v", err)
	}
	if err := db.Create([]*schema.RoleMenu{
		{ID: "legacy-ref", RoleID: "role-a", MenuID: legacy.ID, CreatedAt: now},
		{ID: "duplicate-ref", RoleID: "role-b", MenuID: legacy.ID, CreatedAt: now},
		{ID: "target-ref", RoleID: "role-b", MenuID: workplace.ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("seed role menus: %v", err)
	}

	menuDAL := &dal.Menu{DB: db}
	roleMenuDAL := &dal.RoleMenu{DB: db}
	biz := &Menu{
		MenuDAL:         menuDAL,
		MenuResourceDAL: &dal.MenuResource{DB: db},
		RoleMenuDAL:     roleMenuDAL,
	}
	configured := schema.Menus{
		&schema.Menu{Code: "dashboard", Children: &schema.Menus{&schema.Menu{Code: "workplace"}}},
	}
	if err := biz.migrateLegacyHome(context.Background(), configured); err != nil {
		t.Fatalf("migrate legacy home: %v", err)
	}

	if item, err := menuDAL.Get(context.Background(), legacy.ID); err != nil {
		t.Fatalf("query legacy home: %v", err)
	} else if item != nil {
		t.Fatal("legacy home menu was not deleted")
	}
	refs, err := roleMenuDAL.Query(context.Background(), schema.RoleMenuQueryParam{MenuID: workplace.ID})
	if err != nil {
		t.Fatalf("query workplace refs: %v", err)
	}
	if len(refs.Data) != 2 {
		t.Fatalf("expected 2 deduplicated workplace refs, got %d", len(refs.Data))
	}
}

func TestMigrateTradingDataMenus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:rbac-trading-data-migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.Menu), new(schema.MenuResource), new(schema.RoleMenu)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	trading := &schema.Menu{ID: "trading", Code: "trading", Name: "Trading", Status: schema.MenuStatusEnabled, CreatedAt: now}
	tradingData := &schema.Menu{ID: "trading-data", Code: "trading-data", Name: "Trading Data", Status: schema.MenuStatusEnabled, CreatedAt: now}
	legacy := &schema.Menu{ID: "legacy-orders", Code: "orders", Name: "Orders", ParentID: trading.ID, ParentPath: trading.ID + ".", Status: schema.MenuStatusEnabled, CreatedAt: now}
	moved := &schema.Menu{ID: "moved-orders", Code: "orders", Name: "Orders", ParentID: tradingData.ID, ParentPath: tradingData.ID + ".", Status: schema.MenuStatusEnabled, CreatedAt: now}
	if err := db.Create([]*schema.Menu{trading, tradingData, legacy, moved}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create([]*schema.RoleMenu{
		{ID: "legacy-a", RoleID: "role-a", MenuID: legacy.ID, CreatedAt: now},
		{ID: "legacy-b", RoleID: "role-b", MenuID: legacy.ID, CreatedAt: now},
		{ID: "moved-b", RoleID: "role-b", MenuID: moved.ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	menuDAL := &dal.Menu{DB: db}
	roleMenuDAL := &dal.RoleMenu{DB: db}
	menuBIZ := &Menu{MenuDAL: menuDAL, MenuResourceDAL: &dal.MenuResource{DB: db}, RoleMenuDAL: roleMenuDAL}
	configured := schema.Menus{
		&schema.Menu{Code: "trading"},
		&schema.Menu{Code: "trading-data", Children: &schema.Menus{&schema.Menu{Code: "orders"}}},
	}
	if err := menuBIZ.migrateTradingDataMenus(context.Background(), configured); err != nil {
		t.Fatal(err)
	}
	if item, err := menuDAL.Get(context.Background(), legacy.ID); err != nil || item != nil {
		t.Fatalf("legacy trading menu still exists: item=%#v err=%v", item, err)
	}
	refs, err := roleMenuDAL.Query(context.Background(), schema.RoleMenuQueryParam{MenuID: moved.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs.Data) != 2 {
		t.Fatalf("expected two deduplicated role refs, got %d", len(refs.Data))
	}
}
