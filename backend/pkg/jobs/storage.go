package jobs

import (
	"context"
	"fmt"
	"time"
)

func (j *jobsImpl) Create(projectID uint32, userID string) (*Job, error) {
	startAt := midnightTomorrowUTC()
	description := fmt.Sprintf("Delete user sessions of userId = %s", userID)

	var (
		jobID       int
		status      string
		action      string
		referenceID string
		desc        string
		createdAt   time.Time
		updatedAt   *time.Time
		startAtDB   time.Time
		errors      *string
	)

	err := j.db.QueryRow(`
		INSERT INTO public.jobs (project_id, description, status, action, reference_id, start_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING job_id, description, status, action, reference_id, created_at, updated_at, start_at, errors
	`, projectID, description, StatusScheduled, ActionDeleteUserData, userID, startAt,
	).Scan(&jobID, &desc, &status, &action, &referenceID, &createdAt, &updatedAt, &startAtDB, &errors)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return &Job{
		JobID:       jobID,
		Description: desc,
		Status:      status,
		ProjectID:   projectID,
		Action:      action,
		ReferenceID: referenceID,
		CreatedAt:   toMillis(createdAt),
		UpdatedAt:   toMillisPtr(updatedAt),
		StartAt:     toMillis(startAtDB),
		Errors:      errors,
	}, nil
}

func (j *jobsImpl) Get(jobID int, projectID uint32) (*Job, error) {
	var (
		desc        string
		status      string
		action      string
		referenceID string
		createdAt   time.Time
		updatedAt   *time.Time
		startAt     time.Time
		errors      *string
	)

	err := j.db.QueryRow(`
		SELECT description, status, action, reference_id, created_at, updated_at, start_at, errors
		FROM public.jobs
		WHERE job_id = $1 AND project_id = $2
	`, jobID, projectID,
	).Scan(&desc, &status, &action, &referenceID, &createdAt, &updatedAt, &startAt, &errors)
	if err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}

	return &Job{
		JobID:       jobID,
		Description: desc,
		Status:      status,
		ProjectID:   projectID,
		Action:      action,
		ReferenceID: referenceID,
		CreatedAt:   toMillis(createdAt),
		UpdatedAt:   toMillisPtr(updatedAt),
		StartAt:     toMillis(startAt),
		Errors:      errors,
	}, nil
}

func (j *jobsImpl) GetAll(projectID uint32, limit int, page int) ([]*Job, error) {
	offset := (page - 1) * limit
	rows, err := j.db.Query(`
		SELECT job_id, description, status, action, reference_id, created_at, updated_at, start_at, errors
		FROM public.jobs
		WHERE project_id = $1
		ORDER BY job_id DESC
		LIMIT $2 OFFSET $3
	`, projectID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	result := make([]*Job, 0)
	for rows.Next() {
		var (
			jobID       int
			desc        string
			status      string
			action      string
			referenceID string
			createdAt   time.Time
			updatedAt   *time.Time
			startAt     time.Time
			errors      *string
		)
		if err := rows.Scan(&jobID, &desc, &status, &action, &referenceID, &createdAt, &updatedAt, &startAt, &errors); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		result = append(result, &Job{
			JobID:       jobID,
			Description: desc,
			Status:      status,
			ProjectID:   projectID,
			Action:      action,
			ReferenceID: referenceID,
			CreatedAt:   toMillis(createdAt),
			UpdatedAt:   toMillisPtr(updatedAt),
			StartAt:     toMillis(startAt),
			Errors:      errors,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating jobs: %w", err)
	}

	return result, nil
}

func (j *jobsImpl) Cancel(jobID int, projectID uint32) (*Job, error) {
	var (
		desc        string
		status      string
		action      string
		referenceID string
		createdAt   time.Time
		updatedAt   *time.Time
		startAt     time.Time
		errorsCol   *string
	)

	err := j.db.QueryRow(`
		UPDATE public.jobs
		SET status = $1, updated_at = timezone('utc'::text, now())
		WHERE job_id = $2 AND project_id = $3
			AND status NOT IN ('completed', 'cancelled')
		RETURNING description, status, action, reference_id, created_at, updated_at, start_at, errors
	`, StatusCancelled, jobID, projectID,
	).Scan(&desc, &status, &action, &referenceID, &createdAt, &updatedAt, &startAt, &errorsCol)
	if err != nil {
		existing, getErr := j.Get(jobID, projectID)
		if getErr != nil {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("the requested job has already been %s", existing.Status)
	}

	return &Job{
		JobID:       jobID,
		Description: desc,
		Status:      status,
		ProjectID:   projectID,
		Action:      action,
		ReferenceID: referenceID,
		CreatedAt:   toMillis(createdAt),
		UpdatedAt:   toMillisPtr(updatedAt),
		StartAt:     toMillis(startAt),
		Errors:      errorsCol,
	}, nil
}

func (j *jobsImpl) ExecuteScheduledJobs() error {
	rows, err := j.db.Query(`
		SELECT job_id, project_id, description, status, action, reference_id, created_at, updated_at, start_at, errors
		FROM public.jobs
		WHERE status = $1 AND start_at <= (now() AT TIME ZONE 'utc')
	`, StatusScheduled)
	if err != nil {
		return fmt.Errorf("failed to query scheduled jobs: %w", err)
	}
	defer rows.Close()

	var jobsToExecute []*Job
	for rows.Next() {
		var (
			jobID       int
			projID      uint32
			desc        string
			status      string
			action      string
			referenceID string
			createdAt   time.Time
			updatedAt   *time.Time
			startAt     time.Time
			errors      *string
		)
		if err := rows.Scan(&jobID, &projID, &desc, &status, &action, &referenceID, &createdAt, &updatedAt, &startAt, &errors); err != nil {
			j.log.Error(context.Background(), "failed to scan scheduled job: %v", err)
			continue
		}
		jobsToExecute = append(jobsToExecute, &Job{
			JobID:       jobID,
			ProjectID:   projID,
			Description: desc,
			Status:      status,
			Action:      action,
			ReferenceID: referenceID,
			CreatedAt:   toMillis(createdAt),
			UpdatedAt:   toMillisPtr(updatedAt),
			StartAt:     toMillis(startAt),
			Errors:      errors,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating scheduled jobs: %w", err)
	}

	for _, job := range jobsToExecute {
		j.log.Info(context.Background(), "executing jobId:%d", job.JobID)
		j.executeJob(job)
	}

	return nil
}

func (j *jobsImpl) executeJob(job *Job) {
	ctx := context.Background()

	if job.Action != ActionDeleteUserData {
		errMsg := fmt.Sprintf("unsupported action: %s", job.Action)
		j.log.Error(ctx, errMsg)
		j.updateJobStatus(job.JobID, StatusFailed, &errMsg)
		return
	}

	sessionIDs, err := j.getSessionIDsByUserID(job.ProjectID, job.ReferenceID)
	if err != nil {
		errMsg := fmt.Sprintf("failed to get sessions: %v", err)
		j.log.Error(ctx, errMsg)
		j.updateJobStatus(job.JobID, StatusFailed, &errMsg)
		return
	}

	if len(sessionIDs) > 0 {
		j.log.Info(ctx, "deleting %d sessions for jobId:%d", len(sessionIDs), job.JobID)
		if err := j.deleteSessionsByIDs(sessionIDs); err != nil {
			errMsg := fmt.Sprintf("failed to delete sessions: %v", err)
			j.log.Error(ctx, errMsg)
			j.updateJobStatus(job.JobID, StatusFailed, &errMsg)
			return
		}
	}

	j.log.Info(ctx, "job completed jobId:%d", job.JobID)
	j.updateJobStatus(job.JobID, StatusCompleted, nil)
}

func (j *jobsImpl) updateJobStatus(jobID int, status string, errMsg *string) {
	if err := j.db.Exec(`
		UPDATE public.jobs
		SET status = $1, errors = $2, updated_at = timezone('utc'::text, now())
		WHERE job_id = $3
	`, status, errMsg, jobID); err != nil {
		j.log.Error(context.Background(), "failed to update job %d status: %v", jobID, err)
	}
}

func (j *jobsImpl) getSessionIDsByUserID(projectID uint32, userID string) ([]int64, error) {
	rows, err := j.db.Query(`
		SELECT session_id
		FROM public.sessions
		WHERE project_id = $1 AND user_id = $2
		LIMIT 1000
	`, projectID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (j *jobsImpl) deleteSessionsByIDs(sessionIDs []int64) error {
	if len(sessionIDs) == 0 {
		return nil
	}
	args := make([]interface{}, len(sessionIDs))
	placeholders := ""
	for i, id := range sessionIDs {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf("DELETE FROM public.sessions WHERE session_id IN (%s)", placeholders)
	return j.db.Exec(query, args...)
}
