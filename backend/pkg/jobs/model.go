package jobs

import (
	"errors"
	"time"
)

const (
	ActionDeleteUserData = "delete_user_data"

	StatusScheduled = "scheduled"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

var ErrJobNotFound = errors.New("job not found")

type Job struct {
	JobID       int     `json:"jobId"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	ProjectID   uint32  `json:"projectId"`
	Action      string  `json:"action"`
	ReferenceID string  `json:"referenceId"`
	CreatedAt   int64   `json:"createdAt"`
	UpdatedAt   *int64  `json:"updatedAt"`
	StartAt     int64   `json:"startAt"`
	Errors      *string `json:"errors"`
}

func toMillis(t time.Time) int64 {
	return t.UnixMilli()
}

func toMillisPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	ms := t.UnixMilli()
	return &ms
}

func midnightTomorrowUTC() time.Time {
	now := time.Now().UTC()
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.UTC)
}
