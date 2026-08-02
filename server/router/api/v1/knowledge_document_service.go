package v1

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/usememos/memos/eino/knowledge"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/store"
	mymilvus "github.com/usememos/memos/store/vector/milvus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

/*
	TODO 仔细阅读当前代码内容

1、鉴权（获取当前用户）
2、验证参数（parent 格式、文件内容、文件名）
3、验证知识库是否存在，且属于当前用户
4、保存文件到指定目录
5、创建文档记录（）
6、返回文档信息
7、todo :后台进行向量化
*/
func (s *APIV1Service) UploadDocument(ctx context.Context, request *v1pb.UploadDocumentRequest) (*v1pb.Document, error) {
	// 1. 鉴权
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}
	// 2. 验证参数
	if request.Parent == "" {
		return nil, status.Errorf(codes.InvalidArgument, "parent is required")
	}
	if len(request.Content) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "content is required")
	}
	if request.Filename == "" {
		return nil, status.Errorf(codes.InvalidArgument, "filename is required")
	}
	// 防止路径遍历攻击
	filename := filepath.Base(request.Filename)

	// 3. 验证知识库存在且属于当前用户
	kbUID, err := extractUIDFromName(request.Parent, "knowledgebases")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent: %v", err)
	}
	kb, err := s.Store.GetKnowledgeBase(ctx, &store.FindKnowledgeBase{
		UID:       &kbUID,
		CreatorID: &user.ID,
	})
	if err != nil {
		if errors.Is(err, store.ErrKnowledgeBaseNotFound) {
			return nil, status.Errorf(codes.NotFound, "knowledge base not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get knowledge base: %v", err)
	}
	// 4. 生成文档 UID
	docUID, err := ValidateAndGenerateUID("")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate uid: %v", err)
	}

	// 5. 保存文件到磁盘
	// 路径: documents/{kb_uid}/{doc_uid}_{filename}
	relDir := filepath.Join("documents", kbUID)
	absDir := filepath.Join(s.Profile.Data, relDir)
	if err := os.MkdirAll(absDir, os.ModePerm); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create directory: %v", err)
	}

	fileName := fmt.Sprintf("%s_%s", docUID, filename)
	absPath := filepath.Join(absDir, fileName)
	if err := os.WriteFile(absPath, request.Content, 0644); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write file: %v", err)
	}
	// 存相对路径
	relPath := filepath.ToSlash(filepath.Join(relDir, fileName))

	// 6. 创建文档记录
	doc, err := s.Store.CreateKBDocument(ctx, &store.KBDocument{
		KnowledgeBaseID: kb.ID,
		FileName:        filename,
		FilePath:        relPath,
		FileSize:        int64(len(request.Content)),
		Status:          store.DocumentStatusPending,
	})
	if err != nil {
		// 写入失败，删除文件
		os.Remove(absPath)
		return nil, status.Errorf(codes.Internal, "failed to create document: %v", err)
	}
	slog.Debug("upload file absPath:", absPath)
	// TODO 应该是异步开始处理文档，这里先同步执行
	r, err := knowledge.BuildKnowledgeIndexing(ctx)
	if err != nil {
		// return nil, status.Errorf(codes.Internal, "failed to handle document: %v", err)
		slog.Error("failed to handle document: %v", err)
	}

	// 创建 partition（如果不存在）
	err = mymilvus.SharedClient.CreatePartition(ctx, kbUID)
	if err != nil {
		slog.Error("failed to create Partition: %v", err)
	} else {
		ids, err := r.Invoke(ctx, document.Source{URI: absPath}, compose.WithIndexerOption(milvus.WithPartition(kbUID)))
		if err != nil {
			slog.Error("failed to handle document: %v", err)
		}
		slog.Info("document ids: ", ids)
	}

	return convertDocumentToPb(kb.UID, doc), nil
}

// ListDocuments 列出知识库中的文档
func (s *APIV1Service) ListDocuments(ctx context.Context, request *v1pb.ListDocumentsRequest) (*v1pb.ListDocumentsResponse, error) {
	// 1. 鉴权
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// 2. 验证知识库
	if request.Parent == "" {
		return nil, status.Errorf(codes.InvalidArgument, "parent is required")
	}
	kbUID, err := extractUIDFromName(request.Parent, "knowledgebases")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid parent: %v", err)
	}
	kb, err := s.Store.GetKnowledgeBase(ctx, &store.FindKnowledgeBase{
		UID:       &kbUID,
		CreatorID: &user.ID,
	})
	if err != nil {
		if errors.Is(err, store.ErrKnowledgeBaseNotFound) {
			return nil, status.Errorf(codes.NotFound, "knowledge base not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get knowledge base: %v", err)
	}

	// 3. 构建查询条件
	find := &store.FindKBDocument{
		KnowledgeBaseID: &kb.ID,
	}
	if request.PageSize > 0 {
		limit := int(request.PageSize)
		find.Limit = &limit
	}
	// TODO: 处理 page_token

	// 4. 查询
	list, err := s.Store.ListKBDocuments(ctx, find)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list documents: %v", err)
	}

	// 5. 转换
	documents := make([]*v1pb.Document, 0, len(list))
	for _, doc := range list {
		documents = append(documents, convertDocumentToPb(kbUID, doc))
	}

	return &v1pb.ListDocumentsResponse{
		Documents: documents,
	}, nil
}

// DeleteDocument 删除文档
func (s *APIV1Service) DeleteDocument(ctx context.Context, request *v1pb.DeleteDocumentRequest) (*emptypb.Empty, error) {
	// 1. 鉴权
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	// 2. 解析 name
	// 格式: knowledgebases/{kb_uid}/documents/{doc_uid}
	if request.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}
	kbUID, _, err := parseDocumentName(request.Name)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid name: %v", err)
	}

	// 3. 验证知识库
	kb, err := s.Store.GetKnowledgeBase(ctx, &store.FindKnowledgeBase{
		UID:       &kbUID,
		CreatorID: &user.ID,
	})
	if err != nil {
		if errors.Is(err, store.ErrKnowledgeBaseNotFound) {
			return nil, status.Errorf(codes.NotFound, "knowledge base not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get knowledge base: %v", err)
	}

	// 4. 查询文档
	doc, err := s.Store.GetKBDocument(ctx, &store.FindKBDocument{
		KnowledgeBaseID: &kb.ID,
		// TODO: 需要给 KBDocument 加 UID 字段，这里用 ID 查询
	})
	if err != nil {
		if errors.Is(err, store.ErrDocumentNotFound) {
			return nil, status.Errorf(codes.NotFound, "document not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get document: %v", err)
	}

	// 5. 删除磁盘文件
	absPath := filepath.Join(s.Profile.Data, doc.FilePath)
	os.Remove(absPath)

	// 6. 删除数据库记录
	if err := s.Store.DeleteKBDocument(ctx, doc.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete document: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ==================== 辅助函数 ====================

// parseDocumentName 从资源名中提取知识库 UID 和文档 UID
// 格式: knowledgebases/{kb_uid}/documents/{doc_uid}
func parseDocumentName(name string) (kbUID, docUID string, err error) {
	parts := strings.Split(name, "/")
	if len(parts) != 4 {
		return "", "", fmt.Errorf("invalid format, expected knowledgebases/{kb_uid}/documents/{doc_uid}")
	}
	if parts[0] != "knowledgebases" || parts[2] != "documents" {
		return "", "", fmt.Errorf("invalid format")
	}
	if parts[1] == "" || parts[3] == "" {
		return "", "", fmt.Errorf("uid is empty")
	}
	return parts[1], parts[3], nil
}

// convertDocumentToPb 转换文档格式
func convertDocumentToPb(kbUID string, doc *store.KBDocument) *v1pb.Document {
	return &v1pb.Document{
		Name:         fmt.Sprintf("knowledgebases/%s/documents/%d", kbUID, doc.ID),
		Filename:     doc.FileName,
		FileSize:     doc.FileSize,
		Status:       convertDocumentStatus(doc.Status),
		ErrorMessage: doc.ErrorMessage,
		CreateTime:   timestamppb.New(unixToTime(doc.CreatedTs)),
	}
}

// convertDocumentStatus 转换文档状态
func convertDocumentStatus(status store.DocumentStatus) v1pb.Document_Status {
	switch status {
	case store.DocumentStatusPending:
		return v1pb.Document_PENDING
	case store.DocumentStatusProcessing:
		return v1pb.Document_PROCESSING
	case store.DocumentStatusReady:
		return v1pb.Document_READY
	case store.DocumentStatusFailed:
		return v1pb.Document_FAILED
	default:
		return v1pb.Document_STATUS_UNSPECIFIED
	}
}
