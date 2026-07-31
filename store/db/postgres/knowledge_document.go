package postgres

import (
	"context"

	"github.com/usememos/memos/store"
)

// KBDocument model related methods.
func (d *DB) CreateKBDocument(ctx context.Context, create *store.KBDocument) (*store.KBDocument, error) {
	panic("")
}
func (d *DB) ListKBDocuments(ctx context.Context, find *store.FindKBDocument) ([]*store.KBDocument, error) {
	panic("")
}
func (d *DB) UpdateKBDocument(ctx context.Context, update *store.UpdateKBDocument) error {
	panic("")
}
func (d *DB) DeleteKBDocument(ctx context.Context, delete *store.DeleteKBDocument) error {
	panic("")
}
