package chatReAct

import (
	"context"

	"github.com/cloudwego/eino/compose"
)

func BuildchatChain(ctx context.Context) (r compose.Runnable[any, any], err error) {
	const (
		ChatTemplate1 = "ChatTemplate1"
		preparePrompt = "preparePrompt"
		Lambda5       = "Lambda5"
	)
	g := compose.NewGraph[any, any]()
	chatTemplate1KeyOfChatTemplate, err := newChatTemplate(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddChatTemplateNode(ChatTemplate1, chatTemplate1KeyOfChatTemplate, compose.WithNodeName("template"))
	_ = g.AddLambdaNode(preparePrompt, compose.InvokableLambda(newLambda))
	lambda5KeyOfLambda, err := newLambda1(ctx)
	if err != nil {
		return nil, err
	}
	_ = g.AddLambdaNode(Lambda5, lambda5KeyOfLambda)
	_ = g.AddEdge(compose.START, preparePrompt)
	_ = g.AddEdge(Lambda5, compose.END)
	_ = g.AddEdge(preparePrompt, ChatTemplate1)
	_ = g.AddEdge(ChatTemplate1, Lambda5)
	r, err = g.Compile(ctx, compose.WithGraphName("chatChain"))
	if err != nil {
		return nil, err
	}
	return r, err
}
