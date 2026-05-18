// Package services implements the core business logic.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	passwordvalidator "github.com/wagslane/go-password-validator"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/bcrypt"
)

// Transaction is a type alias for pgx.Tx so that postgres.DB.Begin (which
// returns pgx.Tx) satisfies the services.DB interface.
type Transaction = pgx.Tx

// DB is the database interface that supports beginning transactions.
type DB interface {
	Begin(ctx context.Context) (Transaction, error)
}

const (
	lockoutThreshold = 5
	defaultLockout   = 15 * time.Minute
	// Hard size limits prevent unbounded map growth under high failure traffic.
	// The probabilistic purge (every ~10 calls) may not keep up with rapid failures.
	maxFailedAttemptsMap = 1000
	maxLockoutsMap       = 10000
)

// AuthService handles registration and authentication workflows.
type AuthService struct {
	userRepo        ports.UserRepository
	apiKeySvc       ports.IdentityService
	auditSvc        ports.AuditService
	tenantSvc       ports.TenantService
	db              DB
	logger          *slog.Logger
	failedAttempts  map[string]int
	lockouts        map[string]time.Time
	lockoutDuration time.Duration
	mu              sync.Mutex
}

// NewAuthService constructs an AuthService with its dependencies.
func NewAuthService(userRepo ports.UserRepository, apiKeySvc ports.IdentityService, auditSvc ports.AuditService, tenantSvc ports.TenantService, db DB, logger *slog.Logger) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		apiKeySvc:       apiKeySvc,
		auditSvc:        auditSvc,
		tenantSvc:       tenantSvc,
		db:              db,
		logger:          logger,
		failedAttempts:  make(map[string]int),
		lockouts:        make(map[string]time.Time),
		lockoutDuration: defaultLockout,
	}
}

// SetLockoutDuration overrides the default lockout duration. Useful for testing.
func (s *AuthService) SetLockoutDuration(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockoutDuration = d
}

func (s *AuthService) Register(ctx context.Context, email, password, name string) (*domain.User, error) {
	ctx, span := otel.Tracer("auth-service").Start(ctx, "Register")
	defer span.End()
	span.SetAttributes(attribute.String("user.email", email))
	// Validate password strength
	if err := passwordvalidator.Validate(password, 50); err != nil {
		return nil, errors.New(errors.InvalidInput, "password is too weak: "+err.Error())
	}

	// Check if user already exists
	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New(errors.Conflict, "user with this email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		Name:         name,
		Role:         domain.RoleDeveloper,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now(),
	}

	// Begin transaction for atomic user+tenant creation
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to begin transaction", err)
	}
	txCtx := core.WithTransaction(ctx, tx)
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	err = s.userRepo.Create(txCtx, user)
	if err != nil {
		return nil, err
	}

	// Create Personal Tenant
	tenantName := fmt.Sprintf("%s's Personal Tenant", name)

	// Simple slugify: lowercase, replace spaces with hyphens, keep only alphanumeric
	slugName := strings.ToLower(name)
	slugName = strings.ReplaceAll(slugName, " ", "-")

	// Remove non-alphanumeric chars
	var cleanSlug strings.Builder
	for _, r := range slugName {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleanSlug.WriteRune(r)
		}
	}
	slugName = cleanSlug.String()

	if slugName == "" {
		slugName = "personal"
	}
	tenantSlug := fmt.Sprintf("personal-%s-%s", slugName, user.ID.String()[:8])

	tenant, err := s.tenantSvc.CreateTenant(appcontext.WithInternalCall(txCtx), tenantName, tenantSlug, user.ID)
	if err != nil {
		return nil, err
	}
	txCtx = appcontext.WithTenantID(txCtx, tenant.ID)

	// Reload user to reflect changes made during tenant creation (e.g. DefaultTenantID)
	updatedUser, err := s.userRepo.GetByID(txCtx, user.ID)
	if err == nil {
		user = updatedUser
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return nil, errors.Wrap(errors.Internal, "failed to commit transaction", commitErr)
	}
	err = nil // transaction committed successfully, clear so defer no-ops

	if err := s.auditSvc.Log(ctx, user.ID, "user.register", "user", user.ID.String(), map[string]interface{}{
		"email": email,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "user.register", "user_id", user.ID, "error", err)
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, string, error) {
	ctx, span := otel.Tracer("auth-service").Start(ctx, "Login")
	defer span.End()
	span.SetAttributes(attribute.String("user.email", email))
	s.mu.Lock()
	if lockoutTime, ok := s.lockouts[email]; ok {
		if time.Now().Before(lockoutTime) {
			s.mu.Unlock()
			return nil, "", errors.New(errors.Unauthorized, "account is locked due to too many failed attempts")
		}
		delete(s.lockouts, email)
		delete(s.failedAttempts, email)
	}
	s.mu.Unlock()

	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil || user == nil {
		s.incrementFailure(email)
		platform.AuthAttemptsTotal.WithLabelValues("failure_not_found").Inc()
		return nil, "", errors.New(errors.Unauthorized, "invalid email or password")
	}

	// Compare password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		s.incrementFailure(email)
		return nil, "", errors.New(errors.Unauthorized, "invalid email or password")
	}

	// Clear failures on success
	s.mu.Lock()
	delete(s.failedAttempts, email)
	s.mu.Unlock()

	if user.DefaultTenantID != nil {
		ctx = appcontext.WithTenantID(ctx, *user.DefaultTenantID)
	}
	// or just return a fresh one. In a real platform, login gives you a JWT and
	// you manage API keys separately.
	// For now, let's create a default key for them.
	key, err := s.apiKeySvc.CreateKey(ctx, user.ID, "Default Key")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create initial API key: %w", err)
	}

	if err := s.auditSvc.Log(ctx, user.ID, "user.login", "user", user.ID.String(), map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "user.login", "user_id", user.ID, "error", err)
	}

	platform.AuthAttemptsTotal.WithLabelValues("success").Inc()

	return user, key.Key, nil
}

func (s *AuthService) incrementFailure(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedAttempts[email]++
	platform.AuthAttemptsTotal.WithLabelValues("failure_incorrect_password").Inc()
	if s.failedAttempts[email] >= lockoutThreshold {
		s.lockouts[email] = time.Now().Add(s.lockoutDuration)
		platform.AuthAttemptsTotal.WithLabelValues("failure_lockout").Inc()
	}
	// Probabilistically purge expired entries to prevent unbounded map growth.
	// Every ~10 calls, scan and remove stale entries.
	if len(s.lockouts) > 0 && time.Now().Nanosecond()%10 == 0 {
		s.purgeExpiredLocked()
	}
	// Deterministic hard-limit eviction: if maps grow beyond maxSize, aggressively
	// purge all expired entries. This prevents unbounded growth under high failure
	// traffic before the probabilistic trigger can catch up.
	if len(s.lockouts) > maxLockoutsMap {
		s.purgeExpiredLocked()
	}
	if len(s.failedAttempts) > maxFailedAttemptsMap {
		s.purgeExpiredLocked()
	}
}

// purgeExpiredLocked removes expired lockouts and stale failure records.
// If the maps are still over their hard caps after expiry-based purging,
// evict the oldest lockout entries (and their paired failedAttempts) and
// the highest-count failedAttempts entries until back under the caps.
// Caller must hold s.mu.
func (s *AuthService) purgeExpiredLocked() {
	now := time.Now()
	for email, lockoutTime := range s.lockouts {
		if now.After(lockoutTime) {
			delete(s.lockouts, email)
			delete(s.failedAttempts, email)
		}
	}
	// Also purge failure-only entries that have no lockout and are very old
	// (these are users who failed but didn't reach lockout threshold)
	for email, count := range s.failedAttempts {
		// If count is suspiciously high, purge to prevent unbounded growth
		if count > lockoutThreshold*10 {
			delete(s.failedAttempts, email)
		}
	}

	// Hard-cap eviction: if expiry purging left us above the cap (e.g. an
	// attacker is filling the map with unexpired lockouts faster than they
	// expire), evict the earliest-locked entries until we're back under the
	// limit. This bounds memory at O(cap) regardless of failure traffic.
	if len(s.lockouts) > maxLockoutsMap {
		evictOldestLockoutsLocked(s.lockouts, s.failedAttempts, maxLockoutsMap)
	}
	if len(s.failedAttempts) > maxFailedAttemptsMap {
		evictHighestFailureCountsLocked(s.failedAttempts, maxFailedAttemptsMap)
	}
}

// evictOldestLockoutsLocked drops the lockouts (and paired failedAttempts)
// with the earliest unlock-time until the map is at most targetSize.
func evictOldestLockoutsLocked(lockouts map[string]time.Time, failedAttempts map[string]int, targetSize int) {
	excess := len(lockouts) - targetSize
	if excess <= 0 {
		return
	}
	type entry struct {
		email string
		t     time.Time
	}
	entries := make([]entry, 0, len(lockouts))
	for email, t := range lockouts {
		entries = append(entries, entry{email, t})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].t.Before(entries[j].t) })
	for i := 0; i < excess && i < len(entries); i++ {
		delete(lockouts, entries[i].email)
		delete(failedAttempts, entries[i].email)
	}
}

// evictHighestFailureCountsLocked drops the highest-count failedAttempts
// entries first — those are the addresses most likely already locked-out
// or being abused — until the map is at most targetSize.
func evictHighestFailureCountsLocked(failedAttempts map[string]int, targetSize int) {
	excess := len(failedAttempts) - targetSize
	if excess <= 0 {
		return
	}
	type entry struct {
		email string
		count int
	}
	entries := make([]entry, 0, len(failedAttempts))
	for email, c := range failedAttempts {
		entries = append(entries, entry{email, c})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].count > entries[j].count })
	for i := 0; i < excess && i < len(entries); i++ {
		delete(failedAttempts, entries[i].email)
	}
}

func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			return nil, errors.New(errors.NotFound, "user not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to fetch user", err)
	}
	return user, nil
}
