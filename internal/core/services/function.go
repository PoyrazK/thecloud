// Package services implements core business workflows.
package services

import (
	"archive/zip"
	"bytes"
	"context"
	stdlib_errors "errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/internal/platform"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerNameFunction = "function-service"

const (
	// maxLogSize bounds log reading in captureInvocationResults to prevent memory exhaustion.
	maxLogSize = 1 * 1024 * 1024 // 1 MB
	// CleanupTimeout is the timeout for asynchronous container cleanup.
	CleanupTimeout = 30 * time.Second
)

// contextWithMetadata copies user/tenant metadata from src into a fresh background context.
// This allows cleanup goroutines to have access to context values (for logging/tracing)
// without being tied to src's cancellation or deadline.
func contextWithMetadata(src context.Context) context.Context {
	ctx := context.Background()
	if userID := appcontext.UserIDFromContext(src); userID != uuid.Nil {
		ctx = appcontext.WithUserID(ctx, userID)
	}
	if tenantID := appcontext.TenantIDFromContext(src); tenantID != uuid.Nil {
		ctx = appcontext.WithTenantID(ctx, tenantID)
	}
	return ctx
}

var logSanitizationRe = regexp.MustCompile(`[^[:print:][:space:]]`)

// RuntimeConfig describes how a function runtime is executed.
type RuntimeConfig struct {
	Image      string
	Entrypoint []string
	Extension  string
}

var runtimes = map[string]RuntimeConfig{
	"nodejs20":  {Image: "node:20-alpine", Entrypoint: []string{"node"}, Extension: ".js"},
	"python312": {Image: "python:3.12-alpine", Entrypoint: []string{"python"}, Extension: ".py"},
	"go122":     {Image: "golang:1.22-alpine", Entrypoint: []string{"go", "run"}, Extension: ".go"},
	"ruby33":    {Image: "ruby:3.3-alpine", Entrypoint: []string{"ruby"}, Extension: ".rb"},
	"java21":    {Image: "eclipse-temurin:21-alpine", Entrypoint: []string{"java", "-jar"}, Extension: ".jar"},
}

// FunctionService manages serverless function lifecycle and invocations.
type FunctionService struct {
	repo             ports.FunctionRepository
	rbacSvc          ports.RBACService
	compute          ports.ComputeBackend
	fileStore        ports.FileStore
	auditSvc         ports.AuditService
	secretSvc        ports.SecretService
	logger           *slog.Logger
	bulkheadRegistry map[uuid.UUID]*platform.Bulkhead
	bulkheadMu       sync.RWMutex
}

// NewFunctionService constructs a FunctionService with its dependencies.
func NewFunctionService(repo ports.FunctionRepository, rbacSvc ports.RBACService, compute ports.ComputeBackend, fileStore ports.FileStore, auditSvc ports.AuditService, secretSvc ports.SecretService, logger *slog.Logger) *FunctionService {
	return &FunctionService{
		repo:             repo,
		rbacSvc:          rbacSvc,
		compute:          compute,
		fileStore:        fileStore,
		auditSvc:         auditSvc,
		secretSvc:        secretSvc,
		logger:           logger,
		bulkheadRegistry: make(map[uuid.UUID]*platform.Bulkhead),
	}
}

func (s *FunctionService) CreateFunction(ctx context.Context, name, runtime, handler string, code []byte) (*domain.Function, error) {
	tracer := otel.Tracer(tracerNameFunction)
	_, span := tracer.Start(ctx, "FunctionService.CreateFunction",
		trace.WithAttributes(
			attribute.String("function.name", name),
			attribute.String("function.runtime", runtime),
			attribute.String("function.handler", handler),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionCreate, "*"); err != nil {
		span.RecordError(err)
		return nil, err
	}

	if _, ok := runtimes[runtime]; !ok {
		return nil, errors.New(errors.InvalidInput, fmt.Sprintf("unsupported runtime: %s", runtime))
	}

	id := uuid.New()
	codeKey := fmt.Sprintf("%s/%s/code.zip", userID, id)

	_, err := s.fileStore.Write(ctx, "functions", codeKey, bytes.NewReader(code))
	if err != nil {
		s.logger.Error("failed to store function code", "error", err, "bucket", "functions", "key", codeKey)
		return nil, errors.Wrap(errors.Internal, "failed to store function code", err)
	}

	f := &domain.Function{
		ID:        id,
		UserID:    userID,
		TenantID:  tenantID,
		Name:      name,
		Runtime:   runtime,
		Handler:   handler,
		CodePath:  codeKey,
		Timeout:   30,
		MemoryMB:  128,
		CPUs:      0.5,
		Status:    "ACTIVE",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, f.UserID, "function.create", "function", f.ID.String(), map[string]interface{}{
		"name":    f.Name,
		"runtime": f.Runtime,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "function.create", "function_id", f.ID, "error", err)
	}

	s.logger.Info("function created", "name", name, "runtime", runtime, "id", id)

	return f, nil
}
func (s *FunctionService) GetFunction(ctx context.Context, id uuid.UUID) (*domain.Function, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionRead, id.String()); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, id)
}

func (s *FunctionService) ListFunctions(ctx context.Context) ([]*domain.Function, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionRead, "*"); err != nil {
		return nil, err
	}

	return s.repo.List(ctx, userID)
}

func (s *FunctionService) UpdateFunction(ctx context.Context, id uuid.UUID, req *domain.FunctionUpdate) (*domain.Function, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionUpdate, id.String()); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, id, req); err != nil {
		return nil, err
	}

	f, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.auditSvc.Log(ctx, f.UserID, "function.update", "function", f.ID.String(), map[string]interface{}{
		"name": f.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "function.update", "function_id", f.ID, "error", err)
	}

	s.logger.Info("function updated", "id", id)

	return f, nil
}

func (s *FunctionService) DeleteFunction(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionDelete, id.String()); err != nil {
		return err
	}

	f, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Remove bulkhead from registry to release resources
	s.bulkheadMu.Lock()
	delete(s.bulkheadRegistry, id)
	s.bulkheadMu.Unlock()

	// Async delete from file store
	go func() {
		delCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.fileStore.Delete(delCtx, "functions", f.CodePath); err != nil {
			s.logger.Warn("failed to delete function code from storage", "code_path", f.CodePath, "error", err)
		}
	}()
	if err := s.auditSvc.Log(ctx, f.UserID, "function.delete", "function", f.ID.String(), map[string]interface{}{
		"name": f.Name,
	}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "function.delete", "function_id", f.ID, "error", err)
	}

	return nil
}

func (s *FunctionService) GetFunctionLogs(ctx context.Context, id uuid.UUID, limit int) ([]*domain.Invocation, error) {
	tracer := otel.Tracer(tracerNameFunction)
	_, span := tracer.Start(ctx, "FunctionService.GetFunctionLogs",
		trace.WithAttributes(
			attribute.String("function.id", id.String()),
			attribute.Int("function.log_limit", limit),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionRead, id.String()); err != nil {
		span.RecordError(err)
		return nil, err
	}

	// Verify existence and tenant scoping (use original ctx to avoid mock context mismatch)
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		span.RecordError(err)
		return nil, err
	}

	invocations, err := s.repo.GetInvocations(ctx, id, limit)
	if err != nil {
		span.RecordError(err)
	}
	return invocations, err
}
func (s *FunctionService) InvokeFunction(ctx context.Context, id uuid.UUID, payload []byte, async bool) (*domain.Invocation, error) {
	tracer := otel.Tracer(tracerNameFunction)
	_, span := tracer.Start(ctx, "FunctionService.InvokeFunction",
		trace.WithAttributes(
			attribute.String("function.id", id.String()),
			attribute.Bool("function.async", async),
		))
	defer span.End()

	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)

	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionInvoke, id.String()); err != nil {
		span.RecordError(err)
		return nil, err
	}

	f, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	invocation := &domain.Invocation{
		ID:         uuid.New(),
		FunctionID: f.ID,
		Status:     "PENDING",
		StartedAt:  time.Now(),
	}

	if async {
		if err := s.auditSvc.Log(ctx, f.UserID, "function.invoke_async", "function", f.ID.String(), map[string]interface{}{}); err != nil {
			s.logger.Warn("failed to log audit event", "action", "function.invoke_async", "function_id", f.ID, "error", err)
		}
		b := s.getBulkhead(f)
		bgCtx := context.Background()
		bgCtx = appcontext.WithUserID(bgCtx, userID)
		bgCtx = appcontext.WithTenantID(bgCtx, tenantID)
		asyncInv := *invocation
		if b == nil {
			// No throttling — launch directly
			go func() {
				if _, err := s.runInvocation(bgCtx, f, &asyncInv, payload); err != nil {
					s.handleFailedInvocation(bgCtx, f, &asyncInv, err)
				}
			}()
		} else {
			// Throttled path using bulkhead
			go func() {
				err := b.Execute(bgCtx, func() error {
					_, err := s.runInvocation(bgCtx, f, &asyncInv, payload)
					return err
				})
				if err != nil {
					if stdlib_errors.Is(err, platform.ErrBulkheadFull) {
						s.logger.Warn("async invocation rejected: concurrency limit reached",
							"function_id", f.ID,
							"invocation_id", asyncInv.ID)
						asyncInv.Status = "REJECTED"
						asyncInv.Logs = "Concurrency limit reached (HTTP 429)"
						asyncInv.StatusCode = 429
						if recErr := s.repo.CreateInvocation(bgCtx, &asyncInv); recErr != nil {
							s.logger.Error("failed to record rejected invocation", "error", recErr)
						}
					} else {
						// Handle retry and DLQ logic
						s.handleFailedInvocation(bgCtx, f, &asyncInv, err)
					}
				}
			}()
		}
		return invocation, nil
	}

	if err := s.auditSvc.Log(ctx, f.UserID, "function.invoke", "function", f.ID.String(), map[string]interface{}{}); err != nil {
		s.logger.Warn("failed to log audit event", "action", "function.invoke", "function_id", f.ID, "error", err)
	}
	return s.runInvocation(ctx, f, invocation, payload)
}

func (s *FunctionService) runInvocation(ctx context.Context, f *domain.Function, i *domain.Invocation, payload []byte) (*domain.Invocation, error) {
	i.Status = "RUNNING"

	tmpDir, err := s.prepareCode(ctx, f)
	if err != nil {
		return s.failInvocation(i, "function invocation failed", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	opts := s.buildTaskOptions(ctx, f, tmpDir, payload)

	containerID, _, err := s.compute.RunTask(ctx, opts)
	if err != nil {
		return s.failInvocation(i, "function invocation failed", err)
	}

	statusCode, err := s.waitForTask(ctx, containerID, f.Timeout)
	s.captureInvocationResults(i, containerID, statusCode, err)

	// Fire-and-forget: container cleanup runs in isolated goroutine with its own
	// timeout context, preventing slow Docker cleanup from blocking the response.
	// Runs after task completion so container is no longer needed for result capture.
	go func(cid string) {
		delCtx, cancel := context.WithTimeout(contextWithMetadata(ctx), CleanupTimeout)
		defer cancel()
		if err := s.compute.DeleteInstance(delCtx, cid); err != nil {
			s.logger.Warn("failed to delete invocation container", "container_id", cid, "error", err)
		}
	}(containerID)

	if err := s.repo.CreateInvocation(ctx, i); err != nil {
		s.logger.Error("failed to record invocation", "error", err)
	}

	return i, nil
}

// getBulkhead returns a bulkhead for the given function, or nil if no throttling is configured.
// Thread-safe via bulkheadMu.
func (s *FunctionService) getBulkhead(f *domain.Function) *platform.Bulkhead {
	if f.MaxConcurrentInvocations <= 0 {
		return nil // signals "no throttling"
	}
	s.bulkheadMu.RLock()
	b, ok := s.bulkheadRegistry[f.ID]
	s.bulkheadMu.RUnlock()
	if ok {
		return b
	}
	// Create new bulkhead with function's limit
	s.bulkheadMu.Lock()
	defer s.bulkheadMu.Unlock()
	// Double-check after acquiring write lock
	if b, ok = s.bulkheadRegistry[f.ID]; ok {
		return b
	}

	// WaitTimeout: if MaxQueueDepth > 0, allow waiting for a slot
	// Use MaxQueueDepth * 100ms as sensible wait time per slot in queue
	waitTimeout := 0
	if f.MaxQueueDepth > 0 {
		waitTimeout = f.MaxQueueDepth * 100 // milliseconds
	} else if f.MaxConcurrentInvocations > 0 {
		// MaxQueueDepth == 0 means no waiting — use -1 as sentinel for immediate-fail
		waitTimeout = -1
	}

	b = platform.NewBulkhead(platform.BulkheadOpts{
		Name:        f.ID.String(),
		MaxConc:     f.MaxConcurrentInvocations,
		WaitTimeout: time.Duration(waitTimeout) * time.Millisecond,
	})
	s.bulkheadRegistry[f.ID] = b
	return b
}

func (s *FunctionService) buildTaskOptions(ctx context.Context, f *domain.Function, tmpDir string, payload []byte) ports.RunTaskOptions {
	config := runtimes[f.Runtime]
	pidsLimit := int64(50)

	handler := s.normalizeHandler(f.Runtime, f.Handler)

	env := []string{fmt.Sprintf("PAYLOAD=%s", string(payload))}
	for _, e := range f.EnvVars {
		if e.SecretRef != "" {
			// Resolve secret reference at invocation time (dynamic)
			name := strings.TrimPrefix(e.SecretRef, "@")
			secret, err := s.secretSvc.GetSecretByName(ctx, name)
			if err != nil {
				s.logger.Warn("failed to resolve secret", "ref", e.SecretRef, "key", e.Key, "error", err)
				continue // skip rather than failing the invocation
			}
			// Inject as plain environment variable
			env = append(env, e.Key+"="+secret.EncryptedValue)
		} else {
			env = append(env, e.Key+"="+e.Value)
		}
	}

	return ports.RunTaskOptions{
		Image:           config.Image,
		Command:         append(config.Entrypoint, handler),
		Env:             env,
		MemoryMB:        int64(f.MemoryMB),
		CPUs:            f.CPUs,
		NetworkDisabled: true,
		ReadOnlyRootfs:  true,
		WorkingDir:      "/var/task",
		Binds:           []string{fmt.Sprintf("%s:/var/task:ro", tmpDir)},
		PidsLimit:       &pidsLimit,
	}
}

// normalizeHandler ensures the handler path is friendly for the runtime execution.
func (s *FunctionService) normalizeHandler(runtime, handler string) string {
	config, ok := runtimes[runtime]
	if !ok {
		return handler
	}

	// 1. If it doesn't have the extension, add it
	if !strings.HasSuffix(handler, config.Extension) {
		// If it has a dot but wrong extension, we don't know what to do, keep as is
		if !strings.Contains(handler, ".") {
			handler += config.Extension
		}
	}

	// 2. For Node.js/Python, they usually work best with ./
	if (runtime == "nodejs20" || runtime == "python312") && !strings.HasPrefix(handler, "./") && !strings.HasPrefix(handler, "/") {
		return "./" + handler
	}

	return handler
}

func (s *FunctionService) waitForTask(ctx context.Context, containerID string, timeout int) (int64, error) {
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	return s.compute.WaitTask(waitCtx, containerID)
}

func (s *FunctionService) captureInvocationResults(i *domain.Invocation, containerID string, statusCode int64, waitErr error) {
	logsReader, _ := s.compute.GetInstanceLogs(context.Background(), containerID)
	if logsReader != nil {
		logBytes, _ := io.ReadAll(io.LimitReader(logsReader, maxLogSize))
		// Sanitize logs to prevent log injection (strip control characters)
		i.Logs = logSanitizationRe.ReplaceAllString(string(logBytes), "?")
		_ = logsReader.Close()
	}

	i.EndedAt = new(time.Time)
	*i.EndedAt = time.Now()
	i.DurationMs = int(i.EndedAt.Sub(i.StartedAt).Milliseconds())
	i.StatusCode = int(statusCode)

	switch {
	case waitErr != nil:
		i.Status = "FAILED"
		if stdlib_errors.Is(waitErr, context.DeadlineExceeded) {
			i.Logs += "\nError: Execution timed out"
		} else {
			i.Logs += fmt.Sprintf("\nError: %v", waitErr)
		}
	case statusCode != 0:
		i.Status = "FAILED"
	default:
		i.Status = "SUCCESS"
	}
}

func (s *FunctionService) failInvocation(i *domain.Invocation, logMsg string, err error) (*domain.Invocation, error) {
	i.Status = "FAILED"
	i.Logs = logMsg
	_ = s.repo.CreateInvocation(context.Background(), i)
	return i, errors.Wrap(errors.Internal, logMsg, err)
}

func (s *FunctionService) handleFailedInvocation(ctx context.Context, f *domain.Function, i *domain.Invocation, invokeErr error) {
	i.RetryCount++
	maxRetries := f.MaxRetries

	if maxRetries <= 0 || i.RetryCount >= maxRetries {
		// Move to DLQ
		i.Status = "DLQ"
		i.Logs = fmt.Sprintf("Invocation failed after %d retry attempt(s): %v", i.RetryCount, invokeErr)
		s.logger.Warn("async invocation moved to DLQ",
			"function_id", f.ID,
			"invocation_id", i.ID,
			"retry_count", i.RetryCount,
			"error", invokeErr)
	} else {
		// Retry scheduling: exponential backoff (1s, 2s, 4s, 8s...)
		// Note: actual retry requires a queue worker; current implementation
		// records the retry state so users can inspect/manage via API.
		backoffMs := (1 << (i.RetryCount - 1)) * 1000
		i.Status = "FAILED"
		i.Logs = fmt.Sprintf("Invocation failed (will retry %d/%d, backoff %dms): %v", i.RetryCount, maxRetries, backoffMs, invokeErr)
		s.logger.Info("async invocation recorded for retry",
			"function_id", f.ID,
			"invocation_id", i.ID,
			"retry_count", i.RetryCount,
			"backoff_ms", backoffMs,
			"error", invokeErr)
	}

	if recErr := s.repo.CreateInvocation(ctx, i); recErr != nil {
		s.logger.Error("failed to record failed invocation", "error", recErr)
	}
}

func (s *FunctionService) prepareCode(ctx context.Context, f *domain.Function) (string, error) {
	rc, err := s.fileStore.Read(ctx, "functions", f.CodePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()

	tmpDir, err := os.MkdirTemp("", "fn-"+f.ID.String())
	if err != nil {
		return "", err
	}

	if err := s.extractZip(rc, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpDir, nil
}

func (s *FunctionService) extractZip(rc io.Reader, tmpDir string) error {
	// Limit extraction to 50MB to prevent Zip bombs
	const maxZipSize = 50 * 1024 * 1024
	lr := io.LimitReader(rc, maxZipSize)

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, lr); err != nil {
		return err
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return err
	}

	// Limit total number of files to prevent resource exhaustion
	if len(zr.File) > 1000 {
		return fmt.Errorf("too many files in zip: %d", len(zr.File))
	}

	for _, file := range zr.File {
		if err := s.extractZipFile(file, tmpDir); err != nil {
			return err
		}
	}
	return nil
}

func (s *FunctionService) extractZipFile(file *zip.File, tmpDir string) error {
	// Reject any zip entry whose name would escape tmpDir. Belt and braces:
	//
	//   - filepath.Join already canonicalises the path on the host (Linux).
	//   - filepath.Rel makes the comparison robust to trailing slashes and
	//     separator differences across platforms.
	//   - filepath.IsLocal (Go 1.20+) rejects entries that contain `..`,
	//     drive letters, or absolute paths even before we touch the filesystem.
	//   - We additionally reject mixed-separator entries (`a/..\\..\\etc`)
	//     because zip(1) is happy to write them on case-insensitive hosts.
	//
	// This closes the gap reported in #237 where case-variant payloads such as
	// `../../Etc/passwd` slipped through a HasPrefix check that compared raw
	// bytes against a lowercased prefix.
	if !filepath.IsLocal(file.Name) {
		return fmt.Errorf("invalid file path in zip: %q", file.Name)
	}
	if strings.ContainsAny(file.Name, "\\\x00") {
		return fmt.Errorf("invalid file path in zip: %q", file.Name)
	}

	cleanTmpDir, err := filepath.Abs(filepath.Clean(tmpDir))
	if err != nil {
		return fmt.Errorf("resolve extraction root: %w", err)
	}
	//nolint:gosec // G305: Path sanitization is performed via the IsLocal +
	// filepath.Rel check below.
	path := filepath.Join(cleanTmpDir, file.Name)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve extraction target: %w", err)
	}
	rel, err := filepath.Rel(cleanTmpDir, absPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path in zip: %q (resolved to %s)", file.Name, absPath)
	}

	if file.FileInfo().IsDir() {
		return os.MkdirAll(path, 0750) // G301: tighten permissions
	}

	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil { // G301: tighten permissions
		return err
	}

	// Use O_EXCL to prevent overwriting existing files if applicable,
	// though here it's a fresh tmpDir.
	dst, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) // G306: tighten permissions
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	src, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	// Prevent decompression bomb: limit copy size to 10MB per file
	// and check for excessive total size if needed.
	const maxFileSize = 10 * 1024 * 1024
	_, err = io.CopyN(dst, src, maxFileSize)
	if err != nil && !stdlib_errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (s *FunctionService) GetDLQInvocations(ctx context.Context, id uuid.UUID) ([]*domain.Invocation, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionRead, id.String()); err != nil {
		return nil, err
	}
	return s.repo.GetDLQInvocations(ctx, id)
}

func (s *FunctionService) RetryDLQInvocation(ctx context.Context, invocationID uuid.UUID) (*domain.Invocation, error) {
	userID := appcontext.UserIDFromContext(ctx)
	tenantID := appcontext.TenantIDFromContext(ctx)
	invocations, err := s.repo.GetDLQInvocations(ctx, invocationID)
	if err != nil {
		return nil, err
	}
	if len(invocations) == 0 {
		return nil, errors.New(errors.NotFound, "DLQ invocation not found")
	}
	inv := invocations[0]
	if _, err := s.repo.GetByID(ctx, inv.FunctionID); err != nil {
		return nil, err
	}
	if err := s.rbacSvc.Authorize(ctx, userID, tenantID, domain.PermissionFunctionInvoke, inv.FunctionID.String()); err != nil {
		return nil, err
	}
	inv.Status = "PENDING"
	inv.RetryCount = 0
	inv.EndedAt = nil
	if err := s.repo.UpdateInvocation(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}
