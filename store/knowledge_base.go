package store

type KnowledgeBase struct {
	ID          int32
	UID         string // 用户可见唯一标识，可以分享给其他用户共享
	CreatorID   int32
	Title       string
	Description string
	CreatedTs   int64
	UpdatedTs   int64
}

// 针对知识库的操作
type FindKnowledgeBase struct {
	ID        *int32
	UID       *string
	CreatorID *int32
	Limit     *int
	Offset    *int
}

// UpdateKnowledgeBase 更新字段
type UpdateKnowledgeBase struct {
	ID          int32
	Title       *string
	Description *string
	UpdatedTs   *int64
}
