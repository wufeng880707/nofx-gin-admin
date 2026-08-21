package schema

import (
	"testing"
	"time"
)

func TestScheduledTaskFormValidatesWhitelistAndSchedule(t *testing.T) {
	form := &ScheduledTaskForm{Name: "Account Sync", Handler: TaskHandlerAccountSync,
		CronExpression: "*/5 * * * *", TimeZone: "Asia/Shanghai", TimeoutSeconds: 60,
		RetryIntervalSeconds: 5, MisfirePolicy: TaskMisfireFireOnce, ConcurrencyPolicy: TaskConcurrencyForbid}
	if err := form.Validate(); err != nil {
		t.Fatal(err)
	}
	next, err := NextScheduledTaskRun(form.CronExpression, form.TimeZone,
		time.Date(2026, 8, 19, 0, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if next.IsZero() {
		t.Fatal("next run was not calculated")
	}
}

func TestScheduledTaskFormRejectsArbitraryHandler(t *testing.T) {
	form := &ScheduledTaskForm{Name: "Unsafe", Handler: "shell", CronExpression: "* * * * *",
		TimeZone: "UTC", TimeoutSeconds: 60, RetryIntervalSeconds: 5,
		MisfirePolicy: TaskMisfireSkip, ConcurrencyPolicy: TaskConcurrencyForbid}
	if err := form.Validate(); err == nil {
		t.Fatal("unsafe handler was accepted")
	}
}
