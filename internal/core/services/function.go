package services

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

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

type FunctionService struct {
	repo      ports.FunctionRepository
	docker    ports.DockerClient
	fileStore ports.FileStore
	logger    *slog.Logger
}

func NewFunctionService(repo ports.FunctionRepository, docker ports.DockerClient, fileStore ports.FileStore, logger *slog.Logger) *FunctionService {
	return &FunctionService{
		repo:      repo,
		docker:    docker,
		fileStore: fileStore,
		logger:    logger,
	}
}

func (s *FunctionService) CreateFunction(ctx context.Context, name, runtime, handler string, code []byte) (*domain.Function, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, errors.New(errors.Unauthorized, "user not authenticated")
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
		Name:      name,
		Runtime:   runtime,
		Handler:   handler,
		CodePath:  codeKey,
		Timeout:   30,
		MemoryMB:  128,
		Status:    "ACTIVE",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}

	s.logger.Info("function created", "name", name, "runtime", runtime, "id", id)

	return f, nil
}
func (s *FunctionService) GetFunction(ctx context.Context, id uuid.UUID) (*domain.Function, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *FunctionService) ListFunctions(ctx context.Context) ([]*domain.Function, error) {
	userID := appcontext.UserIDFromContext(ctx)
	if userID == uuid.Nil {
		return nil, errors.New(errors.Unauthorized, "user not authenticated")
	}
	return s.repo.List(ctx, userID)
}

func (s *FunctionService) DeleteFunction(ctx context.Context, id uuid.UUID) error {
	f, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Async delete from file store
	go func() {
		_ = s.fileStore.Delete(context.Background(), "functions", f.CodePath)
	}()

	return nil
}

func (s *FunctionService) GetFunctionLogs(ctx context.Context, id uuid.UUID, limit int) ([]*domain.Invocation, error) {
	return s.repo.GetInvocations(ctx, id, limit)
}

func (s *FunctionService) InvokeFunction(ctx context.Context, id uuid.UUID, payload []byte, async bool) (*domain.Invocation, error) {
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
		go func() {
			_, _ = s.runInvocation(context.Background(), f, invocation, payload)
		}()
		return invocation, nil
	}

	return s.runInvocation(ctx, f, invocation, payload)
}

func (s *FunctionService) runInvocation(ctx context.Context, f *domain.Function, i *domain.Invocation, payload []byte) (*domain.Invocation, error) {
	i.Status = "RUNNING"

	config := runtimes[f.Runtime]

	// 1. Prepare code (Extraction)
	tmpDir, err := s.prepareCode(ctx, f)
	if err != nil {
		i.Status = "FAILED"
		i.Logs = fmt.Sprintf("Error preparing code: %v", err)
		_ = s.repo.CreateInvocation(context.Background(), i)
		return i, err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// 2. Configure Docker Task
	opts := ports.RunTaskOptions{
		Image:           config.Image,
		Command:         append(config.Entrypoint, f.Handler),
		Env:             []string{fmt.Sprintf("PAYLOAD=%s", string(payload))},
		MemoryMB:        int64(f.MemoryMB),
		CPUs:            0.5, // Default for now
		NetworkDisabled: true,
		ReadOnlyRootfs:  true,
		WorkingDir:      "/var/task",
		Binds:           []string{fmt.Sprintf("%s:/var/task:ro", tmpDir)},
	}

	// Set PidsLimit if possible
	pidsLimit := int64(50)
	opts.PidsLimit = &pidsLimit

	// 3. Run Container
	containerID, err := s.docker.RunTask(ctx, opts)
	if err != nil {
		i.Status = "FAILED"
		i.Logs = fmt.Sprintf("Error running task: %v", err)
		_ = s.repo.CreateInvocation(context.Background(), i)
		return i, err
	}
	defer func() {
		_ = s.docker.RemoveContainer(context.Background(), containerID)
	}()

	// 4. Wait for Completion
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(f.Timeout)*time.Second)
	defer cancel()

	statusCode, err := s.docker.WaitContainer(waitCtx, containerID)

	// 5. Capture Results
	logsReader, _ := s.docker.GetLogs(context.Background(), containerID)
	if logsReader != nil {
		logBytes, _ := io.ReadAll(logsReader)
		i.Logs = string(logBytes)
		_ = logsReader.Close()
	}

	i.EndedAt = new(time.Time)
	*i.EndedAt = time.Now()
	i.DurationMs = int(i.EndedAt.Sub(i.StartedAt).Milliseconds())
	i.StatusCode = int(statusCode)

	if err != nil {
		i.Status = "FAILED"
		if err == context.DeadlineExceeded {
			i.Logs += "\nError: Execution timed out"
		} else {
			i.Logs += fmt.Sprintf("\nError: %v", err)
		}
	} else if statusCode != 0 {
		i.Status = "FAILED"
	} else {
		i.Status = "SUCCESS"
	}

	if err := s.repo.CreateInvocation(context.Background(), i); err != nil {
		s.logger.Error("failed to record invocation", "error", err)
	}

	return i, nil
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

	// Copy to buffer because archive/zip needs ReaderAt
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, rc)
	if err != nil {
		return "", err
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return "", err
	}

	for _, file := range zr.File {
		path := filepath.Join(tmpDir, file.Name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return "", err
			}
			continue
		}
		// ...

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}

		dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return "", err
		}

		src, err := file.Open()
		if err != nil {
			_ = dst.Close()
			return "", err
		}

		_, err = io.Copy(dst, src)
		_ = src.Close()
		_ = dst.Close()
		if err != nil {
			return "", err
		}
	}

	return tmpDir, nil
}
