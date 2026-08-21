package trading

import (
	"context"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsureScheduledTaskIsIdempotentAndDoesNotOverwrite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:scheduled-task-default?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.ScheduledTask)); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	task := &schema.ScheduledTask{
		ID: "default-task", OwnerUserID: "root", Name: "账户数据同步",
		Handler: schema.TaskHandlerAccountSync, CronExpression: "@every 5m", TimeZone: "UTC",
		Status: schema.TaskStatusActive, TimeoutSeconds: 60, MaxRetries: 2,
		RetryIntervalSeconds: 5, MisfirePolicy: schema.TaskMisfireFireOnce,
		ConcurrencyPolicy: schema.TaskConcurrencyForbid, NextRunAt: &now,
	}
	ctx := context.Background()
	if err := ensureScheduledTask(ctx, db, task); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(new(schema.ScheduledTask)).Where("id = ?", task.ID).
		Updates(map[string]interface{}{"status": schema.TaskStatusPaused, "next_run_at": nil}).Error; err != nil {
		t.Fatal(err)
	}
	second := *task
	second.ID = "another-default-task-id"
	if err := ensureScheduledTask(ctx, db, &second); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(new(schema.ScheduledTask)).Where("owner_user_id = ? AND name = ?", "root", task.Name).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("default task duplicated: %d", count)
	}
	var stored schema.ScheduledTask
	if err := db.First(&stored, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != schema.TaskStatusPaused || stored.NextRunAt != nil {
		t.Fatalf("existing task was overwritten: %#v", stored)
	}
}
