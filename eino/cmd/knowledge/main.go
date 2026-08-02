package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/document"
	"github.com/joho/godotenv"
	"github.com/usememos/memos/eino/knowledge"
	mymilvus "github.com/usememos/memos/store/vector/milvus"
)

func init() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("load .env failed: %v", err)
	} else {
		log.Print(os.Getenv("MILVUS_ADDRESS"))
		return
	}
}

func main() {

	ctx := context.Background()
	mymilvus.InitMilvusClient(ctx)
	// 2. 初始化 Milvus 客户端（全局只创建一次）
	_, err := mymilvus.NewMilvusClient(ctx)
	if err != nil {
		log.Fatalf("init milvus failed: %v", err)
	}

	// 3. 构建知识索引图
	runnable, err := knowledge.BuildKnowledgeIndexing(ctx)
	if err != nil {
		log.Fatalf("build graph failed: %v", err)
	}
	// 4. 要索引的文件
	filePath := "documents/K6VV7rqZfZmxBkXDWpDVPD/dQHiY6xyqQVkWsGYarrJQV_告警处理手册.md"
	if _, err := os.Stat(filePath); err != nil {
		log.Fatalf("file not found: %v", err)
	}

	// 5. 跑一次索引
	ids, err := runnable.Invoke(ctx, document.Source{URI: filePath})
	if err != nil {
		log.Fatalf("invoke failed: %v", err)
	}
	fmt.Printf("indexed %d chunks:\n", len(ids))
	for _, id := range ids {
		fmt.Println(" -", id)
	}
}
