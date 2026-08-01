package chatChain

import (
	"context"
)

// newLambda component initialization function of node 'userInput' in graph 'chatChain'
func newLambda(ctx context.Context, input string) (output map[string]any, err error) {
	return map[string]any{
		"question": input,
	}, nil
}
