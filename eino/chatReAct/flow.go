package chatReAct

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
)

// newLambda1 component initialization function of node 'Lambda5' in graph 'chatChain'
func newLambda1(ctx context.Context) (lba *compose.Lambda, err error) {
	// TODO Modify component configuration here.
	config := &react.AgentConfig{
		MaxStep: 5,
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
	ins, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, err
	}
	lba, err = compose.AnyLambda(ins.Generate, ins.Stream, nil, nil)
	if err != nil {
		return nil, err
	}
	return lba, nil
}
