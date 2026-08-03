package chatReAct

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// fullStreamToolCallChecker reads the entire stream to determine if tool calls exist.
//
// The default firstChunkStreamToolCallChecker stops at the first non-empty content
// chunk and assumes no tool calls. This breaks for models (like glm-5-2) that output
// a preamble like "我来帮你搜索…" before the tool_calls. The checker sees the content
// and prematurely routes to END, never executing tools.
//
// This checker reads all chunks to check if ANY of them contain tool calls.
// It receives a forked stream from the framework, so the original stream still
// flows to the output in parallel.
func fullStreamToolCallChecker(_ context.Context, sr *schema.StreamReader[*schema.Message]) (bool, error) {
	defer sr.Close()
	hasToolCalls := false
	for {
		msg, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return false, err
		}
		if len(msg.ToolCalls) > 0 {
			hasToolCalls = true
		}
	}
	slog.Info("[react] fullStreamToolCallChecker result", "hasToolCalls", hasToolCalls)
	return hasToolCalls, nil
}

// newLambda1 component initialization function of node 'Lambda5' in graph 'chatChain'
func newLambda1(ctx context.Context) (lba *compose.Lambda, err error) {
	config := &react.AgentConfig{
		MaxStep:               10,
		StreamToolCallChecker: fullStreamToolCallChecker,
	}
	chatModelIns11, err := newChatModel(ctx)
	if err != nil {
		return nil, err
	}
	config.Model = chatModelIns11
	CalculateTool, err := CreateCalculateTool()
	if err != nil {
		return nil, err
	}
	SerpapiTool, err := newSerpapiTool(ctx)
	if err != nil {
		return nil, err
	}
	config.ToolsConfig.Tools = []tool.BaseTool{SerpapiTool, CalculateTool}

	// Log tool registration
	for i, t := range config.ToolsConfig.Tools {
		info, _ := t.Info(ctx)
		slog.Info("[react] registered tool", "index", i, "name", info.Name, "desc", info.Desc)
	}

	ins, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, err
	}
	slog.Info("[react] agent created", "maxStep", config.MaxStep, "toolsCount", len(config.ToolsConfig.Tools))

	lba, err = compose.AnyLambda(ins.Generate, ins.Stream, nil, nil)
	if err != nil {
		return nil, err
	}
	return lba, nil
}
