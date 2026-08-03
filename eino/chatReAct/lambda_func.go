package chatReAct

import (
	"context"

	"github.com/usememos/memos/eino/chatChain"
)

// newLambda component initialization function of node 'preparePrompt' in graph 'chatChain'
func newLambda(ctx context.Context, input map[string]any) (output map[string]any, err error) {
	return chatChain.NewPreparePromptLambdaFunc(ctx, input)
}
