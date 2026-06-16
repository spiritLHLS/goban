package monitor

import (
	"context"
	"testing"
	"time"
)

func TestReportLimiterWaitReturnsFalseWhenContextCancelled(t *testing.T) {
	limiter := &ReportLimiter{next: time.Now().Add(time.Hour)}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	if limiter.Wait(ctx, 30) {
		t.Fatal("expected cancelled wait to return false")
	}
	if time.Since(started) > 200*time.Millisecond {
		t.Fatal("cancelled wait should return promptly")
	}
}

func TestMonitorServiceBeginTaskRespectsStop(t *testing.T) {
	service := NewMonitorService()
	service.ctx, service.cancel = context.WithCancel(context.Background())
	service.running = true

	ctx, ok := service.beginTask(1)
	if !ok {
		t.Fatal("expected first task to start")
	}
	if ctx == nil {
		t.Fatal("expected task context")
	}
	if _, ok := service.beginTask(1); ok {
		t.Fatal("duplicate task should not start")
	}

	service.clearTaskRunning(1)
	service.wg.Done()
	service.cancel()
	service.running = false
	if _, ok := service.beginTask(2); ok {
		t.Fatal("stopped service should not start tasks")
	}
}
