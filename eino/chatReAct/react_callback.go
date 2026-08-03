package chatReAct

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	ucb "github.com/cloudwego/eino/utils/callbacks"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

// ToolCallRecord tracks a single tool call for persistence and display.
type ToolCallRecord struct {
	ID         string
	Name       string
	Arguments  string
	Result     string
	Status     v1pb.ToolCall_ToolCallStatus
	DurationMs int32
	StartTime  time.Time
}

// ReactCallbacks wraps the Eino callback handler and collects tool call records.
type ReactCallbacks struct {
	Handler   callbacks.Handler
	SendEvent func(*v1pb.StreamMessageEvent) error

	mu      sync.Mutex
	records []*ToolCallRecord
}

// NewReactCallbacks creates a callback handler that emits ToolCall stream events
// and forwards intermediate LLM reasoning to the client.
//
// sendEvent MUST be safe for concurrent use (the caller should wrap stream.Send with a mutex).
func NewReactCallbacks(sendEvent func(*v1pb.StreamMessageEvent) error) *ReactCallbacks {
	rc := &ReactCallbacks{
		SendEvent: sendEvent,
		records:   make([]*ToolCallRecord, 0),
	}

	rc.Handler = ucb.NewHandlerHelper().
		ChatModel(&ucb.ModelCallbackHandler{
			OnStart: rc.onChatModelStart,
			// OnEnd 不会对 Stream 调用触发，中间推理转发到 ToolsNode.OnStart 中处理
			OnEndWithStreamOutput: rc.onChatModelStreamEnd,
		}).
		ToolsNode(&ucb.ToolsNodeCallbackHandlers{
			OnStart: rc.onToolsNodeStart,
			OnEnd:   rc.onToolsNodeEnd,
			OnError: rc.onToolsNodeError,
		}).
		Handler()

	return rc
}

// Records returns a copy of the collected tool call records.
func (rc *ReactCallbacks) Records() []*ToolCallRecord {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	out := make([]*ToolCallRecord, len(rc.records))
	copy(out, rc.records)
	return out
}

// onChatModelStart fires before each LLM call. Logs the tools available to the model.
func (rc *ReactCallbacks) onChatModelStart(_ context.Context, _ *callbacks.RunInfo, input *model.CallbackInput) context.Context {
	slog.Info("[react_callback] ChatModel.OnStart",
		"messagesCount", len(input.Messages),
		"toolsCount", len(input.Tools),
	)
	for i, t := range input.Tools {
		slog.Info("[react_callback]   available tool", "index", i, "name", t.Name)
	}
	return context.Background()
}

// onChatModelStreamEnd fires after each streaming LLM call completes.
// We consume the stream copy to accumulate chunks and log the final result.
// Note: For the final step, the stream is also consumed by the normal streamReader.
func (rc *ReactCallbacks) onChatModelStreamEnd(_ context.Context, _ *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
	if output == nil {
		return context.Background()
	}
	// Accumulate streaming chunks to get the final message
	var lastContent string
	var lastReasoning string
	var lastToolCalls []schema.ToolCall
	var lastFinishReason string
	for {
		chunk, err := output.Recv()
		if err != nil {
			break
		}
		if chunk != nil && chunk.Message != nil {
			if chunk.Message.Content != "" {
				lastContent = chunk.Message.Content
			}
			if chunk.Message.ReasoningContent != "" {
				lastReasoning = chunk.Message.ReasoningContent
			}
			if len(chunk.Message.ToolCalls) > 0 {
				lastToolCalls = chunk.Message.ToolCalls
			}
			lastFinishReason = chunk.Message.ResponseMeta.FinishReason
		}
	}
	slog.Info("[react_callback] ChatModel.OnEndWithStreamOutput (accumulated)",
		"contentLen", len(lastContent),
		"contentPreview", preview(lastContent, 200),
		"reasoningLen", len(lastReasoning),
		"toolCallsCount", len(lastToolCalls),
		"finishReason", lastFinishReason,
	)
	return context.Background()
}

// onChatModelEnd fires after each LLM call. For intermediate steps (where the LLM
// decided to call tools), we forward the reasoning content so the client can display
// the "thinking → tool call" flow. The final step's content/reasoning is already
// streamed via the normal streamReader.Recv() loop, so we skip it here.
func (rc *ReactCallbacks) onChatModelEnd(_ context.Context, _ *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
	msg := output.Message
	if msg == nil {
		slog.Info("[react_callback] ChatModel.OnEnd: msg is nil")
		return context.Background()
	}

	slog.Info("[react_callback] ChatModel.OnEnd",
		"contentLen", len(msg.Content),
		"contentPreview", preview(msg.Content, 200),
		"reasoningLen", len(msg.ReasoningContent),
		"toolCallsCount", len(msg.ToolCalls),
		"finishReason", msg.ResponseMeta.FinishReason,
	)
	for i, tc := range msg.ToolCalls {
		slog.Info("[react_callback]   toolCall",
			"index", i,
			"id", tc.ID,
			"name", tc.Function.Name,
			"args", preview(tc.Function.Arguments, 200),
		)
	}

	// Intermediate step: LLM wants to call tools → forward reasoning
	if len(msg.ToolCalls) > 0 {
		if msg.ReasoningContent != "" {
			if err := rc.SendEvent(&v1pb.StreamMessageEvent{
				Event: &v1pb.StreamMessageEvent_Reasoning{
					Reasoning: &v1pb.ReasoningDelta{Delta: msg.ReasoningContent},
				},
			}); err != nil {
				slog.Warn("failed to send intermediate reasoning", "err", err)
			}
		}
		// Some models put reasoning in Content for intermediate steps
		if msg.Content != "" {
			if err := rc.SendEvent(&v1pb.StreamMessageEvent{
				Event: &v1pb.StreamMessageEvent_Content{
					Content: &v1pb.ContentDelta{Delta: msg.Content},
				},
			}); err != nil {
				slog.Warn("failed to send intermediate content", "err", err)
			}
		}
	}

	return context.Background()
}

// onToolsNodeStart fires when the tools node begins execution. The input message
// contains all tool calls the LLM decided to make. We emit ToolCallStart + ToolCallDelta
// for each, and track them for later ToolCallEnd correlation.
func (rc *ReactCallbacks) onToolsNodeStart(_ context.Context, _ *callbacks.RunInfo, input *schema.Message) context.Context {
	if input == nil {
		slog.Info("[react_callback] ToolsNode.OnStart: input is nil")
		return context.Background()
	}

	slog.Info("[react_callback] ToolsNode.OnStart",
		"toolCallsCount", len(input.ToolCalls),
		"reasoningLen", len(input.ReasoningContent),
		"contentLen", len(input.Content),
	)

	// Forward the model's intermediate reasoning BEFORE the tool call events.
	// This gives the frontend the "thinking → tool call" order.
	// Note: we do NOT forward intermediate Content as ContentDelta, because that
	// would pollute streamingContent (the final answer). If the model puts useful
	// info in Content for intermediate steps, the frontend can see it in the
	// ToolCall arguments/results.
	if input.ReasoningContent != "" {
		if err := rc.SendEvent(&v1pb.StreamMessageEvent{
			Event: &v1pb.StreamMessageEvent_Reasoning{
				Reasoning: &v1pb.ReasoningDelta{Delta: input.ReasoningContent},
			},
		}); err != nil {
			slog.Warn("failed to send intermediate reasoning", "err", err)
		}
	}

	now := time.Now()
	for _, tc := range input.ToolCalls {
		// Emit ToolCallStart
		if err := rc.SendEvent(&v1pb.StreamMessageEvent{
			Event: &v1pb.StreamMessageEvent_ToolCallStart{
				ToolCallStart: &v1pb.ToolCallStart{
					ToolCallId: tc.ID,
					Name:       tc.Function.Name,
				},
			},
		}); err != nil {
			slog.Warn("failed to send ToolCallStart", "err", err)
		}

		// Emit ToolCallDelta with full arguments
		if tc.Function.Arguments != "" {
			if err := rc.SendEvent(&v1pb.StreamMessageEvent{
				Event: &v1pb.StreamMessageEvent_ToolCallDelta{
					ToolCallDelta: &v1pb.ToolCallDelta{
						ToolCallId:     tc.ID,
						ArgumentsDelta: tc.Function.Arguments,
					},
				},
			}); err != nil {
				slog.Warn("failed to send ToolCallDelta", "err", err)
			}
		}

		// Track record for OnEnd correlation
		rc.mu.Lock()
		rc.records = append(rc.records, &ToolCallRecord{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
			Status:    v1pb.ToolCall_TOOL_CALL_RUNNING,
			StartTime: now,
		})
		rc.mu.Unlock()
	}

	return context.Background()
}

// onToolsNodeEnd fires when all tools in the node have finished. Each output message
// corresponds to a tool result, correlated by ToolCallID.
func (rc *ReactCallbacks) onToolsNodeEnd(_ context.Context, _ *callbacks.RunInfo, output []*schema.Message) context.Context {
	slog.Info("[react_callback] ToolsNode.OnEnd", "resultsCount", len(output))
	for _, msg := range output {
		if msg == nil {
			continue
		}
		slog.Info("[react_callback]   toolResult", "toolCallId", msg.ToolCallID, "contentLen", len(msg.Content), "contentPreview", preview(msg.Content, 200))

		duration := rc.computeDuration(msg.ToolCallID)

		if err := rc.SendEvent(&v1pb.StreamMessageEvent{
			Event: &v1pb.StreamMessageEvent_ToolCallEnd{
				ToolCallEnd: &v1pb.ToolCallEnd{
					ToolCallId: msg.ToolCallID,
					Result:     msg.Content,
					Status:     v1pb.ToolCall_TOOL_CALL_COMPLETED,
					DurationMs: duration,
				},
			},
		}); err != nil {
			slog.Warn("failed to send ToolCallEnd", "err", err)
		}

		// Update record
		rc.mu.Lock()
		for _, r := range rc.records {
			if r.ID == msg.ToolCallID {
				r.Result = msg.Content
				r.Status = v1pb.ToolCall_TOOL_CALL_COMPLETED
				r.DurationMs = duration
				break
			}
		}
		rc.mu.Unlock()
	}

	return context.Background()
}

// onToolsNodeError fires when the tools node encounters an error.
// We mark all still-running tool calls as failed.
func (rc *ReactCallbacks) onToolsNodeError(_ context.Context, _ *callbacks.RunInfo, err error) context.Context {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for _, r := range rc.records {
		if r.Status == v1pb.ToolCall_TOOL_CALL_RUNNING {
			r.Status = v1pb.ToolCall_TOOL_CALL_FAILED
			r.Result = err.Error()
		}
	}
	return context.Background()
}

// computeDuration calculates the elapsed time since the tool call started.
func (rc *ReactCallbacks) computeDuration(toolCallID string) int32 {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for _, r := range rc.records {
		if r.ID == toolCallID && !r.StartTime.IsZero() {
			return int32(time.Since(r.StartTime).Milliseconds())
		}
	}
	return 0
}

// preview returns a truncated version of s for logging (max n runes).
func preview(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}
