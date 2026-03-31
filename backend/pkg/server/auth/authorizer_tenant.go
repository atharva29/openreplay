package auth

import (
	"fmt"
	"net/http"

	"openreplay/backend/pkg/server/tenant"
	"openreplay/backend/pkg/server/user"
)

func (a *authImpl) isAuthorizedApiKey(apiKey string, projectKey string) (*tenant.Tenant, error) {
	if a.tenants == nil {
		return nil, fmt.Errorf("tenants service is not configured")
	}
	if a.projects == nil {
		return nil, fmt.Errorf("projects service is not configured")
	}

	dbTenant, err := a.tenants.GetTenantByApiKey(apiKey)
	if err != nil {
		return nil, err
	}

	_, err = a.projects.GetProjectByKeyAndTenant(projectKey, dbTenant.TenantID)
	if err != nil {
		return nil, fmt.Errorf("project not found or does not belong to this tenant")
	}

	return dbTenant, nil
}

func (a *authImpl) isAuthorizedApiKeyOnly(apiKey string) (*tenant.Tenant, error) {
	if a.tenants == nil {
		return nil, fmt.Errorf("tenants service is not configured")
	}
	return a.tenants.GetTenantByApiKey(apiKey)
}

func (a *authImpl) validateProjectAccess(r *http.Request, u *user.User) error {
	return nil
}
