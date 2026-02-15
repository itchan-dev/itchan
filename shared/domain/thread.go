package domain

import (
	"time"
)

// to iterate thru layers: handler -> service -> storage
type ThreadCreationData struct {
	Title     ThreadTitle
	Board     BoardShortName
	IsPinned  bool
	OpMessage MessageCreationData
}

type ThreadMetadata struct {
	Id             ThreadId
	Title          ThreadTitle
	Board          BoardShortName
	MessageCount   int
	LastBumped     time.Time
	LastModifiedAt time.Time
	IsPinned       bool
}

type ThreadPagination struct {
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
	TotalCount  int `json:"total_count"` // Total message count
}

type Thread struct {
	ThreadMetadata
	Messages   []*Message        `json:"messages"`
	Pagination *ThreadPagination `json:"pagination,omitempty"`
}
