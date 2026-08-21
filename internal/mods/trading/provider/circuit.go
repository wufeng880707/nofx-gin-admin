package provider

import (
	"errors"
	"sync"
	"time"
)

var ErrProviderCircuitOpen = errors.New("provider circuit is open")

const (
	providerCircuitFailureThreshold = 5
	providerCircuitCooldown         = 30 * time.Second
)

type circuitBreaker struct {
	mu               sync.Mutex
	failureThreshold int
	cooldown         time.Duration
	failures         int
	openedAt         time.Time
	probeInFlight    bool
}

func newCircuitBreaker(failureThreshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{failureThreshold: failureThreshold, cooldown: cooldown}
}

func (a *circuitBreaker) allow(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.openedAt.IsZero() {
		return true
	}
	if now.Sub(a.openedAt) < a.cooldown || a.probeInFlight {
		return false
	}
	a.probeInFlight = true
	return true
}

func (a *circuitBreaker) success() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failures = 0
	a.openedAt = time.Time{}
	a.probeInFlight = false
}

func (a *circuitBreaker) failure(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.probeInFlight || !a.openedAt.IsZero() {
		a.openedAt = now
		a.probeInFlight = false
		return
	}
	a.failures++
	if a.failures >= a.failureThreshold {
		a.openedAt = now
	}
}

func (a *circuitBreaker) cancelProbe() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.probeInFlight = false
}

func (a *circuitBreaker) isOpen(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.openedAt.IsZero() && now.Sub(a.openedAt) < a.cooldown
}

type circuitRegistry struct {
	mu       sync.Mutex
	circuits map[string]*circuitBreaker
}

func newCircuitRegistry() *circuitRegistry {
	return &circuitRegistry{circuits: make(map[string]*circuitBreaker)}
}

func (a *circuitRegistry) get(key string) *circuitBreaker {
	a.mu.Lock()
	defer a.mu.Unlock()
	breaker := a.circuits[key]
	if breaker == nil {
		breaker = newCircuitBreaker(providerCircuitFailureThreshold, providerCircuitCooldown)
		a.circuits[key] = breaker
	}
	return breaker
}

var providerCircuits = newCircuitRegistry()
