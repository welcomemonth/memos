package store

import (
	"context"

	"github.com/pkg/errors"
)

var (
	ErrAIConversationNotFound = errors.New("ai conversation not found")
	ErrAIMessageNotFound      = errors.New("ai message not found")
)

// ========== Conversation ==========

type AIConversation struct {
	ID        int32
	UID       string // 用户可见唯一标识
	CreatorID int32
	Title     string
	Model     string
	CreatedTs int64
	UpdatedTs int64
}

type FindAIConversation struct {
	ID        *int32
	UID       *string
	CreatorID *int32
	Limit     *int
	Offset    *int
}

type UpdateAIConversation struct {
	ID        int32
	Title     *string
	Model     *string
	UpdatedTs *int64
}

// ========== Message ==========

type AIMessageRole string

const (
	AIMessageRoleUser      AIMessageRole = "user"
	AIMessageRoleAssistant AIMessageRole = "assistant"
	AIMessageRoleTool      AIMessageRole = "tool"
	AIMessageRoleSystem    AIMessageRole = "system"
)

type AIMessageStatus string

const (
	AIMessageStatusPending   AIMessageStatus = "pending"
	AIMessageStatusCompleted AIMessageStatus = "completed"
	AIMessageStatusError     AIMessageStatus = "error"
	AIMessageStatusCancelled AIMessageStatus = "cancelled"
)

type AIMessage struct {
	ID                int32
	UID               string
	ConversationID    int32
	Role              AIMessageRole
	Status            AIMessageStatus
	Content           string
	Model             string
	ParentMessageUID  string
	TokensUsed        int32
	ErrorMessage      string
	Payload           string // JSON: reasoning_items, tool_calls, attachments, search_results
	CreatedTs         int64
	UpdatedTs         int64
}

type FindAIMessage struct {
	ID             *int32
	UID            *string
	ConversationID *int32
	Role           *AIMessageRole
	Limit          *int
	Offset         *int
}

type UpdateAIMessage struct {
	ID           int32
	Status       *AIMessageStatus
	Content      *string
	Model        *string
	TokensUsed   *int32
	ErrorMessage *string
	Payload      *string
	UpdatedTs    *int64
}

// ========== Store methods ==========

func (s *Store) CreateAIConversation(ctx context.Context, create *AIConversation) (*AIConversation, error) {
	return s.driver.CreateAIConversation(ctx, create)
}

func (s *Store) GetAIConversation(ctx context.Context, find *FindAIConversation) (*AIConversation, error) {
	list, err := s.driver.ListAIConversations(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrAIConversationNotFound
	}
	return list[0], nil
}

func (s *Store) ListAIConversations(ctx context.Context, find *FindAIConversation) ([]*AIConversation, error) {
	return s.driver.ListAIConversations(ctx, find)
}

func (s *Store) UpdateAIConversation(ctx context.Context, update *UpdateAIConversation) error {
	return s.driver.UpdateAIConversation(ctx, update)
}

func (s *Store) DeleteAIConversation(ctx context.Context, id int32) error {
	return s.driver.DeleteAIConversation(ctx, id)
}

func (s *Store) CreateAIMessage(ctx context.Context, create *AIMessage) (*AIMessage, error) {
	return s.driver.CreateAIMessage(ctx, create)
}

func (s *Store) GetAIMessage(ctx context.Context, find *FindAIMessage) (*AIMessage, error) {
	list, err := s.driver.ListAIMessages(ctx, find)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrAIMessageNotFound
	}
	return list[0], nil
}

func (s *Store) ListAIMessages(ctx context.Context, find *FindAIMessage) ([]*AIMessage, error) {
	return s.driver.ListAIMessages(ctx, find)
}

func (s *Store) UpdateAIMessage(ctx context.Context, update *UpdateAIMessage) error {
	return s.driver.UpdateAIMessage(ctx, update)
}

func (s *Store) DeleteAIMessage(ctx context.Context, id int32) error {
	return s.driver.DeleteAIMessage(ctx, id)
}
