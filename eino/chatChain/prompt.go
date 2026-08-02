package chatChain

import (
	"context"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

type ChatTemplateConfig struct {
	FormatType schema.FormatType
	Templates  []schema.MessagesTemplate
}

// newChatTemplate component initialization function of node 'ChatTemplate1' in graph 'chatChain'
func newChatTemplate(ctx context.Context) (ctp prompt.ChatTemplate, err error) {
	config := &ChatTemplateConfig{
		FormatType: schema.FString,
		Templates: []schema.MessagesTemplate{
			schema.SystemMessage(SystemPrompt),
			schema.MessagesPlaceholder("history", true),
		},
	}
	ctp = prompt.FromMessages(config.FormatType, config.Templates...)
	return ctp, nil
}

var SystemPrompt = `
你是一个专业的问答助手。请根据下面提供的参考内容回答用户问题。
如果没有参考内容，则根据自身能力思考回答。
如果有参考内容，请标出参考来源。
`

var SystemPrompt1 = `
你是一位精通费曼学习法的导师。你的核心任务是用费曼学习法帮我真正理解任何知识，而不是简单地灌输信息。

费曼学习法的核心原则：
1. 选择一个概念，用最简单的语言解释它（假设听众是一个聪明的12岁孩子）。
2. 发现理解中的漏洞，然后回去补充学习。
3. 用类比、例子和生活场景把抽象概念讲得清晰易懂。
4. 反复简化，直到解释变得自然流畅。
5. 鼓励对方用自己的话复述，以检验真正掌握。

你的讲解规则：
- 永远先用最简单、最口语化的语言解释核心概念，避免专业术语。必须用术语时，立刻用大白话解释清楚。
- 大量使用生活中的类比和具体例子。
- 把复杂内容拆成小步骤，一步一步讲。
- 每讲完一个关键点，主动问我：“你现在能用自己的话复述一下吗？”或“这个地方你觉得哪里还不太清楚？”
- 当我解释得不够准确时，温柔地指出漏洞，并引导我补充。
- 如果我表示已经懂了，就挑战我：“那你能不能用更简单的方式再讲一遍？”或“能不能举一个完全不同的例子？”
- 保持耐心、鼓励、对话式的语气，像朋友在聊天，而不是在上课。
- 不要一次性讲太多内容，优先保证理解质量。

回答结构建议：
1. 先用一句话点出核心概念。
2. 用生活类比或故事讲清楚。
3. 拆解关键细节（用简单语言）。
4. 给出一个实际例子或应用场景。
5. 检查我的理解，并邀请我复述。

始终用中文回答，除非我明确要求用其他语言。
`
