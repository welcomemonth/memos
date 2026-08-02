import { useCallback, useEffect, useRef, useState } from "react";
import { BotIcon, BookOpenIcon, ChevronDownIcon, CodeIcon, LightbulbIcon, SendIcon, SparklesIcon, XIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { listKnowledgeBases, type KnowledgeBase } from "@/lib/knowledge";
import useCurrentUser from "@/hooks/useCurrentUser";
import { useTranslate } from "@/utils/i18n";
import MessageBubble from "./MessageBubble";
import type { ChatMessage } from "../types";

interface Props {
  messages: ChatMessage[];
  onSend: (content: string) => void;
  isStreaming: boolean;
  streamingContent: string;
  streamingReasoning: string;
  selectedKnowledgeBases: string[];
  onSelectedKnowledgeBasesChange: (names: string[]) => void;
}

const PROMPT_SUGGESTIONS = [
  { icon: CodeIcon, text: "ai-chat.prompt-code" },
  { icon: LightbulbIcon, text: "ai-chat.prompt-explain" },
  { icon: SparklesIcon, text: "ai-chat.prompt-summarize" },
];

const ChatView = ({
  messages,
  onSend,
  isStreaming,
  streamingContent,
  streamingReasoning,
  selectedKnowledgeBases,
  onSelectedKnowledgeBasesChange,
}: Props) => {
  const t = useTranslate();
  const currentUser = useCurrentUser();
  const [input, setInput] = useState("");
  const [streamingReasoningExpanded, setStreamingReasoningExpanded] = useState(false);
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>([]);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  // Load knowledge bases for selection
  useEffect(() => {
    listKnowledgeBases()
      .then(setKnowledgeBases)
      .catch((err) => console.error("Failed to load knowledge bases:", err));
  }, []);

  // Only auto-scroll when a new message is added, NOT during streaming
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length]);

  const handleSend = useCallback(() => {
    const content = input.trim();
    if (!content || isStreaming) return;
    onSend(content);
    setInput("");
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
    }
    // Scroll to bottom when user sends a message
    setTimeout(() => {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }, 50);
  }, [input, isStreaming, onSend]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInput(e.target.value);
    const el = e.target;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 200) + "px";
  }, []);

  const handleSuggestionClick = useCallback((promptKey: string) => {
    const prompts: Record<string, string> = {
      "ai-chat.prompt-code": "帮我写一段代码：",
      "ai-chat.prompt-explain": "请解释一下这个概念：",
      "ai-chat.prompt-summarize": "请帮我总结一下这段内容：",
    };
    setInput(prompts[promptKey] || "");
    textareaRef.current?.focus();
  }, []);

  const toggleKnowledgeBase = useCallback(
    (name: string) => {
      const next = selectedKnowledgeBases.includes(name)
        ? selectedKnowledgeBases.filter((n) => n !== name)
        : [...selectedKnowledgeBases, name];
      onSelectedKnowledgeBasesChange(next);
    },
    [selectedKnowledgeBases, onSelectedKnowledgeBasesChange],
  );

  const selectedKBs = knowledgeBases.filter((kb) => selectedKnowledgeBases.includes(kb.name));

  const isEmpty = messages.length === 0 && !isStreaming;

  return (
    <div className="flex-1 flex flex-col min-h-0 min-w-0 bg-background">
      {/* ── Header ──────────────────────────────────────────── */}
      <header className="shrink-0 px-4 py-2.5 border-b border-border flex items-center justify-between">
        <div className="flex items-center gap-2">
          <BotIcon className="w-5 h-5 text-primary" />
          <h2 className="text-sm font-semibold text-foreground">{t("ai-chat.title")}</h2>
        </div>
        <Popover>
          <PopoverTrigger asChild>
            <Button variant="outline" size="sm" className="h-7 gap-1.5 text-xs">
              <BookOpenIcon className="w-3.5 h-3.5" />
              {t("ai-chat.knowledge-base")}
              {selectedKBs.length > 0 && (
                <Badge variant="secondary" className="ml-0.5 text-[10px] px-1 py-0 min-w-0">
                  {selectedKBs.length}
                </Badge>
              )}
              <ChevronDownIcon className="w-3 h-3 text-muted-foreground" />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" className="w-64 p-0">
            <div className="px-3 py-2 border-b border-border">
              <p className="text-xs font-medium text-foreground">{t("ai-chat.kb-select")}</p>
            </div>
            <div className="max-h-60 overflow-y-auto">
              {knowledgeBases.length === 0 ? (
                <div className="px-3 py-4 text-center">
                  <p className="text-xs text-muted-foreground">{t("ai-chat.kb-no-items")}</p>
                </div>
              ) : (
                knowledgeBases.map((kb) => (
                  <button
                    key={kb.name}
                    className="w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent/50 transition-colors"
                    onClick={() => toggleKnowledgeBase(kb.name)}
                  >
                    <Checkbox
                      checked={selectedKnowledgeBases.includes(kb.name)}
                      className="pointer-events-none"
                    />
                    <div className="flex-1 min-w-0">
                      <p className="truncate text-foreground">{kb.title}</p>
                      {kb.description && (
                        <p className="text-[11px] text-muted-foreground truncate">{kb.description}</p>
                      )}
                    </div>
                  </button>
                ))
              )}
            </div>
          </PopoverContent>
        </Popover>
      </header>

      {/* ── Messages ────────────────────────────────────────── */}
      <div ref={scrollContainerRef} className="flex-1 overflow-y-auto px-4 py-4">
        {isEmpty ? (
          <div className="flex flex-col items-center justify-center h-full text-center px-4">
            <div className="w-16 h-16 rounded-2xl bg-primary/10 flex items-center justify-center mb-5">
              <BotIcon className="w-8 h-8 text-primary" />
            </div>
            <h3 className="text-lg font-semibold text-foreground mb-2">{t("ai-chat.welcome")}</h3>
            <p className="text-sm text-muted-foreground max-w-md mb-8">{t("ai-chat.welcome-description")}</p>

            <div className="flex flex-wrap justify-center gap-2">
              {PROMPT_SUGGESTIONS.map(({ icon: Icon, text }) => (
                <button
                  key={text}
                  onClick={() => handleSuggestionClick(text)}
                  className="flex items-center gap-1.5 px-3 py-2 rounded-lg border border-border bg-muted/30 text-sm text-foreground/80 hover:bg-muted/60 hover:border-border transition-colors"
                >
                  <Icon className="w-3.5 h-3.5 text-muted-foreground" />
                  <span>{t(text as any)}</span>
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="max-w-3xl mx-auto space-y-4">
            {messages.map((msg) => (
              <MessageBubble key={msg.id} message={msg} avatarUrl={currentUser?.avatarUrl} />
            ))}

            {/* ── Streaming: Reasoning (collapsible, default collapsed) ── */}
            {isStreaming && streamingReasoning && (
              <div className="flex gap-3">
                <div className="shrink-0 mt-0.5">
                  <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
                    <BotIcon className="w-4 h-4 text-primary" />
                  </div>
                </div>
                <div className="max-w-[75%]">
                  <button
                    onClick={() => setStreamingReasoningExpanded((prev) => !prev)}
                    className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors mb-1"
                  >
                    <span className="inline-block animate-spin">⟳</span>
                    <span>思考中…</span>
                    <span className="text-muted-foreground/50">{streamingReasoningExpanded ? "▼" : "▶"}</span>
                  </button>
                  {streamingReasoningExpanded && (
                    <div className="text-xs text-muted-foreground/70 whitespace-pre-wrap break-words pl-4 border-l-2 border-border">
                      {streamingReasoning}
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* ── Streaming: Thinking indicator (no reasoning yet) ──── */}
            {isStreaming && !streamingReasoning && !streamingContent && (
              <div className="flex gap-3">
                <div className="shrink-0 mt-0.5">
                  <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
                    <BotIcon className="w-4 h-4 text-primary" />
                  </div>
                </div>
                <div className="bg-muted/50 rounded-xl rounded-tl-sm px-4 py-3 border border-border/50">
                  <div className="flex gap-1.5">
                    <span className="w-2 h-2 bg-muted-foreground/40 rounded-full animate-bounce" style={{ animationDelay: "0ms" }} />
                    <span className="w-2 h-2 bg-muted-foreground/40 rounded-full animate-bounce" style={{ animationDelay: "150ms" }} />
                    <span className="w-2 h-2 bg-muted-foreground/40 rounded-full animate-bounce" style={{ animationDelay: "300ms" }} />
                  </div>
                </div>
              </div>
            )}

            {/* ── Streaming: Content ───────────────────────────────── */}
            {isStreaming && streamingContent && (
              <MessageBubble
                message={{
                  id: "streaming",
                  role: "assistant",
                  content: streamingContent,
                  timestamp: Date.now(),
                }}
                avatarUrl={undefined}
              />
            )}

            <div ref={messagesEndRef} />
          </div>
        )}
      </div>

      {/* ── Input ───────────────────────────────────────────── */}
      <div className="shrink-0 border-t border-border px-4 py-3">
        <div className="max-w-3xl mx-auto">
          {/* Selected knowledge bases chips */}
          {selectedKBs.length > 0 && (
            <div className="flex flex-wrap gap-1.5 mb-2">
              {selectedKBs.map((kb) => (
                <Badge key={kb.name} variant="secondary" className="gap-1 text-xs py-0.5 pr-1">
                  <BookOpenIcon className="w-3 h-3" />
                  <span className="truncate max-w-[120px]">{kb.title}</span>
                  <button
                    className="ml-0.5 hover:text-foreground transition-colors"
                    onClick={() => toggleKnowledgeBase(kb.name)}
                  >
                    <XIcon className="w-3 h-3" />
                  </button>
                </Badge>
              ))}
            </div>
          )}
          <div className="flex items-end gap-2">
            <Textarea
              ref={textareaRef}
              value={input}
              onChange={handleInputChange}
              onKeyDown={handleKeyDown}
              placeholder={t("ai-chat.input-placeholder")}
              className={cn(
                "min-h-[40px] max-h-[200px] resize-none rounded-xl border-border bg-muted/20 px-4 py-2.5 text-sm leading-relaxed",
                "focus-visible:ring-1 focus-visible:ring-ring focus-visible:border-ring/50",
                "transition-colors placeholder:text-muted-foreground/60",
              )}
              rows={1}
            />
            <Button
              size="icon"
              className="shrink-0 h-10 w-10 rounded-xl"
              disabled={!input.trim() || isStreaming}
              onClick={handleSend}
            >
              <SendIcon className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ChatView;
