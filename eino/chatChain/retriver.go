package chatChain

import (
	"context"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/usememos/memos/eino/common/embedder.go"
	mymilvus "github.com/usememos/memos/store/vector/milvus"
)

func NewRetriever(ctx context.Context, partitionIds []string) (*milvus.Retriever, error) {
	// 1、创建embedder
	embedder, err := embedder.NewEmbedder(ctx)
	if err != nil {
		return nil, err
	}

	// 自定义 SearchParam，COSINE 距离的 radius 应该是很小的值
	sp, _ := entity.NewIndexAUTOINDEXSearchParam(1)
	sp.AddRadius(0.0)      // 搜索半径下限
	sp.AddRangeFilter(2.0) // 范围过滤上限，COSINE 距离范围 [0, 2]

	//2、创建检索节点
	return milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      mymilvus.SharedClient.Client(),
		Collection:  mymilvus.SharedClient.CollectionName(),
		MetricType:  entity.COSINE,
		VectorField: "vector",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		TopK:            3,
		Embedding:       embedder,
		Sp:              sp,
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
