package chatChain

import (
	"context"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
	"github.com/spf13/viper"
)

// newChatModel component initialization function of node 'ChatModel1' in graph 'chatChain'
func newChatModel(ctx context.Context) (cm model.ChatModel, err error) {
	timeout := time.Second * 10
	// TODO Modify component configuration here.
	config := &ark.ChatModelConfig{
		APIKey:  viper.GetString("LLM_API_KEY"),
		Model:   "glm-5-2-260617",
		Timeout: &timeout,
	}
	cm, err = ark.NewChatModel(ctx, config)
	if err != nil {
		return nil, err
	}
	return cm, nil
}
