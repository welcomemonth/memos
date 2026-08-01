package mysql

import (
	"context"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateAIConversation(ctx context.Context, create *store.AIConversation) (*store.AIConversation, error) {
	panic("implement me")
}

func (d *DB) ListAIConversations(ctx context.Context, find *store.FindAIConversation) ([]*store.AIConversation, error) {
	panic("implement me")
}

func (d *DB) UpdateAIConversation(ctx context.Context, update *store.UpdateAIConversation) error {
	panic("implement me")
}

func (d *DB) DeleteAIConversation(ctx context.Context, id int32) error {
	panic("implement me")
}

func (d *DB) CreateAIMessage(ctx context.Context, create *store.AIMessage) (*store.AIMessage, error) {
	panic("implement me")
}

func (d *DB) ListAIMessages(ctx context.Context, find *store.FindAIMessage) ([]*store.AIMessage, error) {
	panic("implement me")
}

func (d *DB) UpdateAIMessage(ctx context.Context, update *store.UpdateAIMessage) error {
	panic("implement me")
}

func (d *DB) DeleteAIMessage(ctx context.Context, id int32) error {
	panic("implement me")
}
