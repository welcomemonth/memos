package store

import (
	"context"

	"github.com/pkg/errors"
)

type DocumentStatus string

var (
	ErrDocumentNotFound   = errors.New("document not found")
	ErrDocumentProcessing = errors.New("document is still processing")
)

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

type DeleteKBDocument struct {
	ID int32
}

func (s *Store) CreateKBDocument(ctx context.Context, create *KBDocument) (*KBDocument, error) {
	return s.driver.CreateKBDocument(ctx, create)
}

func (s *Store) GetKBDocument(ctx context.Context, find *FindKBDocument) (*KBDocument, error) {
	list, err := s.driver.ListKBDocuments(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrDocumentNotFound
	}
	return list[0], nil
}

func (s *Store) ListKBDocuments(ctx context.Context, find *FindKBDocument) ([]*KBDocument, error) {
	return s.driver.ListKBDocuments(ctx, find)
}

func (s *Store) UpdateKBDocument(ctx context.Context, update *UpdateKBDocument) error {
	return s.driver.UpdateKBDocument(ctx, update)
}

func (s *Store) DeleteKBDocument(ctx context.Context, id int32) error {
	return s.driver.DeleteKBDocument(ctx, &DeleteKBDocument{ID: id})
}
