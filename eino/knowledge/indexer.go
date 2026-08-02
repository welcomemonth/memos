package knowledge

import (
	"context"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/cloudwego/eino/components/indexer"
	mymilvus "github.com/usememos/memos/store/vector/milvus"
)

// newIndexer component initialization function of node 'Indexer' in graph 'KnowledgeIndexing'
func newIndexer(ctx context.Context) (idr indexer.Indexer, err error) {

	// TODO Modify component configuration here.
	config := &milvus.IndexerConfig{
		Client:     mymilvus.SharedClient.Client(), // 拿到内部的 cli.Client
		Collection: mymilvus.SharedClient.CollectionName(),
		Fields:     mymilvus.Fields,
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
