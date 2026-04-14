package projects

func (c *projectsImpl) GetProjectByKeyAndTenant(projectKey string, _ int) (*Project, error) {
	return c.GetProjectByKey(projectKey)
}

func (c *projectsImpl) ExistsByName(name string, _ int) (bool, error) {
	return c.existsByName(name)
}

func (c *projectsImpl) ListProjectsByTenantID(_ int) ([]*Project, error) {
	return c.listProjects()
}

func (c *projectsImpl) CreateProject(_ int, name string, platform string) (*Project, error) {
	return c.createProject(name, platform)
}
