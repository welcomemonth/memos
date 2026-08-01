import { aiConversationServiceClient } from "@/connect";
import type { AIConversation, AIMessage, StreamMessageEvent } from "@/types/proto/api/v1/ai_conversion_pb";
import {
  ListConversationsRequestSchema,
  ListMessagesRequestSchema,
  SendMessageRequestSchema,
  StreamMessageRequestSchema,
  UpdateConversationRequestSchema,
  DeleteConversationRequestSchema,
} from "@/types/proto/api/v1/ai_conversion_pb";
import { create } from "@bufbuild/protobuf";
import { FieldMaskSchema } from "@bufbuild/protobuf/wkt";

// Re-export types for convenience
export type { AIConversation, AIMessage };

// 获取对话列表
export async function listConversations(): Promise<AIConversation[]> {
  const request = create(ListConversationsRequestSchema, {});
  const response = await aiConversationServiceClient.listConversations(request);
  return response.conversations;
}

// 获取对话详情
export async function getConversation(name: string): Promise<AIConversation> {
  return await aiConversationServiceClient.getConversation({ name });
}

// 更新对话标题
export async function updateConversationTitle(name: string, title: string): Promise<AIConversation> {
  const request = create(UpdateConversationRequestSchema, {
    conversation: { name, title },
    updateMask: create(FieldMaskSchema, { paths: ["title"] }),
  });
  return await aiConversationServiceClient.updateConversation(request);
}

// 删除对话
export async function deleteConversation(name: string): Promise<void> {
  const request = create(DeleteConversationRequestSchema, { name });
  await aiConversationServiceClient.deleteConversation(request);
}

// 获取对话消息列表
export async function listMessages(conversationName: string): Promise<AIMessage[]> {
  const request = create(ListMessagesRequestSchema, { parent: conversationName });
  const response = await aiConversationServiceClient.listMessages(request);
  return response.messages;
}

// 发送消息（流式）
export async function streamMessage(
  conversationName: string,
  content: string,
  onEvent: (event: StreamEvent) => void,
  onError?: (error: Error) => void,
  onComplete?: () => void,
): Promise<void> {
  try {
    console.log("[streamMessage] start", { conversationName, content });
    const request = create(StreamMessageRequestSchema, {
      parent: conversationName,
      content,
    });

    // 使用 Connect 协议的 server-streaming
    const stream = aiConversationServiceClient.streamMessage(request);
    console.log("[streamMessage] stream created, iterating...");

    for await (const event of stream) {
      console.log("[streamMessage] received event, case=", event.event.case, event.event);
      // 转换 proto event 到我们的 StreamEvent 类型
      if (event.event.case === "conversationCreated") {
        onEvent({
          conversationCreated: {
            name: event.event.value.name,
            title: event.event.value.title,
            model: event.event.value.model,
          },
        });
      } else if (event.event.case === "reasoning") {
        onEvent({
          reasoning: { delta: event.event.value.delta },
        });
      } else if (event.event.case === "content") {
        onEvent({
          content: { delta: event.event.value.delta },
        });
      } else if (event.event.case === "done") {
        onEvent({
          done: {
            messageName: event.event.value.messageName,
            tokensUsed: event.event.value.tokensUsed,
            model: event.event.value.model,
          },
        });
      } else if (event.event.case === "error") {
        onEvent({
          error: {
            message: event.event.value.message,
            code: event.event.value.code,
          },
        });
      }
    }

    console.log("[streamMessage] stream completed normally");
    onComplete?.();
  } catch (err) {
    console.error("[streamMessage] caught error:", err);
    onError?.(err instanceof Error ? err : new Error(String(err)));
  }
}

// 流式事件类型
export interface StreamEvent {
  conversationCreated?: { name: string; title: string; model: string };
  reasoning?: { delta: string };
  content?: { delta: string };
  toolCallStart?: { toolCallId: string; name: string };
  toolCallDelta?: { toolCallId: string; argumentsDelta: string };
  toolCallEnd?: { toolCallId: string; result: string; status: string; durationMs: number };
  done?: { messageName: string; tokensUsed: number; model: string };
  error?: { message: string; code: string };
}

// 生成新的对话 ID
export function generateConversationId(): string {
  return `ai/conversations/conv-${Date.now()}`;
}

// 检查是否是有效的对话名称
export function isValidConversationName(name: string): boolean {
  return name.startsWith("ai/conversations/") && name.length > "ai/conversations/".length;
}
