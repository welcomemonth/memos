package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

// KBDocument model related methods. 全部AI生成代码
func (d *DB) CreateKBDocument(ctx context.Context, create *store.KBDocument) (*store.KBDocument, error) {
	fields := []string{"`knowledge_base_id`", "`file_name`", "`file_path`", "`file_size`", "`status`"}
	placeholder := []string{"?", "?", "?", "?", "?"}
	args := []any{create.KnowledgeBaseID, create.FileName, create.FilePath, create.FileSize, string(create.Status)}

	if create.CreatedTs != 0 {
		fields = append(fields, "`created_ts`")
		placeholder = append(placeholder, "?")
		args = append(args, create.CreatedTs)
	}
	if create.UpdatedTs != 0 {
		fields = append(fields, "`updated_ts`")
		placeholder = append(placeholder, "?")
		args = append(args, create.UpdatedTs)
	}

	stmt := "INSERT INTO `kb_document` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING `id`, `created_ts`, `updated_ts`"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(
		&create.ID,
		&create.CreatedTs,
		&create.UpdatedTs,
	); err != nil {
		return nil, err
	}

	return create, nil
}

func (d *DB) ListKBDocuments(ctx context.Context, find *store.FindKBDocument) ([]*store.KBDocument, error) {
	where, args := []string{"1 = 1"}, []any{}

	if find.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *find.ID)
	}
	if find.KnowledgeBaseID != nil {
		where, args = append(where, "`knowledge_base_id` = ?"), append(args, *find.KnowledgeBaseID)
	}
	if find.Status != nil {
		where, args = append(where, "`status` = ?"), append(args, string(*find.Status))
	}

	query := "SELECT `id`, `knowledge_base_id`, `file_name`, `file_path`, `file_size`, `status`, `error_message`, `created_ts`, `updated_ts` FROM `kb_document` WHERE " + strings.Join(where, " AND ") + " ORDER BY `created_ts` DESC"
	if find.Limit != nil {
		query = fmt.Sprintf("%s LIMIT %d", query, *find.Limit)
		if find.Offset != nil {
			query = fmt.Sprintf("%s OFFSET %d", query, *find.Offset)
		}
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []*store.KBDocument{}
	for rows.Next() {
		doc := &store.KBDocument{}
		var status string
		if err := rows.Scan(&doc.ID, &doc.KnowledgeBaseID, &doc.FileName, &doc.FilePath, &doc.FileSize, &status, &doc.ErrorMessage, &doc.CreatedTs, &doc.UpdatedTs); err != nil {
			return nil, err
		}
		doc.Status = store.DocumentStatus(status)
		list = append(list, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (d *DB) UpdateKBDocument(ctx context.Context, update *store.UpdateKBDocument) error {
	set, args := []string{}, []any{}

	if update.Status != nil {
		set = append(set, "`status` = ?")
		args = append(args, string(*update.Status))
	}
	if update.ErrorMessage != nil {
		set = append(set, "`error_message` = ?")
		args = append(args, *update.ErrorMessage)
	}

	if len(set) == 0 {
		return nil
	}

	set = append(set, "`updated_ts` = ?")
	args = append(args, time.Now().Unix())
	args = append(args, update.ID)

	query := "UPDATE `kb_document` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	_, err := d.db.ExecContext(ctx, query, args...)
	return err
}

func (d *DB) DeleteKBDocument(ctx context.Context, delete *store.DeleteKBDocument) error {
	result, err := d.db.ExecContext(ctx, "DELETE FROM `kb_document` WHERE `id` = ?", delete.ID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}
	return nil
}
