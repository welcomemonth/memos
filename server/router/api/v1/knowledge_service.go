package v1

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
)

/*
3.2 每个方法的通用流程

1. 获取当前用户（鉴权）
2. 验证请求参数
3. 调用 Store 层操作数据库
4. 转换结果格式（store → proto）
5. 返回
*/
// CreateKnowledgeBase 创建知识库
func (s *APIV1Service) CreateKnowledgeBase(ctx context.Context, request *v1pb.CreateKnowledgeBaseRequest) (*v1pb.KnowledgeBase, error) {
	// 1. 获取当前用户
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// 2. 验证请求
	if request.KnowledgeBase == nil {
		return nil, status.Errorf(codes.InvalidArgument, "knowledge_base is required")
	}
	if request.KnowledgeBase.Title == "" {
		return nil, status.Errorf(codes.InvalidArgument, "title is required")
	}

	// 3. 生成 UID
	uid, err := ValidateAndGenerateUID("")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate uid...")
	}
	// 4. 创建知识库
	kb, err := s.Store.CreateKnowledgeBase(ctx, &store.KnowledgeBase{
		UID:         uid,
		CreatorID:   user.ID,
		Title:       request.KnowledgeBase.Title,
		Description: request.KnowledgeBase.Description,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create knowledge base: %v", err)
	}

	// 5. 转换为 proto 格式返回
	return convertKnowledgeBaseToPb(kb), nil
}

// ListKnowledgeBases 获取所有知识库
func (s *APIV1Service) ListKnowledgeBases(ctx context.Context, request *v1pb.ListKnowledgeBasesRequest) (*v1pb.ListKnowledgeBasesResponse, error) {
	// 1. 获取当前用户
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// 2. 构建查询条件
	find := &store.FindKnowledgeBase{
		CreatorID: &user.ID,
	}
	if request.PageSize > 0 {
		limit := int(request.PageSize)
		find.Limit = &limit
	}
	// TODO: 处理 page_token 分页

	// 3. 查询
	list, err := s.Store.ListKnowledgeBases(ctx, find)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list knowledge bases: %v", err)
	}

	// 4. 转换结果
	knowledgeBases := make([]*v1pb.KnowledgeBase, 0, len(list))
	for _, kb := range list {
		knowledgeBases = append(knowledgeBases, convertKnowledgeBaseToPb(kb))
	}

	return &v1pb.ListKnowledgeBasesResponse{
		KnowledgeBases: knowledgeBases,
	}, nil
}

// GetKnowledgeBase 获取单个知识库
func (s *APIV1Service) GetKnowledgeBase(ctx context.Context, request *v1pb.GetKnowledgeBaseRequest) (*v1pb.KnowledgeBase, error) {
	// 1. 获取当前用户
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// 2. 解析 name，提取 uid
	// name 格式: knowledgebases/{uid}
	uid, err := extractUIDFromName(request.Name, "knowledgebases")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	// 3. 查询
	kb, err := s.Store.GetKnowledgeBase(ctx, &store.FindKnowledgeBase{
		UID:       &uid,
		CreatorID: &user.ID,
	})
	if err != nil {
		if err == store.ErrKnowledgeBaseNotFound {
			return nil, status.Errorf(codes.NotFound, "knowledge base not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get knowledge base: %v", err)
	}

	return convertKnowledgeBaseToPb(kb), nil
}

// UpdateKnowledgeBase 更新知识库
func (s *APIV1Service) UpdateKnowledgeBase(ctx context.Context, request *v1pb.UpdateKnowledgeBaseRequest) (*v1pb.KnowledgeBase, error) {
	// 1. 获取当前用户
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// 2. 验证请求
	if request.KnowledgeBase == nil {
		return nil, status.Errorf(codes.InvalidArgument, "knowledge_base is required")
	}

	// 3. 解析 name，提取 uid
	uid, err := extractUIDFromName(request.KnowledgeBase.Name, "knowledgebases")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	// 4. 查询现有知识库
	existing, err := s.Store.GetKnowledgeBase(ctx, &store.FindKnowledgeBase{
		UID:       &uid,
		CreatorID: &user.ID,
	})
	if err != nil {
		if err == store.ErrKnowledgeBaseNotFound {
			return nil, status.Errorf(codes.NotFound, "knowledge base not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get knowledge base: %v", err)
	}

	// 5. 构建更新参数
	update := &store.UpdateKnowledgeBase{
		ID: existing.ID,
	}

	// 6. 根据 update_mask 决定更新哪些字段
	if request.UpdateMask != nil {
		for _, path := range request.UpdateMask.GetPaths() {
			switch path {
			case "title":
				update.Title = &request.KnowledgeBase.Title
			case "description":
				update.Description = &request.KnowledgeBase.Description
			}
		}
	} else {
		// 没有 mask，更新所有字段
		update.Title = &request.KnowledgeBase.Title
		update.Description = &request.KnowledgeBase.Description
	}

	// 7. 执行更新
	if err := s.Store.UpdateKnowledgeBase(ctx, update); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update knowledge base: %v", err)
	}

	// 8. 重新查询返回更新后的结果
	updated, err := s.Store.GetKnowledgeBase(ctx, &store.FindKnowledgeBase{
		UID:       &uid,
		CreatorID: &user.ID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated knowledge base: %v", err)
	}

	return convertKnowledgeBaseToPb(updated), nil
}

// DeleteKnowledgeBase 删除知识库
func (s *APIV1Service) DeleteKnowledgeBase(ctx context.Context, request *v1pb.DeleteKnowledgeBaseRequest) (*emptypb.Empty, error) {
	// 1. 获取当前用户
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// 2. 解析 name，提取 uid
	uid, err := extractUIDFromName(request.Name, "knowledgebases")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	// 3. 查询知识库
	kb, err := s.Store.GetKnowledgeBase(ctx, &store.FindKnowledgeBase{
		UID:       &uid,
		CreatorID: &user.ID,
	})
	if err != nil {
		if errors.Is(err, store.ErrKnowledgeBaseNotFound) {
			return nil, status.Errorf(codes.NotFound, "knowledge base not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get knowledge base: %v", err)
	}

	// 4. 删除
	if err := s.Store.DeleteKnowledgeBase(ctx, kb.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete knowledge base: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ==================== 辅助函数 ====================

// convertKnowledgeBaseToPb 将 store.KnowledgeBase 转换为 v1pb.KnowledgeBase
func convertKnowledgeBaseToPb(kb *store.KnowledgeBase) *v1pb.KnowledgeBase {
	return &v1pb.KnowledgeBase{
		Name:        "knowledgebases/" + kb.UID,
		Title:       kb.Title,
		Description: kb.Description,
		CreateTime:  timestamppb.New(unixToTime(kb.CreatedTs)),
		UpdateTime:  timestamppb.New(unixToTime(kb.UpdatedTs)),
		// TODO: 计算 document_count
		// DocumentCount: int32(len(docs)),
	}
}

// extractUIDFromName 从资源名中提取 UID
// name 格式: {collection}/{uid}
func extractUIDFromName(name, collection string) (string, error) {
	prefix := collection + "/"
	if !strings.HasPrefix(name, prefix) {
		return "", status.Errorf(codes.InvalidArgument, "invalid name format, expected %s/{uid}", collection)
	}
	uid := strings.TrimPrefix(name, prefix)
	if uid == "" {
		return "", status.Errorf(codes.InvalidArgument, "uid is empty")
	}
	return uid, nil
}

// unixToTime 将 Unix 时间戳转换为 time.Time
func unixToTime(ts int64) time.Time {
	// time.Unix 接收两个参数：秒 和 纳秒
	// 如果 ts 是秒级时间戳，纳秒部分传 0 即可
	return time.Unix(ts, 0)
}
