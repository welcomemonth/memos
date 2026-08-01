import { useCallback, useEffect, useState } from "react";
import { useTranslate } from "@/utils/i18n";
import {
  listConversations,
  listMessages,
  streamMessage,
  deleteConversation as deleteConversationApi,
  generateConversationId,
  type StreamEvent,
} from "@/lib/aiChat";
import { AIMessage_Role } from "@/types/proto/api/v1/ai_conversion_pb";
import ChatView from "./components/ChatView";
import Sidebar from "./components/Sidebar";
import type { ChatMessage, Conversation, SidebarTab } from "./types";

/** Convert API conversation to local Conversation type */
const toLocalConversation = (apiConv: { name: string; title: string; model?: string; updateTime?: { seconds: bigint } }): Conversation => ({
  id: apiConv.name,
  name: apiConv.name,
  title: apiConv.title || "新对话",
  model: apiConv.model,
  updatedAt: apiConv.updateTime ? Number(apiConv.updateTime.seconds) * 1000 : Date.now(),
  messages: [],
});

/** Convert API message to local ChatMessage type */
const toLocalMessage = (apiMsg: { name: string; role: number; content: string; model?: string; createTime?: { seconds: bigint } }): ChatMessage => ({
  id: apiMsg.name,
  role: apiMsg.role === AIMessage_Role.USER ? "user" : "assistant",
  content: apiMsg.content,
  model: apiMsg.model,
  timestamp: apiMsg.createTime ? Number(apiMsg.createTime.seconds) * 1000 : Date.now(),
});

const AIChat = () => {
  const t = useTranslate();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConvId, setActiveConvId] = useState<string>("");
  const [activeTab, setActiveTab] = useState<SidebarTab>("chat");
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState("");
  const [streamingReasoning, setStreamingReasoning] = useState("");

  // Load conversations from API
  const loadConversations = useCallback(async () => {
    try {
      const list = await listConversations();
      const mapped = list.map(toLocalConversation);
      setConversations(mapped);
    } catch (err) {
      console.error("Failed to load conversations:", err);
    }
  }, []);

  // Load messages for a specific conversation
  const loadMessages = useCallback(async (convName: string) => {
    try {
      const msgs = await listMessages(convName);
      const mapped = msgs.map(toLocalMessage);
      setConversations((prev) =>
        prev.map((conv) => (conv.id === convName ? { ...conv, messages: mapped } : conv)),
      );
    } catch (err) {
      console.error("Failed to load messages:", err);
    }
  }, []);

  useEffect(() => {
    loadConversations();
  }, [loadConversations]);

  // When activeConvId changes, load messages (unless streaming)
  useEffect(() => {
    if (activeConvId && !isStreaming) {
      loadMessages(activeConvId);
    }
  }, [activeConvId, isStreaming, loadMessages]);

  const activeConversation = conversations.find((c) => c.id === activeConvId);
  const messages = activeConversation?.messages ?? [];

  const handleSend = useCallback(
    async (content: string) => {
      if (!content.trim() || isStreaming) return;

      // Generate new conversation ID if none selected
      const convName = activeConvId || generateConversationId();

      // Optimistically add user message
      const userMsg: ChatMessage = {
        id: `temp-${Date.now()}`,
        role: "user",
        content,
        timestamp: Date.now(),
      };

      setConversations((prev) => {
        const existing = prev.find((c) => c.id === convName);
        if (existing) {
          return prev.map((conv) =>
            conv.id === convName
              ? {
                  ...conv,
                  messages: [...conv.messages, userMsg],
                  title: conv.messages.length === 0 ? content.slice(0, 30) : conv.title,
                  updatedAt: Date.now(),
                }
              : conv,
          );
        }
        // New conversation
        const newConv: Conversation = {
          id: convName,
          name: convName,
          title: content.slice(0, 30),
          updatedAt: Date.now(),
          messages: [userMsg],
        };
        return [newConv, ...prev];
      });

      if (!activeConvId) {
        setActiveConvId(convName);
      }

      setIsStreaming(true);
      setStreamingContent("");
      setStreamingReasoning("");

      await streamMessage(
        convName,
        content,
        (event: StreamEvent) => {
          if (event.conversationCreated) {
            // Update the conversation ID with the server's confirmed name
            const serverName = event.conversationCreated.name;
            setConversations((prev) =>
              prev.map((conv) =>
                conv.id === convName
                  ? {
                      ...conv,
                      id: serverName,
                      name: serverName,
                      title: event.conversationCreated!.title || conv.title,
                      model: event.conversationCreated!.model,
                    }
                  : conv,
              ),
            );
            setActiveConvId(serverName);
            loadConversations();
          }
          if (event.content?.delta) {
            setStreamingContent((prev) => prev + event.content!.delta);
          }
          if (event.reasoning?.delta) {
            setStreamingReasoning((prev) => prev + event.reasoning!.delta);
          }
          if (event.done) {
            setIsStreaming(false);
            setStreamingContent("");
            setStreamingReasoning("");
            // Reload messages from API to get persisted data
            loadMessages(activeConvId || convName);
          }
          if (event.error) {
            console.error("Stream error:", event.error);
            setStreamingContent((prev) => prev + `\n\n[错误: ${event.error!.message}]`);
            setIsStreaming(false);
          }
        },
        (err: Error) => {
          console.error("Stream failed:", err);
          setStreamingContent((prev) => prev + `\n\n[请求失败: ${err.message}]`);
          setIsStreaming(false);
        },
        () => {
          setIsStreaming(false);
        },
      );
    },
    [activeConvId, isStreaming, loadConversations, loadMessages],
  );

  const handleNewConversation = useCallback(() => {
    setActiveConvId("");
  }, []);

  const handleDeleteConversation = useCallback(
    async (convId: string) => {
      if (!confirm("确定要删除这个对话吗？")) return;
      try {
        await deleteConversationApi(convId);
        setConversations((prev) => prev.filter((c) => c.id !== convId));
        if (activeConvId === convId) {
          setActiveConvId("");
        }
      } catch (err) {
        console.error("Failed to delete conversation:", err);
      }
    },
    [activeConvId],
  );

  return (
    <section className="w-full h-svh flex overflow-hidden bg-background">
      <Sidebar
        activeTab={activeTab}
        onTabChange={setActiveTab}
        conversations={conversations}
        activeConvId={activeConvId}
        onSelectConversation={setActiveConvId}
        onNewConversation={handleNewConversation}
        onDeleteConversation={handleDeleteConversation}
      />
      {activeTab === "chat" ? (
        <ChatView
          messages={messages}
          onSend={handleSend}
          isStreaming={isStreaming}
          streamingContent={streamingContent}
          streamingReasoning={streamingReasoning}
        />
      ) : (
        <div className="flex-1 flex items-center justify-center text-muted-foreground">
          <p className="text-sm">{t("ai-chat.coming-soon")}</p>
        </div>
      )}
    </section>
  );
};

export default AIChat;
