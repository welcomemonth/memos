package chatReAct

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	ucb "github.com/cloudwego/eino/utils/callbacks"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

// TraceStep represents one step in the ReAct trace, preserving the interleaved
// order of reasoning, model content, and tool calls across iterations.
type TraceStep struct {
	Type string `json:"type"` // "reasoning" | "content" | "tool_call"

	// For "reasoning" and "content" steps:
	Content string `json:"content,omitempty"`

	// For "tool_call" steps:
	ToolCallID   string `json:"tool_call_id,omitempty"`
	ToolName     string `json:"tool_name,omitempty"`
	Arguments    string `json:"arguments,omitempty"`
	Result       string `json:"result,omitempty"`
	Status       string `json:"status,omitempty"` // "running" | "completed" | "failed"
	DurationMs   int32  `json:"duration_ms,omitempty"`

	// internal: track start time for duration calculation
	startTime time.Time `json:"-"`
}

// ReactCallbacks wraps the Eino callback handler and collects:
// - A react_trace (ordered steps) for persistence
// - Real-time stream events for the frontend
type ReactCallbacks struct {
	Handler   callbacks.Handler
	SendEvent func(*v1pb.StreamMessageEvent) error

	mu    sync.Mutex
	trace []TraceStep
}

// NewReactCallbacks creates a callback handler that:
// 1. Emits ToolCall stream events to the frontend in real-time
// 2. Collects an ordered react_trace for persistence
//
// sendEvent MUST be safe for concurrent use (the caller should wrap stream.Send with a mutex).
func NewReactCallbacks(sendEvent func(*v1pb.StreamMessageEvent) error) *ReactCallbacks {
	rc := &ReactCallbacks{
		SendEvent: sendEvent,
		trace:     make([]TraceStep, 0),
	}

	rc.Handler = ucb.NewHandlerHelper().
		ChatModel(&ucb.ModelCallbackHandler{
			OnStart:               rc.onChatModelStart,
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

// Trace returns a copy of the collected react trace.
func (rc *ReactCallbacks) Trace() []TraceStep {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	out := make([]TraceStep, len(rc.trace))
	copy(out, rc.trace)
	return out
}

// TraceJSON returns the trace as a JSON string for storage in payload.
func (rc *ReactCallbacks) TraceJSON() string {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	b, err := json.Marshal(rc.trace)
	if err != nil {
		slog.Warn("failed to marshal react trace", "err", err)
		return "[]"
	}
	return string(b)
}

// --- ChatModel callbacks ---

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

// onChatModelStreamEnd consumes the forked stream copy to log the accumulated result.
// This does NOT affect the main output stream.
func (rc *ReactCallbacks) onChatModelStreamEnd(_ context.Context, _ *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
	if output == nil {
		return context.Background()
	}
	var lastContent, lastReasoning string
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
		"reasoningLen", len(lastReasoning),
		"toolCallsCount", len(lastToolCalls),
		"finishReason", lastFinishReason,
	)
	return context.Background()
}

// --- ToolsNode callbacks ---

// onToolsNodeStart fires when the tools node begins execution.
// The input message contains reasoning, content, and tool calls from the preceding LLM call.
// We record all of these into the trace AND emit stream events to the frontend.
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

	now := time.Now()

	// 1. Record reasoning step (model's thinking before calling tools)
	if input.ReasoningContent != "" {
		rc.mu.Lock()
		rc.trace = append(rc.trace, TraceStep{
			Type:    "reasoning",
			Content: input.ReasoningContent,
		})
		rc.mu.Unlock()

		// Also send to frontend in real-time
		if err := rc.SendEvent(&v1pb.StreamMessageEvent{
			Event: &v1pb.StreamMessageEvent_Reasoning{
				Reasoning: &v1pb.ReasoningDelta{Delta: input.ReasoningContent},
			},
		}); err != nil {
			slog.Warn("failed to send intermediate reasoning", "err", err)
		}
	}

	// 2. Record content step (model's transitional text before tool calls)
	if input.Content != "" {
		rc.mu.Lock()
		rc.trace = append(rc.trace, TraceStep{
			Type:    "content",
			Content: input.Content,
		})
		rc.mu.Unlock()
	}

	// 3. Record tool_call steps and emit stream events
	for _, tc := range input.ToolCalls {
		// Add to trace
		rc.mu.Lock()
		rc.trace = append(rc.trace, TraceStep{
			Type:       "tool_call",
			ToolCallID: tc.ID,
			ToolName:   tc.Function.Name,
			Arguments:  tc.Function.Arguments,
			Status:     "running",
			startTime:  now,
		})
		rc.mu.Unlock()

		// Emit ToolCallStart to frontend
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

		slog.Info("[react_callback]   toolCall",
			"id", tc.ID, "name", tc.Function.Name,
			"args", preview(tc.Function.Arguments, 200),
		)
	}

	return context.Background()
}

// onToolsNodeEnd fires when all tools in the node have finished.
// We update the trace entries with results AND emit ToolCallEnd events.
func (rc *ReactCallbacks) onToolsNodeEnd(_ context.Context, _ *callbacks.RunInfo, output []*schema.Message) context.Context {
	slog.Info("[react_callback] ToolsNode.OnEnd", "resultsCount", len(output))

	for _, msg := range output {
		if msg == nil {
			continue
		}

		duration := rc.computeDuration(msg.ToolCallID)

		slog.Info("[react_callback]   toolResult",
			"toolCallId", msg.ToolCallID,
			"contentLen", len(msg.Content),
			"contentPreview", preview(msg.Content, 200),
			"durationMs", duration,
		)

		// Update trace entry with result
		rc.mu.Lock()
		for i := range rc.trace {
			if rc.trace[i].Type == "tool_call" && rc.trace[i].ToolCallID == msg.ToolCallID {
				rc.trace[i].Result = msg.Content
				rc.trace[i].Status = "completed"
				rc.trace[i].DurationMs = duration
				break
			}
		}
		rc.mu.Unlock()

		// Emit ToolCallEnd to frontend
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
	}

	return context.Background()
}

// onToolsNodeError fires when the tools node encounters an error.
func (rc *ReactCallbacks) onToolsNodeError(_ context.Context, _ *callbacks.RunInfo, err error) context.Context {
	slog.Warn("[react_callback] ToolsNode.OnError", "err", err)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for i := range rc.trace {
		if rc.trace[i].Type == "tool_call" && rc.trace[i].Status == "running" {
			rc.trace[i].Status = "failed"
			rc.trace[i].Result = err.Error()
		}
	}
	return context.Background()
}

// computeDuration calculates the elapsed time since the tool call started.
func (rc *ReactCallbacks) computeDuration(toolCallID string) int32 {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	for _, step := range rc.trace {
		if step.Type == "tool_call" && step.ToolCallID == toolCallID && !step.startTime.IsZero() {
			return int32(time.Since(step.startTime).Milliseconds())
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
