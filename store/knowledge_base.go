package store

import (
	"context"

	"github.com/pkg/errors"
)

var (
	ErrKnowledgeNotFound     = errors.New("knowledge not found")
	ErrKnowledgeBaseNotFound = errors.New("knowledge base not found")
)

type KnowledgeBase struct {
	ID          int32
	UID         string // 用户可见唯一标识，可以分享给其他用户共享
	CreatorID   int32
	Title       string
	Description string
	CreatedTs   int64
	UpdatedTs   int64
}

// 针对知识库的操作
type FindKnowledgeBase struct {
	ID        *int32
	UID       *string
	CreatorID *int32
	Limit     *int
	Offset    *int
}

// UpdateKnowledgeBase 更新字段
type UpdateKnowledgeBase struct {
	ID          int32
	Title       *string
	Description *string
	UpdatedTs   *int64
}

type DeleteKnowledgeBase struct {
	ID int32
}

func (s *Store) CreateKnowledgeBase(ctx context.Context, create *KnowledgeBase) (*KnowledgeBase, error) {
	// 插入数据库
	knowledge, err := s.driver.CreateKnowledgeBase(ctx, create)
	if err != nil {
		return nil, err
	}
	return knowledge, nil
}

func (s *Store) GetKnowledgeBase(ctx context.Context, find *FindKnowledgeBase) (*KnowledgeBase, error) {
	// 查询单条 是通过list然后获取其中一个
	list, err := s.driver.ListKnowledgeBases(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrKnowledgeBaseNotFound
	}
	return list[0], nil
}

func (s *Store) ListKnowledgeBases(ctx context.Context, find *FindKnowledgeBase) ([]*KnowledgeBase, error) {
	// 查询列表
	return s.driver.ListKnowledgeBases(ctx, find)
}

func (s *Store) UpdateKnowledgeBase(ctx context.Context, update *UpdateKnowledgeBase) error {
	// 更新
	return s.driver.UpdateKnowledgeBase(ctx, update)
}

func (s *Store) DeleteKnowledgeBase(ctx context.Context, id int32) error {
	// 删除
	return s.driver.DeleteKnowledgeBase(ctx, &DeleteKnowledgeBase{
		ID: id,
	})
}
