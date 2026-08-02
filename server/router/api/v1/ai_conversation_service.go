package v1

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cloudwego/eino/schema"

	"github.com/usememos/memos/eino/chatChain"
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

	// 调用 Eino 框架获取 AI 回复
	assistantContent, err := callEinoChat(ctx, request.Content)
	if err != nil {
		slog.Error("Failed to call Eino chat", "error", err)
		assistantContent = "抱歉，AI 服务暂时不可用：" + err.Error()
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

// StreamMessage streams the assistant reply via Connect server-streaming.
//
// 数据流：
//
//	前端 request.Content → Eino runner.Stream() → 逐 chunk 通过 stream.Send() 推送
//	每个 chunk 是 *schema.Message，Content → ContentDelta，ReasoningContent → ReasoningDelta
//	流结束后：先创建 assistant message 到 DB → 发送 StreamDone → 更新 conversation title
func (s *APIV1Service) StreamMessage(request *v1pb.StreamMessageRequest, stream grpc.ServerStreamingServer[v1pb.StreamMessageEvent]) error {
	ctx := stream.Context()

	// ===== 1. 鉴权 =====
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

	// ===== 2. 查找或自动创建 conversation =====
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

	// ===== 3. 如果是新对话，先通知前端 =====
	if created {
		if err := stream.Send(&v1pb.StreamMessageEvent{
			Event: &v1pb.StreamMessageEvent_ConversationCreated{
				ConversationCreated: &v1pb.ConversationCreated{
					Name:  "ai/conversations/" + conv.UID,
					Title: request.Content,
					Model: conv.Model,
				},
			},
		}); err != nil {
			return err
		}
	}

	// ===== 4. 保存用户消息 =====
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

	// ===== 5. 先创建一个 pending 状态的 assistant message =====
	assistantModel := request.Model
	if assistantModel == "" {
		assistantModel = conv.Model
	}
	assistantMsg, err := s.Store.CreateAIMessage(ctx, &store.AIMessage{
		UID:              mustGenerateUID(),
		ConversationID:   conv.ID,
		Role:             store.AIMessageRoleAssistant,
		Status:           store.AIMessageStatusPending,
		Content:          "",
		Model:            assistantModel,
		ParentMessageUID: userMsg.UID,
		Payload:          "{}",
	})
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create assistant message: %v", err)
	}

	// ===== 6. 构建对话历史（排除当前这条用户消息，避免重复） =====
	history, err := buildHistory(ctx, s.Store, conv.ID, userMsg.UID)
	if err != nil {
		slog.Warn("failed to build history", "err", err)
		history = []*schema.Message{}
	}
	slog.Debug("Current Conversation History", "count", len(history), "history", history)
	// ===== 7. 调用 Eino 流式生成 =====
	runner, err := chatChain.BuildchatChain(ctx)
	if err != nil {
		// 构建失败 → 更新 message 状态为 error，通知前端
		errMsg := "failed to build chat chain: " + err.Error()
		_ = s.Store.UpdateAIMessage(ctx, &store.UpdateAIMessage{
			ID:           assistantMsg.ID,
			Status:       ptrStatus(store.AIMessageStatusError),
			ErrorMessage: &errMsg,
		})
		return status.Errorf(codes.Internal, "failed to build chat chain: %v", err)
	}

	streamReader, err := runner.Stream(ctx, map[string]any{
		"question": request.Content,
		"history":  history,
	})
	if err != nil {
		errMsg := "failed to start stream: " + err.Error()
		_ = s.Store.UpdateAIMessage(ctx, &store.UpdateAIMessage{
			ID:           assistantMsg.ID,
			Status:       ptrStatus(store.AIMessageStatusError),
			ErrorMessage: &errMsg,
		})
		return status.Errorf(codes.Internal, "failed to start stream: %v", err)
	}
	defer streamReader.Close()

	// ===== 7. 逐 chunk 推送给前端 =====
	var fullContent string
	var fullReasoning string
	for {
		chunk, recvErr := streamReader.Recv()

		// 流结束
		if errors.Is(recvErr, io.EOF) {
			break
		}
		// 接收出错 → 通知前端，但保留已生成的内容
		if recvErr != nil {
			slog.Error("stream recv error", "err", recvErr)
			_ = stream.Send(&v1pb.StreamMessageEvent{
				Event: &v1pb.StreamMessageEvent_Error{
					Error: &v1pb.StreamError{
						Message: recvErr.Error(),
						Code:    "STREAM_RECV_ERROR",
					},
				},
			})
			break
		}

		// 类型断言为 schema.Message
		msg, ok := chunk.(*schema.Message)
		if !ok {
			slog.Warn("unexpected chunk type", "type", chunk)
			continue
		}

		// 推送 reasoning（思考过程）
		if msg.ReasoningContent != "" {
			fullReasoning += msg.ReasoningContent
			if sendErr := stream.Send(&v1pb.StreamMessageEvent{
				Event: &v1pb.StreamMessageEvent_Reasoning{
					Reasoning: &v1pb.ReasoningDelta{Delta: msg.ReasoningContent},
				},
			}); sendErr != nil {
				// 客户端断开连接
				return sendErr
			}
		}

		// 推送 content（正文）
		if msg.Content != "" {
			if sendErr := stream.Send(&v1pb.StreamMessageEvent{
				Event: &v1pb.StreamMessageEvent_Content{
					Content: &v1pb.ContentDelta{Delta: msg.Content},
				},
			}); sendErr != nil {
				return sendErr
			}
			fullContent += msg.Content
		}
	}

	// ===== 8. 流结束 → 更新 DB 中的 assistant message =====
	// 如果有 reasoning，构建 Payload JSON
	var payloadStr string
	if fullReasoning != "" {
		// 构建 reasoning_items JSON
		payload := map[string]any{
			"reasoning_items": []map[string]any{
				{
					"content":     fullReasoning,
					"duration_ms": 0,
					"tokens":      0,
				},
			},
		}
		if b, err := json.Marshal(payload); err == nil {
			payloadStr = string(b)
		}
	}

	updateMsg := &store.UpdateAIMessage{
		ID:      assistantMsg.ID,
		Status:  ptrStatus(store.AIMessageStatusCompleted),
		Content: &fullContent,
	}
	if payloadStr != "" {
		updateMsg.Payload = &payloadStr
	}
	if err := s.Store.UpdateAIMessage(ctx, updateMsg); err != nil {
		slog.Error("failed to update assistant message", "err", err)
	}

	// ===== 9. 发送完成事件 =====
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

	// ===== 10. 如果是新对话，更新 title =====
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

// ptrStatus returns a pointer to an AIMessageStatus for use in update structs.
func ptrStatus(s store.AIMessageStatus) *store.AIMessageStatus {
	return &s
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
	msg := &v1pb.AIMessage{
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
	}

	// 解析 Payload JSON，填充 reasoning_items
	if m.Payload != "" && m.Payload != "{}" {
		var payload map[string]any
		if err := json.Unmarshal([]byte(m.Payload), &payload); err == nil {
			if items, ok := payload["reasoning_items"].([]any); ok {
				for _, item := range items {
					if itemMap, ok := item.(map[string]any); ok {
						ri := &v1pb.ReasoningItem{}
						if content, ok := itemMap["content"].(string); ok {
							ri.Content = content
						}
						if durationMs, ok := itemMap["duration_ms"].(float64); ok {
							ri.DurationMs = int32(durationMs)
						}
						if tokens, ok := itemMap["tokens"].(float64); ok {
							ri.Tokens = int32(tokens)
						}
						msg.ReasoningItems = append(msg.ReasoningItems, ri)
					}
				}
			}
		}
	}

	return msg
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

// buildHistory 从 DB 读取会话历史，转换为 eino schema.Message 列表。
func buildHistory(ctx context.Context, db *store.Store, conversationID int32, excludeUID string) ([]*schema.Message, error) {
	msgs, err := db.ListAIMessages(ctx, &store.FindAIMessage{
		ConversationID: &conversationID,
	})
	if err != nil {
		return nil, err
	}

	history := make([]*schema.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.UID == excludeUID {
			continue
		}
		if m.Status != store.AIMessageStatusCompleted {
			continue // 没有完成的对话也不加入
		}
		var msg *schema.Message
		switch m.Role {
		case store.AIMessageRoleUser:
			msg = schema.UserMessage(m.Content)
		case store.AIMessageRoleAssistant:
			msg = schema.AssistantMessage(m.Content, nil)
		case store.AIMessageRoleSystem:
			msg = schema.SystemMessage(m.Content)
		default:
			continue
		}
		history = append(history, msg)
	}
	return history, nil
}

// callEinoChat calls the Eino framework to get AI response
func callEinoChat(ctx context.Context, question string) (string, error) {
	// Build the chat chain
	runner, err := chatChain.BuildchatChain(ctx)
	if err != nil {
		return "", err
	}

	// Invoke the chain with the question
	result, err := runner.Invoke(ctx, question)
	if err != nil {
		return "", err
	}

	// Convert result to string
	if msg, ok := result.(*schema.Message); ok && msg != nil {
		return msg.Content, nil
	}

	// If result is already a string
	if str, ok := result.(string); ok {
		return str, nil
	}

	return "无法解析 AI 响应", nil
}
