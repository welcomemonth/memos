package knowledge

import (
	"context"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	mymilvus "github.com/usememos/memos/store/vector/milvus"
)

// newIndexer component initialization function of node 'Indexer' in graph 'KnowledgeIndexing'
func newIndexer(ctx context.Context) (idr indexer.Indexer, err error) {

	// TODO Modify component configuration here.
	config := &milvus.IndexerConfig{
		Client:            mymilvus.SharedClient.Client(), // 拿到内部的 cli.Client
		Collection:        mymilvus.SharedClient.CollectionName(),
		Fields:            mymilvus.Fields,
		DocumentConverter: floatDocumentConverter,
	}
	embeddingIns11, err := newEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	config.Embedding = embeddingIns11
	idr, err = milvus.NewIndexer(ctx, config)
	if err != nil {
		return nil, err
	}
	return idr, nil
}

// 基本必须这个函数，如果不写，则默认的Converter函数会导致他将数据转换为Bytes类型，从而失败
func floatDocumentConverter(ctx context.Context, docs []*schema.Document, vectors [][]float64) ([]interface{}, error) {
	rows := make([]interface{}, 0, len(docs))
	for i, doc := range docs {
		// ✅ 每行用 map，键是字段名
		// 将 []float64 转换为 []float32
		vec32 := make([]float32, len(vectors[i]))
		for j, v := range vectors[i] {
			vec32[j] = float32(v)
		}
		row := map[string]interface{}{
			"id":       doc.ID,
			"content":  doc.Content,
			"metadata": doc.MetaData,
			"vector":   vec32,
		}
		rows = append(rows, row)
	}
	return rows, nil
}
