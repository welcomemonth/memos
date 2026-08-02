package chatChain

import (
	"context"

	"github.com/cloudwego/eino/compose"
)

func BuildchatChain(ctx context.Context) (r compose.Runnable[any, any], err error) {
	const (
		ChatModel1    = "ChatModel1"
		ChatTemplate1 = "ChatTemplate1"
		PreparePrompt = "PreparePrompt"
	)
	g := compose.NewGraph[any, any]()
	_ = g.AddLambdaNode(PreparePrompt, compose.InvokableLambda(NewPreparePromptLambdaFunc), compose.WithNodeName("PreparePromptToChat"))
	chatModel1KeyOfChatModel, err := newChatModel(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddChatModelNode(ChatModel1, chatModel1KeyOfChatModel, compose.WithNodeName("ChatModel"),
		compose.WithInputKey(""))
	chatTemplate1KeyOfChatTemplate, err := newChatTemplate(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddChatTemplateNode(ChatTemplate1, chatTemplate1KeyOfChatTemplate, compose.WithNodeName("template"))
	_ = g.AddEdge(compose.START, PreparePrompt)
	_ = g.AddEdge(PreparePrompt, ChatTemplate1)
	_ = g.AddEdge(ChatModel1, compose.END)
	_ = g.AddEdge(ChatTemplate1, ChatModel1)
	r, err = g.Compile(ctx, compose.WithGraphName("chatChain"))
	if err != nil {
		return nil, err
	}
	return r, err
}
