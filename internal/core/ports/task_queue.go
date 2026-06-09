// Package ports defines service and repository interfaces.
package ports

import (
	"context"
)

// TaskQueue defines a simple producer-consumer interface for background work distribution.
// Producers (services) only need this interface to enqueue jobs.
type TaskQueue interface {
	// Enqueue adds a serializable payload to the specified background processing queue.
	Enqueue(ctx context.Context, queueName string, payload interface{}) error
	// Dequeue pulls the next available raw message string from the background processing queue.
	//
	// Deprecated: parallel consumers should use DurableTaskQueue.Receive instead.
	Dequeue(ctx context.Context, queueName string) (string, error)
}

// DurableMessage represents a message read from a durable queue.
// The consumer must call Ack after successful processing; otherwise
// the message remains pending and will be redelivered after a timeout.
type DurableMessage struct {
	// ID is the stream-assigned message identifier (e.g. Redis Stream ID).
	ID string
	// Payload is the raw JSON string of the job.
	Payload string
	// Queue is the queue (stream) name this message came from.
	Queue string
}

// DurableTaskQueue extends TaskQueue with at-least-once delivery semantics.
// It uses consumer groups so that each message is delivered to exactly one
// consumer within the group, and requires explicit acknowledgement.
type DurableTaskQueue interface {
	TaskQueue

	// EnsureGroup creates the consumer group for the given queue if it does not
	// already exist. Must be called once at startup before Receive.
	EnsureGroup(ctx context.Context, queueName, groupName string) error

	// Receive reads the next available message from the queue for the given
	// consumer group and consumer name. It blocks up to the queue's configured
	// poll interval. Returns nil message and nil error when no message is
	// available (timeout).
	Receive(ctx context.Context, queueName, groupName, consumerName string) (*DurableMessage, error)

	// Ack acknowledges successful processing of a message. After Ack the
	// message will not be redelivered.
	Ack(ctx context.Context, queueName, groupName, messageID string) error

	// Nack signals that the consumer failed to process the message.
	// It relinquishes the current delivery WITHOUT re-queuing or creating
	// a new message ID. The message remains in the pending entries list
	// and will be reclaimed by ReclaimStale after the idle timeout.
	// Implementations MUST NOT create duplicate live copies of the message.
	Nack(ctx context.Context, queueName, groupName, messageID string) error

	// ReclaimStale claims messages that have been pending longer than the
	// given idle threshold and returns them. This allows a healthy consumer
	// to pick up work abandoned by a crashed peer.
	ReclaimStale(ctx context.Context, queueName, groupName, consumerName string, minIdleMs int64, count int64) ([]DurableMessage, error)
}
