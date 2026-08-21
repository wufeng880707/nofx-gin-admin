package runtime

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPersistentSchedulerExecutesDueOccurrenceOnlyOnce(t *testing.T) {
	db := newSchedulerTestDB(t)
	now := time.Now().UTC()
	due := now.Add(-time.Second)
	task := newSchedulerTestTask(due)
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	first := &Registry{DB: db, InstanceID: "instance-a", taskHandlerHook: func(context.Context, string, string) error {
		calls.Add(1)
		return nil
	}}
	second := &Registry{DB: db, InstanceID: "instance-b", taskHandlerHook: first.taskHandlerHook}
	first.dispatchDueScheduledTasks(context.Background())
	first.persistentTaskWG.Wait()
	second.dispatchDueScheduledTasks(context.Background())
	second.persistentTaskWG.Wait()
	if calls.Load() != 1 {
		t.Fatalf("due occurrence executed %d times", calls.Load())
	}
	var runs []*schema.ScheduledTaskRun
	if err := db.Find(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Status != schema.TaskRunSuccess || runs[0].Attempts != 1 {
		t.Fatalf("unexpected runs: %#v", runs)
	}
	var persisted schema.ScheduledTask
	if err := db.First(&persisted, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.NextRunAt == nil || !persisted.NextRunAt.After(now) || persisted.LeaseUntil != nil {
		t.Fatalf("task state was not advanced and released: %#v", persisted)
	}
}

func TestPersistentSchedulerRetriesFiniteAttempts(t *testing.T) {
	db := newSchedulerTestDB(t)
	task := newSchedulerTestTask(time.Now().UTC().Add(time.Hour))
	task.Status = schema.TaskStatusPaused
	task.MaxRetries = 1
	task.RetryIntervalSeconds = 0
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	registry := &Registry{DB: db, InstanceID: "instance-a", taskHandlerHook: func(context.Context, string, string) error {
		if calls.Add(1) == 1 {
			return errors.New("temporary failure")
		}
		return nil
	}}
	run, err := registry.TriggerScheduledTask(context.Background(), task.ID, task.OwnerUserID, "operator-1")
	if err != nil {
		t.Fatal(err)
	}
	registry.persistentTaskWG.Wait()
	if err := db.First(run, "id = ?", run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || run.Status != schema.TaskRunSuccess || run.Attempts != 2 {
		t.Fatalf("unexpected retry result: calls=%d run=%#v", calls.Load(), run)
	}
	var audit schema.ScheduledTaskAudit
	if err := db.First(&audit, "task_id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if audit.Action != schema.TaskAuditTrigger || audit.ActorUserID != "operator-1" || audit.OwnerUserID != task.OwnerUserID {
		t.Fatalf("unexpected manual trigger audit: %#v", audit)
	}
}

func TestPersistentSchedulerAppliesMisfireAndConcurrencyPolicies(t *testing.T) {
	db := newSchedulerTestDB(t)
	old := time.Now().UTC().Add(-2 * time.Minute)
	misfired := newSchedulerTestTask(old)
	misfired.Name = "Misfired"
	misfired.MisfirePolicy = schema.TaskMisfireSkip
	leased := newSchedulerTestTask(time.Now().UTC().Add(-time.Second))
	leased.Name = "Leased"
	leaseUntil := time.Now().UTC().Add(time.Minute)
	leased.LeaseOwner, leased.LeaseUntil = "other-instance", &leaseUntil
	if err := db.Create([]*schema.ScheduledTask{misfired, leased}).Error; err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	registry := &Registry{DB: db, InstanceID: "instance-a", taskHandlerHook: func(context.Context, string, string) error {
		calls.Add(1)
		return nil
	}}
	registry.dispatchDueScheduledTasks(context.Background())
	registry.persistentTaskWG.Wait()
	if calls.Load() != 0 {
		t.Fatalf("skipped tasks executed %d times", calls.Load())
	}
	var runs []*schema.ScheduledTaskRun
	if err := db.Order("task_name ASC").Find(&runs).Error; err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].Status != schema.TaskRunSkipped || runs[1].Status != schema.TaskRunSkipped {
		t.Fatalf("unexpected skipped runs: %#v", runs)
	}
}

func newSchedulerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(new(schema.ScheduledTask), new(schema.ScheduledTaskRun), new(schema.ScheduledTaskAudit)); err != nil {
		t.Fatal(err)
	}
	return db
}

func newSchedulerTestTask(nextRun time.Time) *schema.ScheduledTask {
	return &schema.ScheduledTask{ID: uuid.NewString(), OwnerUserID: "root", Name: uuid.NewString(),
		Handler: schema.TaskHandlerAccountSync, CronExpression: "*/5 * * * *", TimeZone: "UTC",
		Status: schema.TaskStatusActive, TimeoutSeconds: 1, RetryIntervalSeconds: 1,
		MisfirePolicy: schema.TaskMisfireFireOnce, ConcurrencyPolicy: schema.TaskConcurrencyForbid,
		NextRunAt: &nextRun, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
}
