package knowledge

import (
	"context"
	"os"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino/components/embedding"
)

func newEmbedding(ctx context.Context) (eb embedding.Embedder, err error) {
	// TODO Modify component configuration here.
	config := &dashscope.EmbeddingConfig{
		APIKey: os.Getenv("EMBED_API_KEY"),
		Model:  os.Getenv("EMBED_MODEL"),
		// Dimensions: , // 默认就是1024 即使用的维度就是默认的
	}
	eb, err = dashscope.NewEmbedder(ctx, config)
	if err != nil {
		return nil, err
	}
	return eb, nil
}
