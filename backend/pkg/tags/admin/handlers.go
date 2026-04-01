package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"openreplay/backend/pkg/analytics/filters"
	"openreplay/backend/pkg/logger"
	"openreplay/backend/pkg/server/api"
)

type handlersImpl struct {
	log      logger.Logger
	service  TagService
	handlers []*api.Description
}

func NewHandlers(log logger.Logger, req api.RequestHandler, service TagService) (api.Handlers, error) {
	h := &handlersImpl{
		log:     log,
		service: service,
	}
	h.handlers = []*api.Description{
		{"/{project}/tags", "POST", req.HandleWithBody(h.createTag), []string{api.DATA_MANAGEMENT}, "create_tag"},
		{"/{project}/tags", "GET", req.Handle(h.listTags), []string{api.DATA_MANAGEMENT}, api.DoNotTrack},
		{"/{project}/tags/{tagId}", "PUT", req.HandleWithBody(h.updateTag), []string{api.DATA_MANAGEMENT}, "update_tag"},
		{"/{project}/tags/{tagId}", "DELETE", req.Handle(h.deleteTag), []string{api.DATA_MANAGEMENT}, "delete_tag"},
	}
	return h, nil
}

func (h *handlersImpl) GetAll() []*api.Description {
	return h.handlers
}

func (h *handlersImpl) createTag(r *api.RequestContext) (any, int, error) {
	projID, err := r.GetProjectID()
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to get project ID: %s", err)
		return nil, http.StatusBadRequest, err
	}

	var req CreateTagRequest
	if err := json.Unmarshal(r.Body, &req); err != nil {
		h.log.Error(r.Request.Context(), "failed to parse create tag request: %s", err)
		return nil, http.StatusBadRequest, err
	}

	if err := filters.ValidateStruct(req); err != nil {
		h.log.Error(r.Request.Context(), "validation failed for create tag request: %s", err)
		return nil, http.StatusBadRequest, err
	}

	tagID, err := h.service.Create(r.Request.Context(), projID, &req)
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to create tag for project %d: %s", projID, err)
		return nil, http.StatusInternalServerError, err
	}

	return tagID, 0, nil
}

func (h *handlersImpl) listTags(r *api.RequestContext) (any, int, error) {
	projID, err := r.GetProjectID()
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to get project ID: %s", err)
		return nil, http.StatusBadRequest, err
	}

	query := r.Request.URL.Query()
	limit := 10
	page := 1
	if val := query.Get("limit"); val != "" {
		if l, err := strconv.Atoi(val); err == nil && l > 0 {
			limit = l
		}
	}
	if val := query.Get("page"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			page = p
		}
	}
	offset := (page - 1) * limit

	resp, err := h.service.List(r.Request.Context(), projID, limit, offset)
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to list tags for project %d: %s", projID, err)
		return nil, http.StatusInternalServerError, err
	}

	return resp, 0, nil
}

func (h *handlersImpl) updateTag(r *api.RequestContext) (any, int, error) {
	projID, err := r.GetProjectID()
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to get project ID: %s", err)
		return nil, http.StatusBadRequest, err
	}

	tagIDStr, err := api.GetParam(r.Request, "tagId")
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to get tag ID: %s", err)
		return nil, http.StatusBadRequest, err
	}
	tagID, err := strconv.Atoi(tagIDStr)
	if err != nil {
		h.log.Error(r.Request.Context(), "invalid tag ID: %s", err)
		return nil, http.StatusBadRequest, err
	}

	var req UpdateTagRequest
	if err := json.Unmarshal(r.Body, &req); err != nil {
		h.log.Error(r.Request.Context(), "failed to parse update tag request: %s", err)
		return nil, http.StatusBadRequest, err
	}

	if err := filters.ValidateStruct(req); err != nil {
		h.log.Error(r.Request.Context(), "validation failed for update tag request: %s", err)
		return nil, http.StatusBadRequest, err
	}

	if err := h.service.Update(r.Request.Context(), projID, tagID, &req); err != nil {
		if err == ErrNoFieldsToUpdate {
			return nil, http.StatusBadRequest, err
		}
		h.log.Error(r.Request.Context(), "failed to update tag %d for project %d: %s", tagID, projID, err)
		return nil, http.StatusInternalServerError, err
	}

	return map[string]interface{}{"success": true}, 0, nil
}

func (h *handlersImpl) deleteTag(r *api.RequestContext) (any, int, error) {
	projID, err := r.GetProjectID()
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to get project ID: %s", err)
		return nil, http.StatusBadRequest, err
	}

	tagIDStr, err := api.GetParam(r.Request, "tagId")
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to get tag ID: %s", err)
		return nil, http.StatusBadRequest, err
	}
	tagID, err := strconv.Atoi(tagIDStr)
	if err != nil {
		h.log.Error(r.Request.Context(), "invalid tag ID: %s", err)
		return nil, http.StatusBadRequest, err
	}

	if err := h.service.Delete(r.Request.Context(), projID, tagID); err != nil {
		h.log.Error(r.Request.Context(), "failed to delete tag %d for project %d: %s", tagID, projID, err)
		return nil, http.StatusInternalServerError, err
	}

	return map[string]interface{}{"success": true}, 0, nil
}
