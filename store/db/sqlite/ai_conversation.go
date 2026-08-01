package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/usememos/memos/store"
)

func (d *DB) CreateAIConversation(ctx context.Context, create *store.AIConversation) (*store.AIConversation, error) {
	fields := []string{"`uid`", "`creator_id`", "`title`", "`model`"}
	placeholder := []string{"?", "?", "?", "?"}
	args := []any{create.UID, create.CreatorID, create.Title, create.Model}

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

	stmt := "INSERT INTO `ai_conversation` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING `id`, `created_ts`, `updated_ts`"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(&create.ID, &create.CreatedTs, &create.UpdatedTs); err != nil {
		return nil, err
	}
	return create, nil
}

func (d *DB) ListAIConversations(ctx context.Context, find *store.FindAIConversation) ([]*store.AIConversation, error) {
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

	query := "SELECT `id`, `uid`, `creator_id`, `title`, `model`, `created_ts`, `updated_ts` FROM `ai_conversation` WHERE " + strings.Join(where, " AND ") + " ORDER BY `updated_ts` DESC"
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

	var list []*store.AIConversation
	for rows.Next() {
		c := &store.AIConversation{}
		if err := rows.Scan(&c.ID, &c.UID, &c.CreatorID, &c.Title, &c.Model, &c.CreatedTs, &c.UpdatedTs); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (d *DB) UpdateAIConversation(ctx context.Context, update *store.UpdateAIConversation) error {
	set, args := []string{}, []any{}
	if update.Title != nil {
		set = append(set, "`title` = ?")
		args = append(args, *update.Title)
	}
	if update.Model != nil {
		set = append(set, "`model` = ?")
		args = append(args, *update.Model)
	}
	if len(set) == 0 {
		return nil
	}
	set = append(set, "`updated_ts` = ?")
	args = append(args, time.Now().Unix())
	args = append(args, update.ID)

	query := "UPDATE `ai_conversation` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	_, err := d.db.ExecContext(ctx, query, args...)
	return err
}

func (d *DB) DeleteAIConversation(ctx context.Context, id int32) error {
	// 级联删除消息
	if _, err := d.db.ExecContext(ctx, "DELETE FROM `ai_message` WHERE `conversation_id` = ?", id); err != nil {
		return err
	}
	_, err := d.db.ExecContext(ctx, "DELETE FROM `ai_conversation` WHERE `id` = ?", id)
	return err
}

// ========== AIMessage ==========

func (d *DB) CreateAIMessage(ctx context.Context, create *store.AIMessage) (*store.AIMessage, error) {
	fields := []string{"`uid`", "`conversation_id`", "`role`", "`status`", "`content`", "`model`", "`parent_message_uid`", "`tokens_used`", "`error_message`", "`payload`"}
	placeholder := []string{"?", "?", "?", "?", "?", "?", "?", "?", "?", "?"}
	args := []any{create.UID, create.ConversationID, create.Role, create.Status, create.Content, create.Model, create.ParentMessageUID, create.TokensUsed, create.ErrorMessage, create.Payload}

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

	stmt := "INSERT INTO `ai_message` (" + strings.Join(fields, ", ") + ") VALUES (" + strings.Join(placeholder, ", ") + ") RETURNING `id`, `created_ts`, `updated_ts`"
	if err := d.db.QueryRowContext(ctx, stmt, args...).Scan(&create.ID, &create.CreatedTs, &create.UpdatedTs); err != nil {
		return nil, err
	}
	return create, nil
}

func (d *DB) ListAIMessages(ctx context.Context, find *store.FindAIMessage) ([]*store.AIMessage, error) {
	where, args := []string{"1 = 1"}, []any{}
	if find.ID != nil {
		where, args = append(where, "`id` = ?"), append(args, *find.ID)
	}
	if find.UID != nil {
		where, args = append(where, "`uid` = ?"), append(args, *find.UID)
	}
	if find.ConversationID != nil {
		where, args = append(where, "`conversation_id` = ?"), append(args, *find.ConversationID)
	}
	if find.Role != nil {
		where, args = append(where, "`role` = ?"), append(args, string(*find.Role))
	}

	query := "SELECT `id`, `uid`, `conversation_id`, `role`, `status`, `content`, `model`, `parent_message_uid`, `tokens_used`, `error_message`, `payload`, `created_ts`, `updated_ts` FROM `ai_message` WHERE " + strings.Join(where, " AND ") + " ORDER BY `created_ts` ASC"
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

	var list []*store.AIMessage
	for rows.Next() {
		m := &store.AIMessage{}
		var role, status string
		if err := rows.Scan(&m.ID, &m.UID, &m.ConversationID, &role, &status, &m.Content, &m.Model, &m.ParentMessageUID, &m.TokensUsed, &m.ErrorMessage, &m.Payload, &m.CreatedTs, &m.UpdatedTs); err != nil {
			return nil, err
		}
		m.Role = store.AIMessageRole(role)
		m.Status = store.AIMessageStatus(status)
		list = append(list, m)
	}
	return list, rows.Err()
}

func (d *DB) UpdateAIMessage(ctx context.Context, update *store.UpdateAIMessage) error {
	set, args := []string{}, []any{}
	if update.Status != nil {
		set = append(set, "`status` = ?")
		args = append(args, string(*update.Status))
	}
	if update.Content != nil {
		set = append(set, "`content` = ?")
		args = append(args, *update.Content)
	}
	if update.Model != nil {
		set = append(set, "`model` = ?")
		args = append(args, *update.Model)
	}
	if update.TokensUsed != nil {
		set = append(set, "`tokens_used` = ?")
		args = append(args, *update.TokensUsed)
	}
	if update.ErrorMessage != nil {
		set = append(set, "`error_message` = ?")
		args = append(args, *update.ErrorMessage)
	}
	if update.Payload != nil {
		set = append(set, "`payload` = ?")
		args = append(args, *update.Payload)
	}
	if len(set) == 0 {
		return nil
	}
	set = append(set, "`updated_ts` = ?")
	args = append(args, time.Now().Unix())
	args = append(args, update.ID)

	query := "UPDATE `ai_message` SET " + strings.Join(set, ", ") + " WHERE `id` = ?"
	_, err := d.db.ExecContext(ctx, query, args...)
	return err
}

func (d *DB) DeleteAIMessage(ctx context.Context, id int32) error {
	_, err := d.db.ExecContext(ctx, "DELETE FROM `ai_message` WHERE `id` = ?", id)
	return err
}
