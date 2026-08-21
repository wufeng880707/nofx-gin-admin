package runtime

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestAccountSyncSchedulerRunsImmediatelyAndStops(t *testing.T) {
	var cycles atomic.Int64
	registry := &Registry{syncCycleHook: func(_ context.Context, persistSnapshot bool) {
		if persistSnapshot {
			t.Error("short interval synchronization unexpectedly persisted a snapshot")
		}
		cycles.Add(1)
	}}
	registry.StartAccountSyncScheduler(10*time.Millisecond, time.Second, 12*time.Hour, "Asia/Shanghai")

	deadline := time.Now().Add(time.Second)
	for cycles.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if cycles.Load() < 2 {
		t.Fatalf("scheduler did not run immediately and on interval: cycles=%d", cycles.Load())
	}
	if err := registry.StopAccountSyncScheduler(context.Background()); err != nil {
		t.Fatal(err)
	}
	stoppedAt := cycles.Load()
	time.Sleep(30 * time.Millisecond)
	if cycles.Load() != stoppedAt {
		t.Fatalf("scheduler continued after stop: before=%d after=%d", stoppedAt, cycles.Load())
	}
}

func TestAccountSyncSchedulerStartIsIdempotent(t *testing.T) {
	started := make(chan struct{}, 2)
	registry := &Registry{syncCycleHook: func(context.Context, bool) {
		select {
		case started <- struct{}{}:
		default:
		}
	}}
	registry.StartAccountSyncScheduler(time.Hour, time.Second, 12*time.Hour, "Asia/Shanghai")
	registry.StartAccountSyncScheduler(time.Hour, time.Second, 12*time.Hour, "Asia/Shanghai")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not start")
	}
	if err := registry.StopAccountSyncScheduler(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNextAlignedSnapshotTimeUsesMidnightAndNoon(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before noon",
			now:  time.Date(2026, 8, 11, 11, 59, 0, 0, location),
			want: time.Date(2026, 8, 11, 12, 0, 0, 0, location),
		},
		{
			name: "at noon schedules midnight",
			now:  time.Date(2026, 8, 11, 12, 0, 0, 0, location),
			want: time.Date(2026, 8, 12, 0, 0, 0, 0, location),
		},
		{
			name: "after noon",
			now:  time.Date(2026, 8, 11, 23, 0, 0, 0, location),
			want: time.Date(2026, 8, 12, 0, 0, 0, 0, location),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextAlignedSnapshotTime(tt.now, 12*time.Hour, location); !got.Equal(tt.want) {
				t.Fatalf("next snapshot=%s want=%s", got, tt.want)
			}
		})
	}
}
