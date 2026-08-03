package chatChain

import (
	"context"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
)

// newChatModel component initialization function of node 'ChatModel1' in graph 'chatChain'
func NewChatModel(ctx context.Context) (cm model.ChatModel, err error) {
	// timeout := time.Second * 10
	// TODO Modify component configuration here.
	config := &ark.ChatModelConfig{
		APIKey: os.Getenv("ARK_LLM_API_KEY"),
		Model:  "glm-5-2-260617",
		// Timeout: &timeout,
	}
	cm, err = ark.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	return cm, nil
}
