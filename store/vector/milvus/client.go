package milvus

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	cli "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

var Fields = []*entity.Field{
	{
		Name:     "id",
		DataType: entity.FieldTypeVarChar,
		TypeParams: map[string]string{
			"max_length": "256",
		},
		PrimaryKey: true,
	},
	{
		Name:     "vector", // 确保字段名匹配
		DataType: entity.FieldTypeFloatVector,
		TypeParams: map[string]string{
			"dim": "1024",
		},
	},
	{
		Name:     "content",
		DataType: entity.FieldTypeVarChar,
		TypeParams: map[string]string{
			"max_length": "8192",
		},
	},
	{
		Name:     "file_uuid",
		DataType: entity.FieldTypeVarChar,
		TypeParams: map[string]string{
			"max_length": "64",
		},
	},
	{
		Name:     "metadata",
		DataType: entity.FieldTypeJSON,
	},
}

var SharedClient MilvusClient

func InitMilvusClient(ctx context.Context) {
	client, err := NewMilvusClient(ctx)
	if err != nil {
		panic(err)
	}
	SharedClient = *client
}

type MilvusClient struct {
	client         cli.Client
	collectionName string
}

func NewMilvusClient(ctx context.Context) (*MilvusClient, error) {
	DBName := os.Getenv("MILVUS_DBNAME")
	Address := os.Getenv("MILVUS_ADDRESS")
	CollectionName := os.Getenv("MILVUS_COLLECTION")

	if DBName == "" || Address == "" {
		return nil, fmt.Errorf("DBName or Address is empty")
	}
	// 1、先尝试链接default数据
	defaultClient, err := cli.NewClient(ctx, cli.Config{
		Address: Address,
		DBName:  "default",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to default database: %w", err)
	}
	defer defaultClient.Close()
	// 2、检查env中填写的数据库名称是否存在，不存在则创建
	databases, err := defaultClient.ListDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	slog.Info("milvus databases= ", databases)
	knowledgeDBExists := false
	for _, db := range databases {
		if db.Name == DBName {
			knowledgeDBExists = true
			break
		}
	}
	if !knowledgeDBExists {
		err = defaultClient.CreateDatabase(ctx, DBName)
		if err != nil {
			return nil, fmt.Errorf("failed to create %s database: %w", DBName, err)
		}
	}
	// 3、创建链接到knowledge数据库的客户端
	knowledgeClient, err := cli.NewClient(ctx, cli.Config{
		Address: Address,
		DBName:  DBName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s database: %w", DBName, err)
	}
	// 4、创建collection，使用一个collection + 多个partition
	collections, err := knowledgeClient.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}
	knowledgeCollectionExists := false
	for _, collection := range collections {
		if collection.Name == CollectionName {
			knowledgeCollectionExists = true
			break
		}
	}

	if !knowledgeCollectionExists {
		// 创建biz collection的schema
		schema := &entity.Schema{
			CollectionName: CollectionName,
			Description:    "Business knowledge collection",
			Fields:         Fields,
		}

		err = knowledgeClient.CreateCollection(ctx, schema, entity.DefaultShardNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to create biz collection: %w", err)
		}

		// 为id字段创建autoindex索引
		idIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
		if err != nil {
			return nil, fmt.Errorf("failed to create id index: %w", err)
		}
		err = knowledgeClient.CreateIndex(ctx, CollectionName, "id", idIndex, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create id index: %w", err)
		}

		// 为content字段创建autoindex索引
		contentIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
		if err != nil {
			return nil, fmt.Errorf("failed to create content index: %w", err)
		}
		err = knowledgeClient.CreateIndex(ctx, CollectionName, "content", contentIndex, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create content index: %w", err)
		}

		// 为vector字段创建autoindex索引
		vectorIndex, err := entity.NewIndexAUTOINDEX(entity.COSINE)
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
		err = knowledgeClient.CreateIndex(ctx, CollectionName, "vector", vectorIndex, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}

		// 为file_uuid字段创建索引
		fileUUIDIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
		if err != nil {
			return nil, fmt.Errorf("failed to create file_uuid index: %w", err)
		}
		err = knowledgeClient.CreateIndex(ctx, CollectionName, "file_uuid", fileUUIDIndex, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create file_uuid index: %w", err)
		}
	}

	return &MilvusClient{
		client:         knowledgeClient,
		collectionName: CollectionName,
	}, nil
}

func (m *MilvusClient) Client() cli.Client {
	return m.client
}

func (m *MilvusClient) CollectionName() string {
	return m.collectionName
}

// FIXME 这个返回的该是什么？
func (m *MilvusClient) CreateCollection(ctx context.Context, collectionName string) (error, error) {
	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    "Business knowledge collection",
		Fields:         Fields,
	}
	err := m.client.CreateCollection(ctx, schema, entity.DefaultShardNumber)
	if err != nil {
		return nil, fmt.Errorf("filed to create collection: %w", err)
	}
	// 为id字段创建autoindex索引
	idIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
	if err != nil {
		return nil, fmt.Errorf("failed to create id index: %w", err)
	}
	err = m.client.CreateIndex(ctx, collectionName, "id", idIndex, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create id index: %w", err)
	}

	// 为content字段创建autoindex索引
	contentIndex, err := entity.NewIndexAUTOINDEX(entity.L2)
	if err != nil {
		return nil, fmt.Errorf("failed to create content index: %w", err)
	}
	err = m.client.CreateIndex(ctx, collectionName, "content", contentIndex, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create content index: %w", err)
	}

	// 为vector字段创建autoindex索引
	vectorIndex, err := entity.NewIndexAUTOINDEX(entity.COSINE)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector index: %w", err)
	}
	err = m.client.CreateIndex(ctx, collectionName, "vector", vectorIndex, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector index: %w", err)
	}
	return nil, nil
}
