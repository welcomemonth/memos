import { useEffect, useRef, useState } from "react";
import {
  listConversations,
  listMessages,
  streamMessage,
  deleteConversation,
  generateConversationId,
  type AIConversation,
  type AIMessage,
  type StreamEvent,
} from "@/lib/aiChat";
import { AIMessage_Role } from "@/types/proto/api/v1/ai_conversion_pb";

export default function AIChat() {
  const [conversations, setConversations] = useState<AIConversation[]>([]);
  const [selectedConv, setSelectedConv] = useState<string | null>(null);
  const [messages, setMessages] = useState<AIMessage[]>([]);
  const [input, setInput] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamingContent, setStreamingContent] = useState("");
  const [streamingReasoning, setStreamingReasoning] = useState("");
  const [reasoningExpanded, setReasoningExpanded] = useState(true);
  const [pendingUserContent, setPendingUserContent] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // 加载对话列表
  useEffect(() => {
    loadConversations();
  }, []);

  // 加载选中对话的消息（streaming 期间跳过，由 done 事件统一加载）
  useEffect(() => {
    if (selectedConv && !isStreaming) {
      loadMessages(selectedConv);
    } else if (!selectedConv) {
      setMessages([]);
    }
  }, [selectedConv]);

  // 自动滚动到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, streamingContent]);

  const loadConversations = async () => {
    try {
      const list = await listConversations();
      setConversations(list);
    } catch (err) {
      console.error("Failed to load conversations:", err);
    }
  };

  const loadMessages = async (convName: string) => {
    try {
      const msgs = await listMessages(convName);
      setMessages(msgs);
    } catch (err) {
      console.error("Failed to load messages:", err);
    }
  };

  const handleNewConversation = () => {
    setSelectedConv(null);
    setMessages([]);
    setStreamingContent("");
    setStreamingReasoning("");
    setPendingUserContent("");
  };

  const handleDeleteConversation = async (name: string) => {
    if (!confirm("确定要删除这个对话吗？")) return;
    try {
      await deleteConversation(name);
      setConversations((prev) => prev.filter((c) => c.name !== name));
      if (selectedConv === name) {
        setSelectedConv(null);
        setMessages([]);
      }
    } catch (err) {
      console.error("Failed to delete conversation:", err);
    }
  };

  const handleSend = async () => {
    if (!input.trim() || isStreaming) return;

    const content = input.trim();
    setInput("");

    // 如果没有选中的对话，生成一个新的（仅作为流式请求的 parent，不立即设置 selectedConv）
    const convName = selectedConv || generateConversationId();
    // 注意：不在这里调用 setSelectedConv(convName)
    // 新对话等待后端 conversationCreated 事件到达后再设置，
    // 避免 useEffect 提前触发 loadMessages 导致 404（conversation 尚未创建）

    setIsStreaming(true);
    setStreamingContent("");
    setStreamingReasoning("");
    setReasoningExpanded(true);
    setPendingUserContent(content);

    await streamMessage(
      convName,
      content,
      (event: StreamEvent) => {
        console.log("[AIChat] stream event:", event);
        // 处理 conversation created 事件
        if (event.conversationCreated) {
          console.log("[AIChat] conversation created:", event.conversationCreated.name);
          setSelectedConv(event.conversationCreated.name);
          // 刷新对话列表
          loadConversations();
        }
        // 处理内容增量
        if (event.content?.delta) {
          console.log("[AIChat] content delta:", event.content.delta.slice(0, 50));
          setStreamingContent((prev) => prev + event.content!.delta);
        }
        // 处理推理过程增量
        if (event.reasoning?.delta) {
          setStreamingReasoning((prev) => prev + event.reasoning!.delta);
        }
        // 处理完成事件
        if (event.done) {
          console.log("[AIChat] stream done, loading messages for:", convName);
          // 清除 pending 状态
          setPendingUserContent("");
          setStreamingContent("");
          setStreamingReasoning("");
          setIsStreaming(false);
          // 重新加载消息（convName 已经是正确的 conversation name）
          loadMessages(convName);
        }
        // 处理错误事件
        if (event.error) {
          console.error("[AIChat] stream error:", event.error);
          setStreamingContent((prev) => prev + `\n\n[错误: ${event.error!.message}]`);
          setIsStreaming(false);
        }
      },
      (err: Error) => {
        console.error("[AIChat] stream failed:", err);
        setStreamingContent((prev) => prev + `\n\n[请求失败: ${err.message}]`);
        setIsStreaming(false);
      },
      () => {
        console.log("[AIChat] stream onComplete called");
        setIsStreaming(false);
      },
    );
  };

  return (
    <div className="flex h-[calc(100vh-8rem)]">
      {/* 左侧：对话列表 */}
      <aside className="w-64 border-r border-border flex flex-col">
        <div className="p-4 border-b border-border">
          <button
            onClick={handleNewConversation}
            className="w-full px-4 py-2 bg-primary text-primary-foreground rounded hover:bg-primary/90"
          >
            + 新对话
          </button>
        </div>
        <div className="flex-1 overflow-auto p-2">
          {conversations.map((conv) => (
            <div
              key={conv.name}
              className={`p-3 mb-2 rounded cursor-pointer hover:bg-accent ${
                selectedConv === conv.name ? "bg-accent" : ""
              }`}
              onClick={() => setSelectedConv(conv.name)}
            >
              <div className="flex justify-between items-start">
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium truncate">
                    {conv.title || "新对话"}
                  </div>
                  <div className="text-xs text-muted-foreground truncate">
                    {conv.model || "默认模型"}
                  </div>
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDeleteConversation(conv.name);
                  }}
                  className="text-xs text-muted-foreground hover:text-destructive"
                >
                  ×
                </button>
              </div>
            </div>
          ))}
          {conversations.length === 0 && (
            <div className="text-center text-muted-foreground text-sm py-8">
              暂无对话
            </div>
          )}
        </div>
      </aside>

      {/* 右侧：聊天区域 */}
      <main className="flex-1 flex flex-col">
        {/* 消息列表 */}
        <div className="flex-1 overflow-auto p-4">
          {messages.length === 0 && !isStreaming && (
            <div className="flex items-center justify-center h-full text-muted-foreground">
              开始新的对话吧
            </div>
          )}
          {messages.map((msg) => (
            <div
              key={msg.name}
              className={`mb-4 ${msg.role === AIMessage_Role.USER ? "text-right" : "text-left"}`}
            >
              <div
                className={`inline-block max-w-[80%] px-4 py-2 rounded-lg ${
                  msg.role === AIMessage_Role.USER
                    ? "bg-primary text-primary-foreground"
                    : "bg-accent text-accent-foreground"
                }`}
              >
                <div className="whitespace-pre-wrap break-words">{msg.content}</div>
                {msg.model && (
                  <div className="text-xs opacity-70 mt-1">{msg.model}</div>
                )}
              </div>
            </div>
          ))}
          {/* 显示待发送的用户消息 */}
          {pendingUserContent && (
            <div className="mb-4 text-right">
              <div className="inline-block max-w-[80%] px-4 py-2 rounded-lg bg-primary text-primary-foreground">
                <div className="whitespace-pre-wrap break-words">{pendingUserContent}</div>
              </div>
            </div>
          )}
          {/* 流式推理过程（可折叠） */}
          {isStreaming && streamingReasoning && (
            <div className="mb-2 text-left">
              <div className="inline-block max-w-[80%]">
                <button
                  onClick={() => setReasoningExpanded((prev) => !prev)}
                  className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors mb-1"
                >
                  <span className="inline-block animate-spin">⟳</span>
                  <span>思考中…</span>
                  <span className="text-muted-foreground/50">{reasoningExpanded ? "▼" : "▶"}</span>
                </button>
                {reasoningExpanded && (
                  <div className="text-xs text-muted-foreground/70 whitespace-pre-wrap break-words pl-4 border-l-2 border-muted">
                    {streamingReasoning}
                  </div>
                )}
              </div>
            </div>
          )}
          {/* 流式推理中，尚未有推理内容 */}
          {isStreaming && !streamingReasoning && !streamingContent && (
            <div className="mb-4 text-left">
              <div className="inline-block px-4 py-2 rounded-lg bg-accent text-accent-foreground">
                <span className="animate-pulse">AI 正在思考...</span>
              </div>
            </div>
          )}
          {/* 流式正文内容 */}
          {isStreaming && streamingContent && (
            <div className="mb-4 text-left">
              <div className="inline-block max-w-[80%] px-4 py-2 rounded-lg bg-accent text-accent-foreground">
                <div className="whitespace-pre-wrap break-words">{streamingContent}</div>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* 输入框 */}
        <div className="border-t border-border p-4">
          <div className="flex gap-2">
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  handleSend();
                }
              }}
              placeholder="输入消息... (Enter 发送)"
              disabled={isStreaming}
              className="flex-1 px-4 py-2 border border-border rounded focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50"
            />
            <button
              onClick={handleSend}
              disabled={isStreaming || !input.trim()}
              className="px-6 py-2 bg-primary text-primary-foreground rounded hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              发送
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
