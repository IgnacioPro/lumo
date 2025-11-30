package middleware

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
)

// UsageTracker tracks API usage per tenant
type UsageTracker struct {
	tenantRepo *repository.TenantRepository
	counters   map[uuid.UUID]*tenantCounter
	mu         sync.RWMutex
	flushTick  *time.Ticker
	done       chan struct{}
}

type tenantCounter struct {
	apiCalls  int64
	events    int64
	lastFlush time.Time
	mu        sync.Mutex
}

// NewUsageTracker creates a new usage tracker
func NewUsageTracker(tenantRepo *repository.TenantRepository) *UsageTracker {
	ut := &UsageTracker{
		tenantRepo: tenantRepo,
		counters:   make(map[uuid.UUID]*tenantCounter),
		flushTick:  time.NewTicker(1 * time.Minute),
		done:       make(chan struct{}),
	}

	go ut.flushLoop()
	return ut
}

// flushLoop periodically flushes counters to the database
func (ut *UsageTracker) flushLoop() {
	for {
		select {
		case <-ut.flushTick.C:
			ut.flushAll()
		case <-ut.done:
			ut.flushTick.Stop()
			ut.flushAll() // Final flush
			return
		}
	}
}

// Stop stops the usage tracker
func (ut *UsageTracker) Stop() {
	close(ut.done)
}

// flushAll flushes all tenant counters to the database
func (ut *UsageTracker) flushAll() {
	ut.mu.RLock()
	tenantIDs := make([]uuid.UUID, 0, len(ut.counters))
	for id := range ut.counters {
		tenantIDs = append(tenantIDs, id)
	}
	ut.mu.RUnlock()

	for _, tenantID := range tenantIDs {
		ut.flushTenant(tenantID)
	}
}

// flushTenant flushes a single tenant's counters
func (ut *UsageTracker) flushTenant(tenantID uuid.UUID) {
	ut.mu.RLock()
	counter, exists := ut.counters[tenantID]
	ut.mu.RUnlock()

	if !exists {
		return
	}

	counter.mu.Lock()
	apiCalls := counter.apiCalls
	events := counter.events
	counter.apiCalls = 0
	counter.events = 0
	counter.lastFlush = time.Now()
	counter.mu.Unlock()

	if apiCalls == 0 && events == 0 {
		return
	}

	// Update in database (fire and forget for performance)
	// In production, you'd want proper error handling and retry logic
	_ = ut.tenantRepo.IncrementUsage(context.Background(), tenantID, apiCalls, events)
}

// getCounter gets or creates a counter for a tenant
func (ut *UsageTracker) getCounter(tenantID uuid.UUID) *tenantCounter {
	ut.mu.RLock()
	counter, exists := ut.counters[tenantID]
	ut.mu.RUnlock()

	if exists {
		return counter
	}

	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Double-check after acquiring write lock
	if counter, exists = ut.counters[tenantID]; exists {
		return counter
	}

	counter = &tenantCounter{
		lastFlush: time.Now(),
	}
	ut.counters[tenantID] = counter
	return counter
}

// TrackAPICall increments the API call counter for a tenant
func (ut *UsageTracker) TrackAPICall(tenantID uuid.UUID) {
	counter := ut.getCounter(tenantID)
	counter.mu.Lock()
	counter.apiCalls++
	counter.mu.Unlock()
}

// TrackEvents increments the event counter for a tenant
func (ut *UsageTracker) TrackEvents(tenantID uuid.UUID, count int64) {
	counter := ut.getCounter(tenantID)
	counter.mu.Lock()
	counter.events += count
	counter.mu.Unlock()
}

// GetCurrentUsage returns the current (unflushed) usage for a tenant
func (ut *UsageTracker) GetCurrentUsage(tenantID uuid.UUID) (apiCalls, events int64) {
	ut.mu.RLock()
	counter, exists := ut.counters[tenantID]
	ut.mu.RUnlock()

	if !exists {
		return 0, 0
	}

	counter.mu.Lock()
	apiCalls = counter.apiCalls
	events = counter.events
	counter.mu.Unlock()

	return
}

// UsageTracking middleware tracks API usage per tenant
func UsageTracking(tracker *UsageTracker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := GetTenantID(r.Context())
			if tenantID != uuid.Nil {
				tracker.TrackAPICall(tenantID)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TenantLimits stores rate limit configuration per plan
type TenantLimits struct {
	RequestsPerMinute map[models.TenantPlan]int
	RequestsPerHour   map[models.TenantPlan]int
	EventsPerDay      map[models.TenantPlan]int
}

// DefaultTenantLimits returns the default rate limits per plan
func DefaultTenantLimits() *TenantLimits {
	return &TenantLimits{
		RequestsPerMinute: map[models.TenantPlan]int{
			models.TenantPlanTrial:      30,
			models.TenantPlanStarter:    60,
			models.TenantPlanPro:        300,
			models.TenantPlanEnterprise: 1000,
		},
		RequestsPerHour: map[models.TenantPlan]int{
			models.TenantPlanTrial:      500,
			models.TenantPlanStarter:    3600,
			models.TenantPlanPro:        18000,
			models.TenantPlanEnterprise: 100000,
		},
		EventsPerDay: map[models.TenantPlan]int{
			models.TenantPlanTrial:      1000,
			models.TenantPlanStarter:    10000,
			models.TenantPlanPro:        100000,
			models.TenantPlanEnterprise: 1000000,
		},
	}
}

// TenantRateLimiterState holds per-tenant rate limiting state
type TenantRateLimiterState struct {
	tenantRepo *repository.TenantRepository
	limits     *TenantLimits
	windows    map[uuid.UUID]*rateLimitWindow
	mu         sync.RWMutex
}

type rateLimitWindow struct {
	minuteCount int
	minuteStart time.Time
	hourCount   int
	hourStart   time.Time
	mu          sync.Mutex
}

// NewTenantRateLimiterState creates a new tenant rate limiter
func NewTenantRateLimiterState(tenantRepo *repository.TenantRepository, limits *TenantLimits) *TenantRateLimiterState {
	if limits == nil {
		limits = DefaultTenantLimits()
	}
	return &TenantRateLimiterState{
		tenantRepo: tenantRepo,
		limits:     limits,
		windows:    make(map[uuid.UUID]*rateLimitWindow),
	}
}

// getWindow gets or creates a rate limit window for a tenant
func (s *TenantRateLimiterState) getWindow(tenantID uuid.UUID) *rateLimitWindow {
	s.mu.RLock()
	window, exists := s.windows[tenantID]
	s.mu.RUnlock()

	if exists {
		return window
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if window, exists = s.windows[tenantID]; exists {
		return window
	}

	now := time.Now()
	window = &rateLimitWindow{
		minuteStart: now,
		hourStart:   now,
	}
	s.windows[tenantID] = window
	return window
}

// CheckAndIncrement checks if a request is allowed and increments counters
func (s *TenantRateLimiterState) CheckAndIncrement(tenantID uuid.UUID, plan models.TenantPlan) (allowed bool, retryAfter time.Duration) {
	window := s.getWindow(tenantID)
	now := time.Now()

	window.mu.Lock()
	defer window.mu.Unlock()

	// Reset minute window if needed
	if now.Sub(window.minuteStart) >= time.Minute {
		window.minuteCount = 0
		window.minuteStart = now
	}

	// Reset hour window if needed
	if now.Sub(window.hourStart) >= time.Hour {
		window.hourCount = 0
		window.hourStart = now
	}

	// Get limits for plan
	minuteLimit := s.limits.RequestsPerMinute[plan]
	hourLimit := s.limits.RequestsPerHour[plan]

	// Check minute limit
	if window.minuteCount >= minuteLimit {
		return false, time.Minute - now.Sub(window.minuteStart)
	}

	// Check hour limit
	if window.hourCount >= hourLimit {
		return false, time.Hour - now.Sub(window.hourStart)
	}

	// Increment counters
	window.minuteCount++
	window.hourCount++

	return true, 0
}

// PerTenantRateLimit applies per-tenant rate limiting based on plan
func PerTenantRateLimit(state *TenantRateLimiterState) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := GetTenant(r.Context())
			if tenant == nil {
				// No tenant context, skip rate limiting
				next.ServeHTTP(w, r)
				return
			}

			// Enterprise dedicated tenants skip rate limiting
			if tenant.IsEnterprise() && tenant.IsDedicated() {
				next.ServeHTTP(w, r)
				return
			}

			allowed, retryAfter := state.CheckAndIncrement(tenant.ID, tenant.Plan)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				w.Header().Set("X-RateLimit-Plan", string(tenant.Plan))
				response.Error(w, http.StatusTooManyRequests, "rate_limit_exceeded",
					"Rate limit exceeded. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// EventLimitChecker checks if a tenant can submit more events
type EventLimitChecker struct {
	tenantRepo *repository.TenantRepository
	limits     *TenantLimits
}

// NewEventLimitChecker creates a new event limit checker
func NewEventLimitChecker(tenantRepo *repository.TenantRepository, limits *TenantLimits) *EventLimitChecker {
	if limits == nil {
		limits = DefaultTenantLimits()
	}
	return &EventLimitChecker{
		tenantRepo: tenantRepo,
		limits:     limits,
	}
}

// CanSubmitEvents checks if a tenant can submit the given number of events
func (c *EventLimitChecker) CanSubmitEvents(tenant *models.Tenant, count int) (bool, int) {
	if tenant == nil {
		return true, 0 // No tenant, allow (backwards compatibility)
	}

	// Get daily limit from plan or tenant override
	dailyLimit := c.limits.EventsPerDay[tenant.Plan]
	if tenant.MaxEventsPerDay > 0 {
		dailyLimit = tenant.MaxEventsPerDay
	}

	// Get today's usage
	// In production, this would query the tenant_usage table
	// For now, we return true with remaining = dailyLimit - count
	remaining := dailyLimit - count
	if remaining < 0 {
		remaining = 0
	}

	return count <= dailyLimit, remaining
}

// EventLimitMiddleware checks event limits before allowing event submission
func EventLimitMiddleware(checker *EventLimitChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only apply to event submission endpoints
			if r.URL.Path != "/api/v1/events" || r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			tenant := GetTenant(r.Context())
			if tenant == nil {
				next.ServeHTTP(w, r)
				return
			}

			// We can't know the count before parsing, so we check with 1
			// The actual limit check happens in the handler after parsing
			allowed, remaining := checker.CanSubmitEvents(tenant, 1)
			if !allowed {
				w.Header().Set("X-Events-Remaining", "0")
				response.Error(w, http.StatusPaymentRequired, "event_limit_exceeded",
					"Daily event limit exceeded. Please upgrade your plan.")
				return
			}

			w.Header().Set("X-Events-Remaining", strconv.Itoa(remaining))
			next.ServeHTTP(w, r)
		})
	}
}
