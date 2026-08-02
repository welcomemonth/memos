package chatChain

import (
	"context"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/usememos/memos/eino/common/embedder.go"
	mymilvus "github.com/usememos/memos/store/vector/milvus"
)

func NewRetriever(ctx context.Context) (*milvus.Retriever, error) {
	// 1、创建embedder
	embedder, err := embedder.NewEmbedder(ctx)
	if err != nil {
		return nil, err
	}
	//2、创建检索节点
	return milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      mymilvus.SharedClient.Client(),
		Collection:  mymilvus.SharedClient.CollectionName(),
		VectorField: "vector",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		TopK:            3,
		Embedding:       embedder,
		VectorConverter: FloatConvertToDocument,
	})
}

func FloatConvertToDocument(ctx context.Context, vectors [][]float64) ([]entity.Vector, error) {
	result := make([]entity.Vector, 0, len(vectors))
	for _, vec := range vectors {
		float32Vec := make([]float32, len(vec))
		for i, v := range vec {
			float32Vec[i] = float32(v)
		}
		result = append(result, entity.FloatVector(float32Vec))
	}
	return result, nil
}
