package handler

import "time"

type user struct {
	Id        uint32
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type userResponse struct {
	CursorID string
	Done     bool
	Users    []user
}

type newSearchRequest struct {
	IDMatch  string `query:"id_match"`
	PageSize int    `query:"page_size"`
}

type nextPageRequest struct {
	Cursor   string `param:"cursor"`
	PageSize int    `query:"page_size"`
}
