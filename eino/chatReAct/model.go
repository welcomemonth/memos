package chatReAct

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/usememos/memos/eino/chatChain"
)

func newChatModel(ctx context.Context) (cm model.ChatModel, err error) {
	return chatChain.NewChatModel(ctx)
}
