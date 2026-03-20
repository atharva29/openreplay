package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"openreplay/backend/pkg/db/postgres/pool"
	"openreplay/backend/pkg/logger"
)

var (
	ErrTagNotFound      = errors.New("tag not found")
	ErrNoFieldsToUpdate = errors.New("no fields to update")
)

type TagService interface {
	Create(ctx context.Context, projectID uint32, req *CreateTagRequest) (int, error)
	List(ctx context.Context, projectID uint32) ([]TagResponse, error)
	Update(ctx context.Context, projectID uint32, tagID int, req *UpdateTagRequest) error
	Delete(ctx context.Context, projectID uint32, tagID int) error
}

type tagServiceImpl struct {
	log    logger.Logger
	pgconn pool.Pool
}

func NewTagService(log logger.Logger, pgconn pool.Pool) TagService {
	return &tagServiceImpl{
		log:    log,
		pgconn: pgconn,
	}
}

func (s *tagServiceImpl) Create(ctx context.Context, projectID uint32, req *CreateTagRequest) (int, error) {
	const query = `
		INSERT INTO public.tags (project_id, name, selector, ignore_click_rage, ignore_dead_click, location)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING tag_id
	`

	name := strings.TrimSpace(req.Name)
	var tagID int
	err := s.pgconn.QueryRow(query, projectID, name, req.Selector, req.IgnoreClickRage, req.IgnoreDeadClick, req.Location).Scan(&tagID)
	if err != nil {
		s.log.Error(ctx, "failed to create tag: %s", err)
		return 0, fmt.Errorf("create tag: %s", err)
	}
	return tagID, nil
}

func (s *tagServiceImpl) List(ctx context.Context, projectID uint32) ([]TagResponse, error) {
	const query = `
		SELECT tag_id, name, selector, ignore_click_rage, ignore_dead_click, location
		FROM public.tags
		WHERE project_id = $1 AND deleted_at IS NULL
		ORDER BY name
	`

	rows, err := s.pgconn.Query(query, projectID)
	if err != nil {
		s.log.Error(ctx, "failed to list tags: %s", err)
		return nil, fmt.Errorf("list tags: %s", err)
	}
	defer rows.Close()

	tags := make([]TagResponse, 0)
	for rows.Next() {
		var t TagResponse
		if err := rows.Scan(&t.TagID, &t.Name, &t.Selector, &t.IgnoreClickRage, &t.IgnoreDeadClick, &t.Location); err != nil {
			s.log.Error(ctx, "failed to scan tag row: %s", err)
			return nil, fmt.Errorf("scan tag: %s", err)
		}
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		s.log.Error(ctx, "failed to iterate tag rows: %s", err)
		return nil, fmt.Errorf("iterate tags: %s", err)
	}
	return tags, nil
}

func (s *tagServiceImpl) Update(ctx context.Context, projectID uint32, tagID int, req *UpdateTagRequest) error {
	setClauses := make([]string, 0, 2)
	params := make([]interface{}, 0, 4)
	idx := 1

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", idx))
		params = append(params, name)
		idx++
	}
	if req.Location != nil {
		setClauses = append(setClauses, fmt.Sprintf("location = $%d", idx))
		params = append(params, *req.Location)
		idx++
	}

	if len(setClauses) == 0 {
		return ErrNoFieldsToUpdate
	}

	query := fmt.Sprintf(`
		UPDATE public.tags
		SET %s
		WHERE tag_id = $%d AND project_id = $%d AND deleted_at IS NULL
	`, strings.Join(setClauses, ", "), idx, idx+1)
	params = append(params, tagID, projectID)

	if err := s.pgconn.Exec(query, params...); err != nil {
		s.log.Error(ctx, "failed to update tag %d: %s", tagID, err)
		return fmt.Errorf("update tag: %s", err)
	}
	return nil
}

func (s *tagServiceImpl) Delete(ctx context.Context, projectID uint32, tagID int) error {
	const query = `
		UPDATE public.tags
		SET deleted_at = now() AT TIME ZONE 'utc'
		WHERE tag_id = $1 AND project_id = $2 AND deleted_at IS NULL
	`

	if err := s.pgconn.Exec(query, tagID, projectID); err != nil {
		s.log.Error(ctx, "failed to delete tag %d: %s", tagID, err)
		return fmt.Errorf("delete tag: %s", err)
	}
	return nil
}
