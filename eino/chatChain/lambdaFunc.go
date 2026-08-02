package chatChain

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func NewPreparePromptLambdaFunc(ctx context.Context, input map[string]any) (map[string]any, error) {
	// 复制 input 到 promptMap
	promptMap := make(map[string]any)
	for k, v := range input {
		promptMap[k] = v
	}

	// 默认 reference 为 "空"
	promptMap["reference"] = "空"

	// 拿到用户问题；本轮用户问题在 conversation service 中已经从 history 里剔除，
	// 必须在本函数中重新组装为 UserMessage 追加进 history，否则模型看不到本轮输入。
	question, _ := input["question"].(string)

	// 有知识库 partitionId 时才走 RAG 检索；检索失败不阻断主流程，reference 留空。
	partitionId, _ := input["partitionId"].(string)
	reference := ""
	if partitionId != "" && question != "" {
		retriever, err := NewRetriever(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "create retriever failed", "error", err)
		} else {
			results, err := retriever.Retrieve(ctx, question)
			if err != nil {
				slog.WarnContext(ctx, "retrieve failed", "error", err, "question", question)
			} else {
				reference = joinDocuments(results)
			}
		}
	}

	// 组装最终 UserMessage，追加到 history 末尾。
	// - 走 RAG 且有检索结果：按 UserWithRAGTemplate 渲染（带 reference 上下文）。
	// - 未走 RAG / 检索为空 / 无 question：直接以裸 question 作为 UserMessage。
	history, _ := input["history"].([]*schema.Message)
	history = append([]*schema.Message(nil), history...) // 浅拷贝，避免修改入参
	history = append(history, assembleUserMessage(question, reference))
	promptMap["history"] = history
	slog.Debug("history: ", history)
	return promptMap, nil
}

// assembleUserMessage 根据是否有 reference 决定使用 RAG 模板还是裸问题。
func assembleUserMessage(question, reference string) *schema.Message {
	if reference != "" {
		return schema.UserMessage(renderRAGUserMessage(question, reference))
	}
	return schema.UserMessage(question)
}

// renderRAGUserMessage 用 UserWithRAGTemplate 把 reference 与 question 合并为最终发给模型的 user 文本。
func renderRAGUserMessage(question, reference string) string {
	if reference == "" {
		reference = ""
	}
	r := strings.NewReplacer("{reference}", reference, "{question}", question)
	return r.Replace(UserWithRAGTemplate)
}

// joinDocuments 将检索到的文档拼接成字符串
func joinDocuments(docs []*schema.Document) string {
	if len(docs) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, doc := range docs {
		sb.WriteString(fmt.Sprintf("[Source %d]: %s\n", i+1, doc.Content))
	}
	return sb.String()
}

var UserWithRAGTemplate = `
参考内容：
<documents>
{reference}
</documents>


用户问题：
<question>
{question}
</question>
`
