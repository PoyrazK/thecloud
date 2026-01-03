package domain

import (
	"time"

	"github.com/google/uuid"
)

type QueueStatus string

const (
	QueueStatusActive   QueueStatus = "ACTIVE"
	QueueStatusDeleting QueueStatus = "DELETING"
)

type Queue struct {
	ID                uuid.UUID   `json:"id"`
	UserID            uuid.UUID   `json:"user_id"`
	Name              string      `json:"name"`
	ARN               string      `json:"arn"`
	VisibilityTimeout int         `json:"visibility_timeout"`
	RetentionDays     int         `json:"retention_days"`
	MaxMessageSize    int         `json:"max_message_size"`
	Status            QueueStatus `json:"status"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type Message struct {
	ID            uuid.UUID `json:"id"`
	QueueID       uuid.UUID `json:"queue_id"`
	Body          string    `json:"body"`
	ReceiptHandle string    `json:"receipt_handle,omitempty"`
	VisibleAt     time.Time `json:"visible_at"`
	ReceivedCount int       `json:"received_count"`
	CreatedAt     time.Time `json:"created_at"`
}
