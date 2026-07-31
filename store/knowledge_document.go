package store

type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "PENDING"
	DocumentStatusProcessing DocumentStatus = "PROCESSING"
	DocumentStatusReady      DocumentStatus = "READY"
	DocumentStatusFailed     DocumentStatus = "FAILED"
)

type KBDocument struct {
	ID              int32
	KnowledgeBaseID int32
	FileName        string
	FilePath        string
	FileSize        int64
	Status          DocumentStatus
	ErrorMessage    string
	CreatedTs       int64
	UpdatedTs       int64
}

// FindKBDocument 查询条件
type FindKBDocument struct {
	ID              *int32
	KnowledgeBaseID *int32
	Status          *DocumentStatus
	Limit           *int
	Offset          *int
}

// UpdateKBDocument 更新字段
type UpdateKBDocument struct {
	ID           int32
	Status       *DocumentStatus
	ErrorMessage *string
	UpdatedTs    *int64
}
