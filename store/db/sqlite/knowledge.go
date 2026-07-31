package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateKnowledgeBase(ctx context.Context, create *store.KnowledgeBase) (*store.KnowledgeBase, error) {
	// panic("implement me")
	fields := []string{"`uid`", "`creator_id`", "`title`", "`description`"}
	placeholder := []string{"?", "?", "?", "?"}
	args := []any{create.UID, create.CreatorID, create.Title, create.Description}

	// 可选时间戳
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
	//  stmt := "INSERT INTO `knowledge_base` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING `id`, `created_ts`, `updated_ts`"
	stmt := "INSERT INTO `knowledge_base` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING `id`, `created_ts`, `updated_ts`"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(&create.ID, &create.CreatedTs, &create.UpdatedTs); err != nil {
		return nil, err
	}
	return create, nil
}

func (d *DB) ListKnowledgeBases(ctx context.Context, find *store.FindKnowledgeBase) ([]*store.KnowledgeBase, error) {
	where, args := []string{"1 = 1"}, []any{}
	if find.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, "`uid` = ?"), append(args, *find.UID)
	}
	if find.CreatorID != nil {
		where, args = append(where, "`creator_id` = ?"), append(args, *find.CreatorID)
	}

	query := "SELECT `id`, `uid`, `creator_id`, `title`, `description`, `created_ts`, `updated_ts` FROM `knowledge_base` WHERE " + strings.Join(where, " AND ") + " ORDER BY `updated_ts` DESC"
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
	list := []*store.KnowledgeBase{}
	for rows.Next() {
		kb := &store.KnowledgeBase{}
		if err := rows.Scan(&kb.ID, &kb.UID, &kb.CreatorID, &kb.Title, &kb.Description, &kb.CreatedTs, &kb.UpdatedTs); err != nil {
			return nil, err
		}
		list = append(list, kb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
func (d *DB) UpdateKnowledgeBase(ctx context.Context, update *store.UpdateKnowledgeBase) error {
	set, args := []string{}, []any{}
	if update.Title != nil {
		set = append(set, "`title` = ?")
		args = append(args, *update.Title)
	}
	if update.Description != nil {
		set = append(set, "`description` = ?")
		args = append(args, *update.Description)
	}
	if len(set) == 0 {
		return nil
	}
	// 自动更新 updated_ts
	set = append(set, "`updated_ts` = ?")
	args = append(args, time.Now().Unix())
	args = append(args, update.ID)

	query := "UPDATE `knowledge_base` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	_, err := d.db.ExecContext(ctx, query, args...)
	return err
}
func (d *DB) DeleteKnowledgeBase(ctx context.Context, delete *store.DeleteKnowledgeBase) error {
	// 先删除关联的文档 TODO 后续可以更新选择是否删除关联文档
	if _, err := d.db.ExecContext(ctx, "DELETE FROM `kb_document` WHERE `knowledge_base_id` = ?", delete.ID); err != nil {
		return err
	}
	// 再删除知识库
	result, err := d.db.ExecContext(ctx, "DELETE FROM `knowledge_base` WHERE `id` = ?", delete.ID)
	if err != nil {
		return err
	}
	if _, err := result.RowsAffected(); err != nil {
		return err
	}
	return nil
}
