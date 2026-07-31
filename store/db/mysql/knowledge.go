package mysql

import (
	"context"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateKnowledgeBase(ctx context.Context, create *store.KnowledgeBase) (*store.KnowledgeBase, error) {
	panic("implement me")
}

func (d *DB) ListKnowledgeBases(ctx context.Context, find *store.FindKnowledgeBase) ([]*store.KnowledgeBase, error) {
	panic("")
}
func (d *DB) UpdateKnowledgeBase(ctx context.Context, update *store.UpdateKnowledgeBase) error {
	panic("err")
}
func (d *DB) DeleteKnowledgeBase(ctx context.Context, delete *store.DeleteKnowledgeBase) error {
	panic("err")
}
