import { aiConversationServiceClient } from "@/connect";
import type { AIConversation, AIMessage } from "@/types/proto/api/v1/ai_conversion_pb";
import {
  ListConversationsRequestSchema,
  ListMessagesRequestSchema,
  SendMessageRequestSchema,
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

// 发送消息（非流式，模拟流式事件回调）
export async function sendMessage(
  conversationName: string,
  content: string,
  onEvent: (event: StreamEvent) => void,
  onError?: (error: Error) => void,
  onComplete?: () => void,
): Promise<void> {
  try {
    const request = create(SendMessageRequestSchema, {
      parent: conversationName,
      content,
    });

    const response = await aiConversationServiceClient.sendMessage(request);

    // 如果有新创建的对话，发送 conversationCreated 事件
    if (response.conversation) {
      onEvent({
        conversationCreated: {
          name: response.conversation.name,
          title: response.conversation.title,
          model: response.conversation.model,
        },
      });
    }

    // 发送消息内容
    if (response.message) {
      onEvent({
        content: { delta: response.message.content },
      });
      onEvent({
        done: {
          messageName: response.message.name,
          tokensUsed: response.message.tokensUsed,
          model: response.message.model,
        },
      });
    }

    onComplete?.();
  } catch (err) {
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
