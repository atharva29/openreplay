package jobs

import (
	"openreplay/backend/pkg/db/postgres/pool"
	"openreplay/backend/pkg/logger"
)

type Jobs interface {
	Create(projectID uint32, userID string) (*Job, error)
	Get(jobID int, projectID uint32) (*Job, error)
	GetAll(projectID uint32) ([]*Job, error)
	Cancel(jobID int, projectID uint32) (*Job, error)
	ExecuteScheduledJobs() error
}

type jobsImpl struct {
	log logger.Logger
	db  pool.Pool
}

func New(log logger.Logger, db pool.Pool) Jobs {
	return &jobsImpl{log: log, db: db}
}
