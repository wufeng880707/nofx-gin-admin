package schema

import (
	"strings"
	"time"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/LyricTian/gin-admin/v10/pkg/errors"
	"github.com/LyricTian/gin-admin/v10/pkg/util"
	"github.com/robfig/cron/v3"
)

const (
	TaskHandlerAccountSync     = "account_sync"
	TaskHandlerAccountSnapshot = "account_snapshot"

	TaskStatusActive   = "active"
	TaskStatusPaused   = "paused"
	TaskStatusDisabled = "disabled"

	TaskMisfireSkip     = "skip"
	TaskMisfireFireOnce = "fire_once"

	TaskConcurrencyForbid = "forbid"
	TaskConcurrencyAllow  = "allow"

	TaskRunPending = "pending"
	TaskRunRunning = "running"
	TaskRunSuccess = "success"
	TaskRunFailed  = "failed"
	TaskRunSkipped = "skipped"

	TaskTriggerSchedule = "schedule"
	TaskTriggerManual   = "manual"

	TaskAuditCreate  = "create"
	TaskAuditUpdate  = "update"
	TaskAuditDelete  = "delete"
	TaskAuditTrigger = "trigger"
	TaskAuditPause   = "pause"
	TaskAuditResume  = "resume"
	TaskAuditDisable = "disable"
)

var taskCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

type ScheduledTask struct {
	ID                   string     `json:"id" gorm:"size:36;primaryKey"`
	OwnerUserID          string     `json:"owner_user_id" gorm:"size:20;not null;index;uniqueIndex:uk_trading_scheduled_task_name,priority:1"`
	Name                 string     `json:"name" gorm:"size:128;not null;uniqueIndex:uk_trading_scheduled_task_name,priority:2"`
	Handler              string     `json:"handler" gorm:"size:64;not null;index"`
	CronExpression       string     `json:"cron_expression" gorm:"size:128;not null"`
	TimeZone             string     `json:"time_zone" gorm:"size:64;not null"`
	Status               string     `json:"status" gorm:"size:16;not null;default:active;index"`
	TimeoutSeconds       int        `json:"timeout_seconds" gorm:"not null;default:60"`
	MaxRetries           int        `json:"max_retries" gorm:"not null;default:0"`
	RetryIntervalSeconds int        `json:"retry_interval_seconds" gorm:"not null;default:5"`
	MisfirePolicy        string     `json:"misfire_policy" gorm:"size:16;not null;default:skip"`
	ConcurrencyPolicy    string     `json:"concurrency_policy" gorm:"size:16;not null;default:forbid"`
	NextRunAt            *time.Time `json:"next_run_at" gorm:"index"`
	LastRunAt            *time.Time `json:"last_run_at" gorm:"index"`
	LeaseOwner           string     `json:"-" gorm:"size:128"`
	LeaseUntil           *time.Time `json:"-" gorm:"index"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (*ScheduledTask) TableName() string {
	return config.C.FormatTableName("trading_scheduled_tasks")
}

type ScheduledTaskRun struct {
	ID            string     `json:"id" gorm:"size:36;primaryKey"`
	TaskID        string     `json:"task_id" gorm:"size:36;not null;index;uniqueIndex:uk_trading_task_run_occurrence,priority:1"`
	OwnerUserID   string     `json:"owner_user_id" gorm:"size:20;not null;index"`
	TaskName      string     `json:"task_name" gorm:"size:128;not null"`
	Handler       string     `json:"handler" gorm:"size:64;not null;index"`
	TriggerSource string     `json:"trigger_source" gorm:"size:16;not null"`
	ScheduledAt   time.Time  `json:"scheduled_at" gorm:"not null;index;uniqueIndex:uk_trading_task_run_occurrence,priority:2"`
	StartedAt     *time.Time `json:"started_at" gorm:"index"`
	FinishedAt    *time.Time `json:"finished_at" gorm:"index"`
	Status        string     `json:"status" gorm:"size:16;not null;index"`
	Attempts      int        `json:"attempts" gorm:"not null;default:0"`
	DurationMS    int64      `json:"duration_ms" gorm:"not null;default:0"`
	InstanceID    string     `json:"instance_id" gorm:"size:128;not null"`
	ErrorMessage  string     `json:"error_message" gorm:"size:2000"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (*ScheduledTaskRun) TableName() string {
	return config.C.FormatTableName("trading_scheduled_task_runs")
}

type ScheduledTaskAudit struct {
	ID          string    `json:"id" gorm:"size:36;primaryKey"`
	TaskID      string    `json:"task_id" gorm:"size:36;not null;index"`
	OwnerUserID string    `json:"owner_user_id" gorm:"size:20;not null;index"`
	ActorUserID string    `json:"actor_user_id" gorm:"size:20;not null;index"`
	TaskName    string    `json:"task_name" gorm:"size:128;not null"`
	Action      string    `json:"action" gorm:"size:32;not null;index"`
	BeforeJSON  string    `json:"before_json" gorm:"type:text"`
	AfterJSON   string    `json:"after_json" gorm:"type:text"`
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
}

func (*ScheduledTaskAudit) TableName() string {
	return config.C.FormatTableName("trading_scheduled_task_audits")
}

type ScheduledTaskForm struct {
	OwnerUserID          string `json:"owner_user_id"`
	Name                 string `json:"name" binding:"required,max=128"`
	Handler              string `json:"handler" binding:"required"`
	CronExpression       string `json:"cron_expression" binding:"required,max=128"`
	TimeZone             string `json:"time_zone" binding:"required,max=64"`
	TimeoutSeconds       int    `json:"timeout_seconds" binding:"min=1,max=3600"`
	MaxRetries           int    `json:"max_retries" binding:"min=0,max=5"`
	RetryIntervalSeconds int    `json:"retry_interval_seconds" binding:"min=1,max=600"`
	MisfirePolicy        string `json:"misfire_policy" binding:"required"`
	ConcurrencyPolicy    string `json:"concurrency_policy" binding:"required"`
}

func (a *ScheduledTaskForm) Validate() error {
	a.Name = strings.TrimSpace(a.Name)
	a.Handler = strings.ToLower(strings.TrimSpace(a.Handler))
	a.CronExpression = strings.TrimSpace(a.CronExpression)
	a.TimeZone = strings.TrimSpace(a.TimeZone)
	a.MisfirePolicy = strings.ToLower(strings.TrimSpace(a.MisfirePolicy))
	a.ConcurrencyPolicy = strings.ToLower(strings.TrimSpace(a.ConcurrencyPolicy))
	if !IsScheduledTaskHandlerAllowed(a.Handler) {
		return errors.BadRequest("trading_task_handler_not_allowed", "Scheduled task handler is not allowed")
	}
	if _, _, err := ParseScheduledTaskSchedule(a.CronExpression, a.TimeZone); err != nil {
		return errors.BadRequest("trading_task_schedule_invalid", "Invalid cron expression or time zone: %v", err)
	}
	if a.MisfirePolicy != TaskMisfireSkip && a.MisfirePolicy != TaskMisfireFireOnce {
		return errors.BadRequest("trading_task_misfire_policy_invalid", "Unsupported misfire policy")
	}
	if a.ConcurrencyPolicy != TaskConcurrencyForbid && a.ConcurrencyPolicy != TaskConcurrencyAllow {
		return errors.BadRequest("trading_task_concurrency_policy_invalid", "Unsupported concurrency policy")
	}
	return nil
}

func IsScheduledTaskHandlerAllowed(handler string) bool {
	switch strings.ToLower(strings.TrimSpace(handler)) {
	case TaskHandlerAccountSync, TaskHandlerAccountSnapshot:
		return true
	default:
		return false
	}
}

func ParseScheduledTaskSchedule(expression, timeZone string) (cron.Schedule, *time.Location, error) {
	location, err := time.LoadLocation(strings.TrimSpace(timeZone))
	if err != nil {
		return nil, nil, err
	}
	schedule, err := taskCronParser.Parse(strings.TrimSpace(expression))
	if err != nil {
		return nil, nil, err
	}
	return schedule, location, nil
}

func NextScheduledTaskRun(expression, timeZone string, after time.Time) (time.Time, error) {
	schedule, location, err := ParseScheduledTaskSchedule(expression, timeZone)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(after.In(location)).UTC(), nil
}

type ScheduledTaskQueryParam struct {
	util.PaginationParam
	OwnerUserID string `form:"ownerUserID"`
	Status      string `form:"status"`
	Handler     string `form:"handler"`
}

type ScheduledTaskRunQueryParam struct {
	util.PaginationParam
	OwnerUserID string `form:"ownerUserID"`
	TaskID      string `form:"taskID"`
	Status      string `form:"status"`
}

type ScheduledTaskAuditQueryParam struct {
	util.PaginationParam
	OwnerUserID string `form:"ownerUserID"`
	TaskID      string `form:"taskID"`
	Action      string `form:"action"`
}

type ScheduledTaskQueryResult struct {
	Data       []*ScheduledTask
	PageResult *util.PaginationResult
}

type ScheduledTaskRunQueryResult struct {
	Data       []*ScheduledTaskRun
	PageResult *util.PaginationResult
}

type ScheduledTaskAuditQueryResult struct {
	Data       []*ScheduledTaskAudit
	PageResult *util.PaginationResult
}
