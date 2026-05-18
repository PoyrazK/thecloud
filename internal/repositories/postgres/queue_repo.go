// Package postgres provides PostgreSQL-backed repository implementations.
package postgres

import (
	"context"
	"time"

	stdlib_errors "errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/poyrazk/thecloud/internal/errors"
)

// PostgresQueueRepository implements a message queue system using PostgreSQL as the backend.
// It supports visibility timeouts and message locking for concurrent processing.
type PostgresQueueRepository struct {
	db DB
}

// NewPostgresQueueRepository creates a new instance of PostgresQueueRepository.
func NewPostgresQueueRepository(db DB) ports.QueueRepository {
	return &PostgresQueueRepository{db: db}
}

// Create provisions a new message queue entity.
func (r *PostgresQueueRepository) Create(ctx context.Context, q *domain.Queue) error {
	query := `
		INSERT INTO queues (id, user_id, tenant_id, name, arn, visibility_timeout, retention_days, max_message_size, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, query,
		q.ID, q.UserID, q.TenantID, q.Name, q.ARN, q.VisibilityTimeout, q.RetentionDays, q.MaxMessageSize, q.Status, q.CreatedAt, q.UpdatedAt)
	return err
}

// GetByID retrieves a queue definition by its unique identifier.
func (r *PostgresQueueRepository) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Queue, error) {
	query := `SELECT id, user_id, tenant_id, name, arn, visibility_timeout, retention_days, max_message_size, status, created_at, updated_at FROM queues WHERE id = $1 AND tenant_id = $2`
	return r.scanQueue(r.db.QueryRow(ctx, query, id, tenantID))
}

// GetByName retrieves a queue definition by its user-defined name.
func (r *PostgresQueueRepository) GetByName(ctx context.Context, name string, tenantID uuid.UUID) (*domain.Queue, error) {
	query := `SELECT id, user_id, tenant_id, name, arn, visibility_timeout, retention_days, max_message_size, status, created_at, updated_at FROM queues WHERE name = $1 AND tenant_id = $2`
	return r.scanQueue(r.db.QueryRow(ctx, query, name, tenantID))
}

// List returns all queues for the specified tenant.
func (r *PostgresQueueRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Queue, error) {
	query := `SELECT id, user_id, tenant_id, name, arn, visibility_timeout, retention_days, max_message_size, status, created_at, updated_at FROM queues WHERE tenant_id = $1`
	rows, err := r.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	return r.scanQueues(rows)
}

func (r *PostgresQueueRepository) scanQueue(row pgx.Row) (*domain.Queue, error) {
	q := &domain.Queue{}
	var status string
	err := row.Scan(&q.ID, &q.UserID, &q.TenantID, &q.Name, &q.ARN, &q.VisibilityTimeout, &q.RetentionDays, &q.MaxMessageSize, &status, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Return nil, nil when not found as per previous behavior
		}
		return nil, err
	}
	q.Status = domain.QueueStatus(status)
	return q, nil
}

func (r *PostgresQueueRepository) scanQueues(rows pgx.Rows) ([]*domain.Queue, error) {
	defer rows.Close()
	var queues []*domain.Queue
	for rows.Next() {
		q, err := r.scanQueue(rows)
		if err != nil {
			return nil, err
		}
		queues = append(queues, q)
	}
	return queues, nil
}

// Delete removes a queue definition; note that dependent messages should be handled by DB constraints.
func (r *PostgresQueueRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	cmd, err := r.db.Exec(ctx, "DELETE FROM queues WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "queue not found")
	}
	return nil
}

// SendMessage appends a new message to the queue.
// It handles clock skew by potentially using the database server time (NOW()).
func (r *PostgresQueueRepository) SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error) {
	m := &domain.Message{
		ID:        uuid.New(),
		QueueID:   queueID,
		Body:      body,
		VisibleAt: time.Now(),
		CreatedAt: time.Now().UTC(),
	}
	query := `INSERT INTO queue_messages (id, queue_id, body, visible_at, created_at) VALUES ($1, $2, $3, $4, $5)`
	var err error
	if time.Since(m.VisibleAt) < time.Second && time.Until(m.VisibleAt) < time.Second {
		query = `INSERT INTO queue_messages (id, queue_id, body, visible_at, created_at) VALUES ($1, $2, $3, NOW(), NOW())`
		_, err = r.db.Exec(ctx, query, m.ID, m.QueueID, m.Body)
	} else {
		_, err = r.db.Exec(ctx, query, m.ID, m.QueueID, m.Body, m.VisibleAt, m.CreatedAt)
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ReceiveMessages polls the queue for available messages and marks them as "in-flight"
// by setting a visibility timeout based on the provided parameter.
func (r *PostgresQueueRepository) ReceiveMessages(ctx context.Context, queueID uuid.UUID, maxMessages, visibilityTimeout int) ([]*domain.Message, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Select visible messages and lock them
	query := `
		SELECT id, queue_id, body, received_count, created_at 
		FROM queue_messages 
		WHERE queue_id = $1 AND visible_at <= NOW() 
		FOR UPDATE SKIP LOCKED 
		LIMIT $2
	`
	rows, err := tx.Query(ctx, query, queueID, maxMessages)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*domain.Message
	now := time.Now()
	for rows.Next() {
		m := &domain.Message{}
		if err := rows.Scan(&m.ID, &m.QueueID, &m.Body, &m.ReceivedCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		m.ReceiptHandle = uuid.New().String()
		m.VisibleAt = now.Add(time.Duration(visibilityTimeout) * time.Second)
		m.ReceivedCount++
		messages = append(messages, m)
	}
	rows.Close() // Close before update

	// 2. Update status of received messages
	updateQuery := `UPDATE queue_messages SET receipt_handle = $1, visible_at = $2, received_count = received_count + 1 WHERE id = $3`
	for _, m := range messages {
		_, err := tx.Exec(ctx, updateQuery, m.ReceiptHandle, m.VisibleAt, m.ID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return messages, nil
}

// DeleteMessage permanently removes a message from the queue after successful processing.
func (r *PostgresQueueRepository) DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error {
	result, err := r.db.Exec(ctx, "DELETE FROM queue_messages WHERE queue_id = $1 AND receipt_handle = $2", queueID, receiptHandle)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "message not found or receipt handle expired")
	}
	return nil
}

// PurgeMessages deletes all messages currently in the specified queue.
func (r *PostgresQueueRepository) PurgeMessages(ctx context.Context, queueID uuid.UUID) (int64, error) {
	result, err := r.db.Exec(ctx, "DELETE FROM queue_messages WHERE queue_id = $1", queueID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// GetQueueStats returns the count of visible (ready) and in-flight (waiting) messages.
func (r *PostgresQueueRepository) GetQueueStats(ctx context.Context, queueID uuid.UUID) (int, int, error) {
	var visible, inFlight int
	query := `
		SELECT 
			COUNT(*) FILTER (WHERE visible_at <= NOW()) as visible,
			COUNT(*) FILTER (WHERE visible_at > NOW()) as in_flight
		FROM queue_messages 
		WHERE queue_id = $1
	`
	err := r.db.QueryRow(ctx, query, queueID).Scan(&visible, &inFlight)
	return visible, inFlight, err
}
