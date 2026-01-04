// Package domain contains the core domain models for the cloud platform.
package domain

import (
	"time"

	"github.com/google/uuid"
)

type CacheEngine string

const (
	EngineRedis CacheEngine = "redis"
)

type CacheStatus string

const (
	CacheStatusCreating CacheStatus = "CREATING"
	CacheStatusRunning  CacheStatus = "RUNNING"
	CacheStatusStopped  CacheStatus = "STOPPED"
	CacheStatusDeleting CacheStatus = "DELETING"
	CacheStatusFailed   CacheStatus = "FAILED"
)

type Cache struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	Engine      CacheEngine
	Version     string
	Status      CacheStatus
	VpcID       *uuid.UUID
	ContainerID string
	Port        int
	Password    string
	MemoryMB    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
