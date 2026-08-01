package v1

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

// ========== 会话 CRUD ==========

func (s *APIV1Service) ListConversations(ctx context.Context, request *v1pb.ListConversationsRequest) (*v1pb.ListConversationsResponse, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	find := &store.FindAIConversation{
		CreatorID: &user.ID,
	}
	if request.PageSize > 0 {
		limit := int(request.PageSize)
		find.Limit = &limit
	}
	// TODO: handle page_token pagination

	list, err := s.Store.ListAIConversations(ctx, find)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list conversations: %v", err)
	}

	conversations := make([]*v1pb.AIConversation, 0, len(list))
	for _, c := range list {
		conversations = append(conversations, convertConversationToPb(c))
	}

	return &v1pb.ListConversationsResponse{
		Conversations: conversations,
	}, nil
}

func (s *APIV1Service) GetConversation(ctx context.Context, request *v1pb.GetConversationRequest) (*v1pb.AIConversation, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	uid, err := extractUIDFromName(request.Name, "ai/conversations")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	c, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		UID:       &uid,
		CreatorID: &user.ID,
	})
	if err != nil {
		if err == store.ErrAIConversationNotFound {
			return nil, status.Errorf(codes.NotFound, "conversation not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}

	return convertConversationToPb(c), nil
}

func (s *APIV1Service) UpdateConversation(ctx context.Context, request *v1pb.UpdateConversationRequest) (*v1pb.AIConversation, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}
	if request.Conversation == nil {
		return nil, status.Errorf(codes.InvalidArgument, "conversation is required")
	}

	uid, err := extractUIDFromName(request.Conversation.Name, "ai/conversations")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	existing, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		UID:       &uid,
		CreatorID: &user.ID,
	})
	if err != nil {
		if err == store.ErrAIConversationNotFound {
			return nil, status.Errorf(codes.NotFound, "conversation not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}

	update := &store.UpdateAIConversation{ID: existing.ID}
	if request.UpdateMask != nil {
		for _, path := range request.UpdateMask.GetPaths() {
			switch path {
			case "title":
				update.Title = &request.Conversation.Title
			case "model":
				update.Model = &request.Conversation.Model
			}
		}
	} else {
		update.Title = &request.Conversation.Title
		update.Model = &request.Conversation.Model
	}

	if err := s.Store.UpdateAIConversation(ctx, update); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update conversation: %v", err)
	}

	updated, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		UID:       &uid,
		CreatorID: &user.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated conversation: %v", err)
	}

	return convertConversationToPb(updated), nil
}

func (s *APIV1Service) DeleteConversation(ctx context.Context, request *v1pb.DeleteConversationRequest) (*emptypb.Empty, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	uid, err := extractUIDFromName(request.Name, "ai/conversations")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	c, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		UID:       &uid,
		CreatorID: &user.ID,
	})
	if err != nil {
		if err == store.ErrAIConversationNotFound {
			return nil, status.Errorf(codes.NotFound, "conversation not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}

	if err := s.Store.DeleteAIConversation(ctx, c.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete conversation: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ========== 消息 ==========

func (s *APIV1Service) ListMessages(ctx context.Context, request *v1pb.ListMessagesRequest) (*v1pb.ListMessagesResponse, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	convUID, err := extractUIDFromName(request.Parent, "ai/conversations")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent: %v", err)
	}

	conv, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		UID:       &convUID,
		CreatorID: &user.ID,
	})
	if err != nil {
		if err == store.ErrAIConversationNotFound {
			return nil, status.Errorf(codes.NotFound, "conversation not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}

	find := &store.FindAIMessage{
		ConversationID: &conv.ID,
	}
	if request.PageSize > 0 {
		limit := int(request.PageSize)
		find.Limit = &limit
	}

	list, err := s.Store.ListAIMessages(ctx, find)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list messages: %v", err)
	}

	messages := make([]*v1pb.AIMessage, 0, len(list))
	for _, m := range list {
		messages = append(messages, convertMessageToPb(m, conv.UID))
	}

	return &v1pb.ListMessagesResponse{
		Messages: messages,
	}, nil
}

func (s *APIV1Service) GetMessage(ctx context.Context, request *v1pb.GetMessageRequest) (*v1pb.AIMessage, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	msgUID, err := extractUIDFromName(request.Name, "ai/messages")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	msg, err := s.Store.GetAIMessage(ctx, &store.FindAIMessage{UID: &msgUID})
	if err != nil {
		if err == store.ErrAIMessageNotFound {
			return nil, status.Errorf(codes.NotFound, "message not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get message: %v", err)
	}

	conv, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{ID: &msg.ConversationID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if conv.CreatorID != user.ID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	return convertMessageToPb(msg, conv.UID), nil
}

func (s *APIV1Service) DeleteMessage(ctx context.Context, request *v1pb.DeleteMessageRequest) (*emptypb.Empty, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	msgUID, err := extractUIDFromName(request.Name, "ai/messages")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	msg, err := s.Store.GetAIMessage(ctx, &store.FindAIMessage{UID: &msgUID})
	if err != nil {
		if err == store.ErrAIMessageNotFound {
			return nil, status.Errorf(codes.NotFound, "message not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get message: %v", err)
	}

	conv, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{ID: &msg.ConversationID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
	}
	if conv.CreatorID != user.ID {
		return nil, status.Errorf(codes.PermissionDenied, "permission denied")
	}

	if err := s.Store.DeleteAIMessage(ctx, msg.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete message: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ========== 对话（发送消息）==========

func (s *APIV1Service) SendMessage(ctx context.Context, request *v1pb.SendMessageRequest) (*v1pb.SendMessageResponse, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}
	if request.Content == "" {
		return nil, status.Errorf(codes.InvalidArgument, "content is required")
	}

	convUID, err := extractUIDFromName(request.Parent, "ai/conversations")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent: %v", err)
	}

	// 查找或自动创建 conversation
	conv, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		UID:       &convUID,
		CreatorID: &user.ID,
	})
	created := false
	if err != nil {
		if err != store.ErrAIConversationNotFound {
			return nil, status.Errorf(codes.Internal, "failed to get conversation: %v", err)
		}
		conv, err = s.Store.CreateAIConversation(ctx, &store.AIConversation{
			UID:       convUID,
			CreatorID: user.ID,
			Title:     "",
			Model:     request.Model,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create conversation: %v", err)
		}
		created = true
	}

	// 创建 user message
	userMsg, err := s.Store.CreateAIMessage(ctx, &store.AIMessage{
		UID:              mustGenerateUID(),
		ConversationID:   conv.ID,
		Role:             store.AIMessageRoleUser,
		Status:           store.AIMessageStatusCompleted,
		Content:          request.Content,
		Model:            request.Model,
		ParentMessageUID: request.ParentMessage,
		Payload:          "{}",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user message: %v", err)
	}

	// TODO: 调用 Eino 框架获取 AI 回复
	assistantContent := "AI 回复功能尚未集成，请等待 Eino 框架接入。"
	assistantModel := request.Model
	if assistantModel == "" {
		assistantModel = conv.Model
	}

	assistantMsg, err := s.Store.CreateAIMessage(ctx, &store.AIMessage{
		UID:              mustGenerateUID(),
		ConversationID:   conv.ID,
		Role:             store.AIMessageRoleAssistant,
		Status:           store.AIMessageStatusCompleted,
		Content:          assistantContent,
		Model:            assistantModel,
		ParentMessageUID: userMsg.UID,
		Payload:          "{}",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create assistant message: %v", err)
	}

	// 如果是新对话，根据第一条消息设置 title
	if created && request.Content != "" {
		title := request.Content
		if len([]rune(title)) > 30 {
			title = string([]rune(title)[:30]) + "..."
		}
		_ = s.Store.UpdateAIConversation(ctx, &store.UpdateAIConversation{
			ID:    conv.ID,
			Title: &title,
		})
		conv.Title = title
	}

	return &v1pb.SendMessageResponse{
		Message:      convertMessageToPb(assistantMsg, conv.UID),
		Conversation: convertConversationToPb(conv),
	}, nil
}

// StreamMessage streams the assistant reply.
// TODO: 集成 Eino 框架后实现完整的流式 AI 对话
func (s *APIV1Service) StreamMessage(request *v1pb.StreamMessageRequest, stream grpc.ServerStreamingServer[v1pb.StreamMessageEvent]) error {
	ctx := stream.Context()

	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return status.Errorf(codes.Unauthenticated, "user not authenticated")
	}
	if request.Content == "" {
		return status.Errorf(codes.InvalidArgument, "content is required")
	}

	convUID, err := extractUIDFromName(request.Parent, "ai/conversations")
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid parent: %v", err)
	}

	// 查找或自动创建 conversation
	conv, err := s.Store.GetAIConversation(ctx, &store.FindAIConversation{
		UID:       &convUID,
		CreatorID: &user.ID,
	})
	created := false
	if err != nil {
		if err != store.ErrAIConversationNotFound {
			return status.Errorf(codes.Internal, "failed to get conversation: %v", err)
		}
		conv, err = s.Store.CreateAIConversation(ctx, &store.AIConversation{
			UID:       convUID,
			CreatorID: user.ID,
			Title:     "",
			Model:     request.Model,
		})
		if err != nil {
			return status.Errorf(codes.Internal, "failed to create conversation: %v", err)
		}
		created = true
	}

	// 如果是新对话，发送 ConversationCreated 事件
	if created {
		if err := stream.Send(&v1pb.StreamMessageEvent{
			Event: &v1pb.StreamMessageEvent_ConversationCreated{
				ConversationCreated: &v1pb.ConversationCreated{
					Name:  "ai/conversations/" + conv.UID,
					Title: conv.Title,
					Model: conv.Model,
				},
			},
		}); err != nil {
			return err
		}
	}

	// 创建 user message
	userMsg, err := s.Store.CreateAIMessage(ctx, &store.AIMessage{
		UID:              mustGenerateUID(),
		ConversationID:   conv.ID,
		Role:             store.AIMessageRoleUser,
		Status:           store.AIMessageStatusCompleted,
		Content:          request.Content,
		Model:            request.Model,
		ParentMessageUID: request.ParentMessage,
		Payload:          "{}",
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create user message: %v", err)
	}

	// TODO: 调用 Eino 框架流式接口，逐帧发送 StreamMessageEvent
	content := "流式 AI 回复功能尚未集成，请等待 Eino 框架接入。"
	if err := stream.Send(&v1pb.StreamMessageEvent{
		Event: &v1pb.StreamMessageEvent_Content{
			Content: &v1pb.ContentDelta{Delta: content},
		},
	}); err != nil {
		return err
	}

	assistantModel := request.Model
	if assistantModel == "" {
		assistantModel = conv.Model
	}

	assistantMsg, err := s.Store.CreateAIMessage(ctx, &store.AIMessage{
		UID:              mustGenerateUID(),
		ConversationID:   conv.ID,
		Role:             store.AIMessageRoleAssistant,
		Status:           store.AIMessageStatusCompleted,
		Content:          content,
		Model:            assistantModel,
		ParentMessageUID: userMsg.UID,
		Payload:          "{}",
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create assistant message: %v", err)
	}

	// 发送完成事件
	if err := stream.Send(&v1pb.StreamMessageEvent{
		Event: &v1pb.StreamMessageEvent_Done{
			Done: &v1pb.StreamDone{
				MessageName: "ai/messages/" + assistantMsg.UID,
				Model:       assistantModel,
			},
		},
	}); err != nil {
		return err
	}

	// 如果是新对话，更新 title
	if created && request.Content != "" {
		title := request.Content
		if len([]rune(title)) > 30 {
			title = string([]rune(title)[:30]) + "..."
		}
		_ = s.Store.UpdateAIConversation(ctx, &store.UpdateAIConversation{
			ID:    conv.ID,
			Title: &title,
		})
	}

	return nil
}

// ========== 转换函数 ==========

func convertConversationToPb(c *store.AIConversation) *v1pb.AIConversation {
	return &v1pb.AIConversation{
		Name:       "ai/conversations/" + c.UID,
		Title:      c.Title,
		Model:      c.Model,
		CreateTime: timestamppb.New(time.Unix(c.CreatedTs, 0)),
		UpdateTime: timestamppb.New(time.Unix(c.UpdatedTs, 0)),
	}
}

func convertMessageToPb(m *store.AIMessage, convUID string) *v1pb.AIMessage {
	return &v1pb.AIMessage{
		Name:          "ai/messages/" + m.UID,
		Parent:        "ai/conversations/" + convUID,
		Role:          convertMessageRole(m.Role),
		Status:        convertMessageStatus(m.Status),
		Content:       m.Content,
		Model:         m.Model,
		ParentMessage: m.ParentMessageUID,
		TokensUsed:    m.TokensUsed,
		ErrorMessage:  m.ErrorMessage,
		CreateTime:    timestamppb.New(time.Unix(m.CreatedTs, 0)),
		UpdateTime:    timestamppb.New(time.Unix(m.UpdatedTs, 0)),
		// TODO: parse payload JSON to populate reasoning_items, tool_calls, attachments, search_results
	}
}

func convertMessageRole(role store.AIMessageRole) v1pb.AIMessage_Role {
	switch role {
	case store.AIMessageRoleUser:
		return v1pb.AIMessage_USER
	case store.AIMessageRoleAssistant:
		return v1pb.AIMessage_ASSISTANT
	case store.AIMessageRoleTool:
		return v1pb.AIMessage_TOOL
	case store.AIMessageRoleSystem:
		return v1pb.AIMessage_SYSTEM
	default:
		return v1pb.AIMessage_ROLE_UNSPECIFIED
	}
}

func convertMessageStatus(s store.AIMessageStatus) v1pb.AIMessage_Status {
	switch s {
	case store.AIMessageStatusPending:
		return v1pb.AIMessage_PENDING
	case store.AIMessageStatusCompleted:
		return v1pb.AIMessage_COMPLETED
	case store.AIMessageStatusError:
		return v1pb.AIMessage_ERROR
	case store.AIMessageStatusCancelled:
		return v1pb.AIMessage_CANCELLED
	default:
		return v1pb.AIMessage_STATUS_UNSPECIFIED
	}
}

func mustGenerateUID() string {
	uid, err := ValidateAndGenerateUID("")
	if err != nil {
		panic("failed to generate UID: " + err.Error())
	}
	return uid
}
