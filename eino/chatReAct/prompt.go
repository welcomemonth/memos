package chatReAct

import (
	"context"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/usememos/memos/eino/chatChain"
)

type ChatTemplateConfig struct {
	FormatType schema.FormatType
	Templates  []schema.MessagesTemplate
}

// newChatTemplate component initialization function of node 'ChatTemplate1' in graph 'chatChain'
func newChatTemplate(ctx context.Context) (ctp prompt.ChatTemplate, err error) {
	return chatChain.NewChatTemplate(ctx)
}
