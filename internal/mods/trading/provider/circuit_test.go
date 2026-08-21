package provider

import (
	"testing"
	"time"
)

func TestCircuitBreakerOpenHalfOpenAndRecover(t *testing.T) {
	now := time.Now()
	breaker := newCircuitBreaker(2, time.Minute)
	if !breaker.allow(now) {
		t.Fatal("closed circuit should allow request")
	}
	breaker.failure(now)
	breaker.failure(now)
	if breaker.allow(now.Add(time.Second)) {
		t.Fatal("open circuit should reject request")
	}
	probeAt := now.Add(time.Minute)
	if !breaker.allow(probeAt) {
		t.Fatal("half-open circuit should allow one probe")
	}
	if breaker.allow(probeAt) {
		t.Fatal("half-open circuit should reject concurrent probes")
	}
	breaker.success()
	if !breaker.allow(probeAt) || breaker.isOpen(probeAt) {
		t.Fatal("successful probe should close circuit")
	}
}

func TestCircuitBreakerFailedProbeReopens(t *testing.T) {
	now := time.Now()
	breaker := newCircuitBreaker(1, time.Second)
	breaker.failure(now)
	probeAt := now.Add(time.Second)
	if !breaker.allow(probeAt) {
		t.Fatal("expected half-open probe")
	}
	breaker.failure(probeAt)
	if breaker.allow(probeAt.Add(500 * time.Millisecond)) {
		t.Fatal("failed probe should reopen circuit")
	}
}
