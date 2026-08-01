package store

import "time"

// ========== 会话 ==========

type AIConversation struct {
	ID        string    `json:"id"`
	UID       string    `json:"uid"`       // 所属用户
	Title     string    `json:"title"`
	Model     string    `json:"model"`     // 使用的模型标识
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ========== 消息 ==========

// MessageRole 消息角色
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"
	MessageRoleSystem    MessageRole = "system"
)

// MessageStatus 消息状态（用于流式场景）
type MessageStatus string

const (
	MessageStatusPending    MessageStatus = "pending"    // 生成中
	MessageStatusCompleted  MessageStatus = "completed"  // 已完成
	MessageStatusError      MessageStatus = "error"      // 出错
	MessageStatusCancelled  MessageStatus = "cancelled"  // 已取消
)

// AIMessage 对话消息
type AIMessage struct {
	ID             string        `json:"id"`
	ConversationID string        `json:"conversationId"`
	Role           MessageRole   `json:"role"`
	Status         MessageStatus `json:"status"`

	// 正文内容（Markdown）
	Content string `json:"content"`

	// AI 思考内容（thinking/reasoning），可能多段
	ReasoningItems []ReasoningItem `json:"reasoningItems,omitempty"`

	// 工具调用列表（assistant 消息发出，tool 消息返回结果）
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`

	// 附件（图片、文件等）
	Attachments []MessageAttachment `json:"attachments,omitempty"`

	// 搜索引用（联网搜索场景）
	SearchResults []SearchResult `json:"searchResults,omitempty"`

	// 元数据
	Model       string `json:"model,omitempty"`       // 实际使用的模型
	ParentID    string `json:"parentId,omitempty"`    // 父消息ID（用于消息树/分支）
	TokensUsed  int    `json:"tokensUsed,omitempty"`  // token 消耗
	ErrorMsg    string `json:"errorMsg,omitempty"`    // 错误信息

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ========== 思考过程 ==========

// ReasoningItem AI 的思考/推理过程片段
type ReasoningItem struct {
	ID        string `json:"id"`
	Content   string `json:"content"`    // 思考内容
	Duration  int    `json:"duration"`   // 思考耗时（毫秒）
	Tokens    int    `json:"tokens"`     // 思考 token 数
	CreatedAt time.Time `json:"createdAt"`
}

// ========== 工具调用 ==========

// ToolCall 工具调用记录
type ToolCall struct {
	ID         string         `json:"id"`
	MessageID  string         `json:"messageId"`  // 所属消息ID
	Name       string         `json:"name"`       // 工具名称，如 "web_search"
	Args       string         `json:"args"`       // 调用参数（JSON 字符串）
	Result     string         `json:"result"`     // 返回结果
	Status     ToolCallStatus `json:"status"`     // 执行状态
	Duration   int            `json:"duration"`   // 执行耗时（毫秒）
	ErrorMsg   string         `json:"errorMsg,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// ToolCallStatus 工具调用状态
type ToolCallStatus string

const (
	ToolCallPending   ToolCallStatus = "pending"   // 等待执行
	ToolCallRunning   ToolCallStatus = "running"   // 执行中
	ToolCallCompleted ToolCallStatus = "completed" // 执行完成
	ToolCallFailed    ToolCallStatus = "failed"    // 执行失败
)

// ========== 附件 ==========

// AttachmentType 附件类型
type AttachmentType string

const (
	AttachmentImage AttachmentType = "image"
	AttachmentFile  AttachmentType = "file"
	AttachmentVideo AttachmentType = "video"
	AttachmentAudio AttachmentType = "audio"
)

// MessageAttachment 消息附件
type MessageAttachment struct {
	ID        string         `json:"id"`
	MessageID string         `json:"messageId"`
	Type      AttachmentType `json:"type"`
	Name      string         `json:"name"`      // 文件名
	URL       string         `json:"url"`       // 访问地址
	MimeType  string         `json:"mimeType"`  // MIME 类型
	Size      int64          `json:"size"`      // 文件大小（字节）
	CreatedAt time.Time      `json:"createdAt"`
}

// ========== 搜索引用 ==========

// SearchResult 联网搜索结果引用
type SearchResult struct {
	ID        string `json:"id"`
	MessageID string `json:"messageId"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Snippet   string `json:"snippet"`  // 摘要
	Source    string `json:"source"`   // 来源站点
	UpdatedAt string `json:"updatedAt,omitempty"` // 页面更新时间
}
