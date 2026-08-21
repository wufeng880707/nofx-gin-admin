package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/mods/trading/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultPersistentSchedulerPollInterval = 5 * time.Second
	persistentTaskMisfireGrace             = 30 * time.Second
)

var (
	ErrScheduledTaskNotRunnable = errors.New("scheduled task is not runnable")
	ErrScheduledTaskConcurrent  = errors.New("scheduled task already has an active run")
)

func (a *Registry) StartPersistentTaskScheduler(pollInterval time.Duration) {
	if pollInterval <= 0 {
		pollInterval = defaultPersistentSchedulerPollInterval
	}
	a.persistentSchedulerMu.Lock()
	if a.persistentSchedulerCancel != nil {
		a.persistentSchedulerMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	a.persistentSchedulerCancel = cancel
	a.persistentSchedulerDone = done
	a.persistentSchedulerMu.Unlock()

	go func() {
		defer close(done)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		a.dispatchDueScheduledTasks(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.dispatchDueScheduledTasks(ctx)
			}
		}
	}()
}

func (a *Registry) StopPersistentTaskScheduler(ctx context.Context) error {
	a.persistentSchedulerMu.Lock()
	cancel := a.persistentSchedulerCancel
	done := a.persistentSchedulerDone
	a.persistentSchedulerCancel = nil
	a.persistentSchedulerDone = nil
	a.persistentSchedulerMu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	waitDone := make(chan struct{})
	go func() {
		a.persistentTaskWG.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Registry) dispatchDueScheduledTasks(ctx context.Context) {
	if a.DB == nil {
		return
	}
	now := time.Now().UTC()
	var taskIDs []string
	if err := a.DB.WithContext(ctx).Model(new(schema.ScheduledTask)).
		Where("status = ? AND next_run_at IS NOT NULL AND next_run_at <= ?", schema.TaskStatusActive, now).
		Order("next_run_at ASC").Limit(100).Pluck("id", &taskIDs).Error; err != nil {
		return
	}
	for _, taskID := range taskIDs {
		task, run, execute, err := a.claimScheduledTask(ctx, taskID, now, schema.TaskTriggerSchedule, false)
		if err != nil || !execute {
			continue
		}
		a.persistentTaskWG.Add(1)
		go func() {
			defer a.persistentTaskWG.Done()
			a.executeScheduledTask(ctx, task, run)
		}()
	}
}

func (a *Registry) TriggerScheduledTask(ctx context.Context, taskID, ownerUserID, actorUserID string) (*schema.ScheduledTaskRun, error) {
	if a.DB == nil {
		return nil, errors.New("scheduler database is not configured")
	}
	now := time.Now().UTC()
	task, run, execute, err := a.claimScheduledTaskForOwner(ctx, taskID, ownerUserID, actorUserID, now, schema.TaskTriggerManual, true)
	if err != nil {
		return nil, err
	}
	if !execute {
		return run, ErrScheduledTaskNotRunnable
	}
	a.persistentTaskWG.Add(1)
	go func() {
		defer a.persistentTaskWG.Done()
		a.executeScheduledTask(context.Background(), task, run)
	}()
	return run, nil
}

func (a *Registry) claimScheduledTask(ctx context.Context, taskID string, now time.Time,
	triggerSource string, manual bool) (*schema.ScheduledTask, *schema.ScheduledTaskRun, bool, error) {
	return a.claimScheduledTaskForOwner(ctx, taskID, "", "", now, triggerSource, manual)
}

func (a *Registry) claimScheduledTaskForOwner(ctx context.Context, taskID, ownerUserID, actorUserID string, now time.Time,
	triggerSource string, manual bool) (*schema.ScheduledTask, *schema.ScheduledTaskRun, bool, error) {
	var task *schema.ScheduledTask
	var run *schema.ScheduledTaskRun
	execute := false
	err := a.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item := new(schema.ScheduledTask)
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID)
		if ownerUserID != "" {
			query = query.Where("owner_user_id = ?", ownerUserID)
		}
		if err := query.First(item).Error; err != nil {
			return err
		}
		if item.Status == schema.TaskStatusDisabled || (!manual && item.Status != schema.TaskStatusActive) {
			return ErrScheduledTaskNotRunnable
		}
		beforeJSON := ""
		if manual {
			data, err := json.Marshal(item)
			if err != nil {
				return err
			}
			beforeJSON = string(data)
		}
		scheduledAt := now
		if !manual {
			if item.NextRunAt == nil || item.NextRunAt.After(now) {
				return nil
			}
			scheduledAt = item.NextRunAt.UTC()
			nextRun, err := schema.NextScheduledTaskRun(item.CronExpression, item.TimeZone, now)
			if err != nil {
				return err
			}
			item.NextRunAt = &nextRun
		}

		run = &schema.ScheduledTaskRun{ID: uuid.NewString(), TaskID: item.ID, OwnerUserID: item.OwnerUserID,
			TaskName: item.Name, Handler: item.Handler, TriggerSource: triggerSource, ScheduledAt: scheduledAt,
			Status: schema.TaskRunRunning, InstanceID: a.InstanceID, CreatedAt: now, UpdatedAt: now}
		startedAt := now
		run.StartedAt = &startedAt

		misfired := !manual && now.Sub(scheduledAt) > persistentTaskMisfireGrace
		leaseActive := item.ConcurrencyPolicy == schema.TaskConcurrencyForbid && item.LeaseUntil != nil && item.LeaseUntil.After(now)
		if (misfired && item.MisfirePolicy == schema.TaskMisfireSkip) || leaseActive {
			finishedAt := now
			run.Status = schema.TaskRunSkipped
			run.FinishedAt = &finishedAt
			if leaseActive {
				run.ErrorMessage = ErrScheduledTaskConcurrent.Error()
			} else {
				run.ErrorMessage = "scheduled occurrence exceeded the misfire grace period"
			}
		} else {
			execute = true
			if item.ConcurrencyPolicy == schema.TaskConcurrencyForbid {
				leaseUntil := now.Add(taskMaximumRuntime(item))
				item.LeaseOwner, item.LeaseUntil = a.InstanceID, &leaseUntil
			}
		}
		item.LastRunAt = &now
		item.UpdatedAt = now
		if err := tx.Select("next_run_at", "last_run_at", "lease_owner", "lease_until", "updated_at").Updates(item).Error; err != nil {
			return err
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(run)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			execute = false
		}
		if manual && execute {
			if strings.TrimSpace(actorUserID) == "" {
				actorUserID = item.OwnerUserID
			}
			afterJSON, err := json.Marshal(map[string]interface{}{
				"run_id": run.ID, "status": run.Status, "scheduled_at": run.ScheduledAt,
			})
			if err != nil {
				return err
			}
			audit := &schema.ScheduledTaskAudit{
				ID: uuid.NewString(), TaskID: item.ID, OwnerUserID: item.OwnerUserID,
				ActorUserID: actorUserID, TaskName: item.Name, Action: schema.TaskAuditTrigger,
				BeforeJSON: beforeJSON, AfterJSON: string(afterJSON), CreatedAt: now,
			}
			if err := tx.Create(audit).Error; err != nil {
				return err
			}
		}
		task = item
		return nil
	})
	return task, run, execute, err
}

func (a *Registry) executeScheduledTask(parent context.Context, task *schema.ScheduledTask, run *schema.ScheduledTaskRun) {
	if task == nil || run == nil {
		return
	}
	started := time.Now().UTC()
	var runErr error
	attempts := 0
	for attempt := 0; attempt <= task.MaxRetries; attempt++ {
		if parent.Err() != nil {
			runErr = parent.Err()
			break
		}
		attempts = attempt + 1
		attemptCtx, cancel := context.WithTimeout(parent, time.Duration(task.TimeoutSeconds)*time.Second)
		runErr = a.executeScheduledTaskHandler(attemptCtx, task.OwnerUserID, task.Handler)
		cancel()
		if runErr == nil {
			break
		}
		if attempt < task.MaxRetries {
			if err := waitSchedulerContext(parent, time.Duration(task.RetryIntervalSeconds)*time.Second); err != nil {
				runErr = err
				break
			}
		}
	}
	finished := time.Now().UTC()
	status, message := schema.TaskRunSuccess, ""
	if runErr != nil {
		status, message = schema.TaskRunFailed, limitTaskError(runErr.Error())
	}
	_ = a.DB.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(new(schema.ScheduledTaskRun)).Where("id = ?", run.ID).Updates(map[string]interface{}{
			"status": status, "attempts": attempts, "duration_ms": finished.Sub(started).Milliseconds(),
			"error_message": message, "finished_at": finished, "updated_at": finished,
		}).Error; err != nil {
			return err
		}
		return tx.Model(new(schema.ScheduledTask)).Where("id = ? AND lease_owner = ?", task.ID, a.InstanceID).
			Updates(map[string]interface{}{"lease_owner": "", "lease_until": nil, "updated_at": finished}).Error
	})
}

func (a *Registry) executeScheduledTaskHandler(ctx context.Context, ownerUserID, handler string) error {
	if a.taskHandlerHook != nil {
		return a.taskHandlerHook(ctx, ownerUserID, handler)
	}
	switch handler {
	case schema.TaskHandlerAccountSync:
		return a.runAccountSyncCycleForOwner(ctx, defaultAccountSyncTimeout, false, ownerUserID)
	case schema.TaskHandlerAccountSnapshot:
		return a.runAccountSyncCycleForOwner(ctx, defaultAccountSyncTimeout, true, ownerUserID)
	default:
		return fmt.Errorf("scheduled task handler %q is not allowed", handler)
	}
}

func taskMaximumRuntime(task *schema.ScheduledTask) time.Duration {
	attempts := task.MaxRetries + 1
	duration := time.Duration(attempts*task.TimeoutSeconds) * time.Second
	if task.MaxRetries > 0 {
		duration += time.Duration(task.MaxRetries*task.RetryIntervalSeconds) * time.Second
	}
	return duration + 30*time.Second
}

func limitTaskError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 2000 {
		return value[:2000]
	}
	return value
}

func waitSchedulerContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
