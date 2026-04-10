package api_key

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"openreplay/backend/pkg/assist/proxy"
	"openreplay/backend/pkg/server/api"
)

func (h *handlersImpl) assistHandlers(req api.RequestHandler) []*api.Description {
	return []*api.Description{
		{"/v1/assist/credentials", "GET", req.Handle(h.getAssistCredentials), []string{api.PublicKeyPermission}, api.DoNotTrack},
		{"/v1/projects/{project}/assist/sessions", "GET", req.Handle(h.getAssistSessions), []string{api.PublicKeyPermission}, api.DoNotTrack},
		{"/v1/projects/{project}/assist/sessions", "POST", req.HandleWithBody(h.searchAssistSessions), []string{api.PublicKeyPermission}, api.DoNotTrack},
	}
}

type iceServer struct {
	URLs       string `json:"urls"`
	Username   string `json:"username,omitempty"`
	Credential string `json:"credential,omitempty"`
}

func generateSalt() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *handlersImpl) getIceServers() ([]iceServer, error) {
	iceServersStr := h.cfg.IceServers
	if iceServersStr == "" {
		return nil, fmt.Errorf("ICE servers not configured")
	}

	servers := strings.Split(iceServersStr, "|")
	secret := h.cfg.AssistSecret
	result := make([]iceServer, 0, len(servers))

	if secret != "" {
		ttl := h.cfg.AssistTTL * 3600
		timestamp := time.Now().Unix() + int64(ttl)
		user := generateSalt()
		username := fmt.Sprintf("%d:%s", timestamp, user)

		mac := hmac.New(sha1.New, []byte(secret))
		mac.Write([]byte(username))
		credential := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		for _, s := range servers {
			url := strings.SplitN(s, ",", 2)[0]
			if strings.HasPrefix(strings.ToLower(url), "stun") {
				result = append(result, iceServer{URLs: url})
			} else {
				result = append(result, iceServer{
					URLs:       url,
					Username:   username,
					Credential: credential,
				})
			}
		}
	} else {
		for _, s := range servers {
			parts := strings.SplitN(s, ",", 4)
			if len(parts) == 3 {
				result = append(result, iceServer{
					URLs:       parts[0],
					Username:   parts[1],
					Credential: parts[2],
				})
			} else {
				result = append(result, iceServer{URLs: parts[0]})
			}
		}
	}

	return result, nil
}

func unwrapAssistData(resp interface{}) interface{} {
	if m, ok := resp.(map[string]interface{}); ok {
		if inner, ok := m["data"]; ok {
			return inner
		}
	}
	return resp
}

func (h *handlersImpl) getAssistCredentials(r *api.RequestContext) (any, int, error) {
	servers, err := h.getIceServers()
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get assist credentials")
	}
	return servers, 0, nil
}

func (h *handlersImpl) getAssistSessions(r *api.RequestContext) (any, int, error) {
	projID, statusCode, err := h.resolveProjectID(r)
	if err != nil {
		return nil, statusCode, err
	}

	userId := r.Request.URL.Query().Get("userId")

	req := &proxy.GetLiveSessionsRequest{
		Sort:  "timestamp",
		Order: "desc",
		Limit: 10,
		Page:  1,
	}

	if userId != "" {
		req.Filters = []interface{}{
			map[string]interface{}{
				"name":     "userId",
				"operator": "is",
				"value":    []string{userId},
			},
		}
	}

	resp, err := h.assist.GetLiveSessionsWS(projID, req)
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to get assist sessions: %s", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to get assist sessions")
	}

	return unwrapAssistData(resp), 0, nil
}

func (h *handlersImpl) searchAssistSessions(r *api.RequestContext) (any, int, error) {
	projID, statusCode, err := h.resolveProjectID(r)
	if err != nil {
		return nil, statusCode, err
	}

	req := &proxy.GetLiveSessionsRequest{}
	if err := json.Unmarshal(r.Body, req); err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("invalid JSON body")
	}

	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Sort == "" {
		req.Sort = "timestamp"
	}
	if req.Order == "" {
		req.Order = "desc"
	}

	resp, err := h.assist.GetLiveSessionsWS(projID, req)
	if err != nil {
		h.log.Error(r.Request.Context(), "failed to search assist sessions: %s", err)
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to search assist sessions")
	}

	return unwrapAssistData(resp), 0, nil
}
