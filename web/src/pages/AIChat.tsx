import { useEffect, useRef, useState } from "react";
import {
  listConversations,
  listMessages,
  sendMessage,
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
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // 加载对话列表
  useEffect(() => {
    loadConversations();
  }, []);

  // 加载选中对话的消息
  useEffect(() => {
    if (selectedConv) {
      loadMessages(selectedConv);
    } else {
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

    // 如果没有选中的对话，生成一个新的
    const convName = selectedConv || generateConversationId();
    if (!selectedConv) {
      setSelectedConv(convName);
    }

    setIsStreaming(true);
    setStreamingContent("");

    // 立即重新加载消息以显示用户消息（服务端已保存）
    setTimeout(() => loadMessages(convName), 200);

    await sendMessage(
      convName,
      content,
      (event: StreamEvent) => {
        // 处理 conversation created 事件
        if (event.conversationCreated) {
          setSelectedConv(event.conversationCreated.name);
          // 刷新对话列表
          loadConversations();
        }
        // 处理内容增量
        if (event.content?.delta) {
          setStreamingContent((prev) => prev + event.content!.delta);
        }
        // 处理完成事件
        if (event.done) {
          // 重新加载消息以获取完整的 assistant 消息
          loadMessages(convName);
          setStreamingContent("");
          setIsStreaming(false);
        }
        // 处理错误事件
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
          {isStreaming && streamingContent && (
            <div className="mb-4 text-left">
              <div className="inline-block max-w-[80%] px-4 py-2 rounded-lg bg-accent text-accent-foreground">
                <div className="whitespace-pre-wrap break-words">{streamingContent}</div>
              </div>
            </div>
          )}
          {isStreaming && !streamingContent && (
            <div className="mb-4 text-left">
              <div className="inline-block px-4 py-2 rounded-lg bg-accent text-accent-foreground">
                <span className="animate-pulse">AI 正在思考...</span>
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
