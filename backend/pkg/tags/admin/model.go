package admin

type CreateTagRequest struct {
	Name            string  `json:"name" validate:"required,min=1,max=100"`
	Selector        string  `json:"selector" validate:"required,min=1,max=255"`
	IgnoreClickRage bool    `json:"ignoreClickRage"`
	IgnoreDeadClick bool    `json:"ignoreDeadClick"`
	Location        *string `json:"location,omitempty" validate:"omitempty,max=500"`
}

type UpdateTagRequest struct {
	Name     *string `json:"name,omitempty" validate:"omitempty,min=1,max=100"`
	Location *string `json:"location,omitempty" validate:"omitempty,max=500"`
}

type TagResponse struct {
	TagID           int     `json:"tagId"`
	Name            string  `json:"name"`
	Selector        string  `json:"selector"`
	IgnoreClickRage bool    `json:"ignoreClickRage"`
	IgnoreDeadClick bool    `json:"ignoreDeadClick"`
	Location        *string `json:"location"`
	Volume          int64   `json:"volume"`
	Users           int64   `json:"users"`
}

type ListTagsResponse struct {
	Tags  []TagResponse `json:"tags"`
	Total int           `json:"total"`
}

type TagStats struct {
	Volume      int64
	UniqueUsers int64
}
