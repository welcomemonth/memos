import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslate } from "@/utils/i18n";
import {
  listConversations,
  listMessages,
  streamMessage,
  deleteConversation as deleteConversationApi,
  generateConversationId,
  type StreamEvent,
} from "@/lib/aiChat";
import { AIMessage_Role } from "@/types/proto/api/v1/ai_conversion_service_pb";
import { Routes } from "@/router";
import ChatView from "./components/ChatView";
import Sidebar from "./components/Sidebar";
import type { ChatMessage, Conversation, SidebarTab } from "./types";

/** Convert API conversation to local Conversation type */
const toLocalConversation = (apiConv: {
  name: string;
  title: string;
  model?: string;
  updateTime?: { seconds: bigint };
}): Conversation => ({
  id: apiConv.name,
  name: apiConv.name,
  title: apiConv.title || "新对话",
  model: apiConv.model,
  updatedAt: apiConv.updateTime ? Number(apiConv.updateTime.seconds) * 1000 : Date.now(),
  messages: [],
});

/** Convert API message to local ChatMessage type */
const toLocalMessage = (apiMsg: {
  name: string;
  role: number;
  content: string;
  model?: string;
  reasoningItems?: Array<{ content: string }>;
  createTime?: { seconds: bigint };
}): ChatMessage => ({
  id: apiMsg.name,
  role: apiMsg.role === AIMessage_Role.USER ? "user" : "assistant",
  content: apiMsg.content,
  model: apiMsg.model,
  reasoning: apiMsg.reasoningItems?.map((r) => r.content).filter(Boolean).join("\n\n") || undefined,
  timestamp: apiMsg.createTime ? Number(apiMsg.createTime.seconds) * 1000 : Date.now(),
});

const AIChat = () => {
  const t = useTranslate();
  const navigate = useNavigate();
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConvId, setActiveConvId] = useState<string>("");
  const [activeTab, setActiveTab] = useState<SidebarTab>("chat");
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState("");
  const [streamingReasoning, setStreamingReasoning] = useState("");
  const [selectedKnowledgeBases, setSelectedKnowledgeBases] = useState<string[]>([]);

  // Track the currently streaming conversation to avoid stale closures
  const streamingConvIdRef = useRef<string>("");

  // Load conversations from API, preserving locally-held messages (optimistic user
  // messages added during streaming) and local-only conversations that haven't
  // synced to the server yet.
  const loadConversations = useCallback(async () => {
    try {
      const list = await listConversations();
      const mapped = list.map(toLocalConversation);
      setConversations((prev) => {
        const prevById = new Map(prev.map((c) => [c.id, c]));
        const updated = mapped.map((c) => {
          const local = prevById.get(c.id);
          if (local) {
            prevById.delete(c.id);
            return { ...c, messages: local.messages };
          }
          return c;
        });
        // Keep local-only conversations (e.g., just-created during streaming)
        const localOnly = Array.from(prevById.values());
        return [...updated, ...localOnly];
      });
    } catch (err) {
      console.error("Failed to load conversations:", err);
    }
  }, []);

  // Load messages for a specific conversation (only on conversation switch)
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

  // Load messages only when switching conversations (not during streaming)
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
      streamingConvIdRef.current = convName;

      await streamMessage(
        convName,
        content,
        selectedKnowledgeBases[0] ?? "",
        (event: StreamEvent) => {
          if (event.conversationCreated) {
            const serverName = event.conversationCreated.name;
            const prevId = streamingConvIdRef.current;
            streamingConvIdRef.current = serverName;

            setConversations((prev) =>
              prev.map((conv) =>
                conv.id === prevId
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
            // Append assistant message directly to local state (no API reload)
            const currentConvId = streamingConvIdRef.current;
            const assistantMsg: ChatMessage = {
              id: event.done.messageName || `assistant-${Date.now()}`,
              role: "assistant",
              content: streamingContentRef.current,
              reasoning: streamingReasoningRef.current || undefined,
              model: event.done.model,
              timestamp: Date.now(),
            };

            setConversations((prev) =>
              prev.map((conv) =>
                conv.id === currentConvId
                  ? { ...conv, messages: [...conv.messages, assistantMsg], updatedAt: Date.now() }
                  : conv,
              ),
            );

            setStreamingContent("");
            setStreamingReasoning("");
            setIsStreaming(false);
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
    [activeConvId, isStreaming, loadConversations],
  );

  // Refs to track latest streaming state for the done handler (avoid stale closures)
  const streamingContentRef = useRef("");
  const streamingReasoningRef = useRef("");
  useEffect(() => {
    streamingContentRef.current = streamingContent;
  }, [streamingContent]);
  useEffect(() => {
    streamingReasoningRef.current = streamingReasoning;
  }, [streamingReasoning]);

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
        onTabChange={(tab) => {
          if (tab === "knowledge") {
            navigate(Routes.KNOWLEDGE);
          } else {
            setActiveTab(tab);
          }
        }}
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
          selectedKnowledgeBases={selectedKnowledgeBases}
          onSelectedKnowledgeBasesChange={setSelectedKnowledgeBases}
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
