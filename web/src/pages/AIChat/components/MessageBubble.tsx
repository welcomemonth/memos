import { useState } from "react";
import { BotIcon, BookOpenIcon, ChevronDownIcon, CodeIcon, WrenchIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import UserAvatar from "@/components/UserAvatar";
import { useTranslate } from "@/utils/i18n";
import type { ChatMessage, ToolCallInfo } from "../types";

interface Props {
  message: ChatMessage;
  avatarUrl?: string;
}

/**
 * Render inline markdown: **bold** → <strong>
 */
const renderInlineMarkdown = (text: string): React.ReactNode => {
  return text.split(/(\*\*.*?\*\*)/g).map((segment, i) => {
    if (segment.startsWith("**") && segment.endsWith("**")) {
      return (
        <strong key={i} className="font-semibold">
          {segment.slice(2, -2)}
        </strong>
      );
    }
    return segment;
  });
};

/**
 * Parse message content into segments: plain text and code blocks.
 */
const parseContent = (content: string): Array<{ type: "text" | "code"; value: string; lang?: string }> => {
  const segments: Array<{ type: "text" | "code"; value: string; lang?: string }> = [];
  const codeBlockRegex = /```(\w*)\n?([\s\S]*?)```/g;
  let lastIndex = 0;
  let match;

  while ((match = codeBlockRegex.exec(content)) !== null) {
    if (match.index > lastIndex) {
      segments.push({ type: "text", value: content.slice(lastIndex, match.index) });
    }
    segments.push({ type: "code", value: match[2].trim(), lang: match[1] || undefined });
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < content.length) {
    segments.push({ type: "text", value: content.slice(lastIndex) });
  }

  return segments;
};

/**
 * Render a code block with language label and "Write Code" action button.
 */
const CodeBlock = ({ code, lang }: { code: string; lang?: string }) => {
  const t = useTranslate();

  return (
    <div className="my-2 rounded-lg overflow-hidden border border-border bg-card">
      {lang && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-muted/40 border-b border-border">
          <div className="flex items-center gap-1.5">
            <CodeIcon className="w-3.5 h-3.5 text-muted-foreground" />
            <span className="text-xs font-mono text-muted-foreground">{lang}</span>
          </div>
          <Button variant="ghost" size="sm" className="h-6 text-xs gap-1 px-2">
            <BookOpenIcon className="w-3 h-3" />
            {t("ai-chat.write-code")}
          </Button>
        </div>
      )}
      <pre className="px-3 py-2.5 overflow-x-auto text-sm font-mono leading-relaxed text-foreground">
        <code>{code}</code>
      </pre>
    </div>
  );
};

/**
 * Collapsible reasoning section for assistant messages.
 */
const ReasoningSection = ({ reasoning }: { reasoning: string }) => {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="mb-2">
      <button
        onClick={() => setExpanded((prev) => !prev)}
        className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
      >
        <BotIcon className="w-3.5 h-3.5" />
        <span>查看思考过程</span>
        <ChevronDownIcon
          className={cn("w-3.5 h-3.5 transition-transform duration-200", expanded && "rotate-180")}
        />
      </button>
      {expanded && (
        <div className="mt-1.5 text-xs text-muted-foreground/70 whitespace-pre-wrap break-words pl-4 border-l-2 border-border">
          {reasoning}
        </div>
      )}
    </div>
  );
};

/**
 * Collapsible tool calls section for assistant messages.
 */
const ToolCallsSection = ({ toolCalls }: { toolCalls: ToolCallInfo[] }) => {
  const [expanded, setExpanded] = useState(false);

  const statusIcon = (status: ToolCallInfo["status"]) => {
    switch (status) {
      case "completed":
        return <span className="text-green-500">✓</span>;
      case "failed":
        return <span className="text-red-500">✗</span>;
      default:
        return <span className="text-blue-500">⟳</span>;
    }
  };

  return (
    <div className="mb-2">
      <button
        onClick={() => setExpanded((prev) => !prev)}
        className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
      >
        <WrenchIcon className="w-3.5 h-3.5" />
        <span>
          工具调用 ({toolCalls.length})
          {toolCalls.every((tc) => tc.status === "completed") ? "" : " …"}
        </span>
        <ChevronDownIcon
          className={cn("w-3.5 h-3.5 transition-transform duration-200", expanded && "rotate-180")}
        />
      </button>
      {expanded && (
        <div className="mt-1.5 space-y-2 pl-4 border-l-2 border-border">
          {toolCalls.map((tc) => (
            <div key={tc.id} className="text-xs">
              <div className="flex items-center gap-1.5 font-medium text-foreground">
                {statusIcon(tc.status)}
                <span>{tc.name}</span>
                {tc.durationMs != null && tc.durationMs > 0 && (
                  <span className="text-muted-foreground font-normal">{tc.durationMs}ms</span>
                )}
              </div>
              {tc.arguments && (
                <pre className="mt-0.5 bg-muted/50 rounded px-2 py-1 font-mono text-[11px] overflow-x-auto whitespace-pre-wrap break-all">
                  {tc.arguments}
                </pre>
              )}
              {tc.result && (
                <pre className="mt-0.5 bg-muted/30 rounded px-2 py-1 font-mono text-[11px] overflow-x-auto whitespace-pre-wrap break-all max-h-32 overflow-y-auto">
                  {tc.result}
                </pre>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

const MessageBubble = ({ message, avatarUrl }: Props) => {
  const isUser = message.role === "user";
  const segments = parseContent(message.content);

  return (
    <div className={cn("flex gap-3 group", isUser ? "flex-row-reverse" : "flex-row")}>
      {/* Avatar */}
      <div className="shrink-0 mt-0.5">
        {isUser ? (
          <UserAvatar avatarUrl={avatarUrl} className="w-8 h-8" />
        ) : (
          <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
            <BotIcon className="w-4 h-4 text-primary" />
          </div>
        )}
      </div>

      {/* Bubble */}
      <div className={cn("max-w-[75%] min-w-0", isUser ? "" : "")}>
        {/* Reasoning (only for assistant messages with reasoning) */}
        {!isUser && message.reasoning && <ReasoningSection reasoning={message.reasoning} />}

        {/* Tool calls (only for assistant messages with tool calls) */}
        {!isUser && message.toolCalls && message.toolCalls.length > 0 && (
          <ToolCallsSection toolCalls={message.toolCalls} />
        )}

        <div
          className={cn(
            "rounded-xl px-4 py-3 text-sm leading-relaxed",
            isUser
              ? "bg-primary text-primary-foreground rounded-tr-sm"
              : "bg-muted/50 text-foreground rounded-tl-sm border border-border/50",
          )}
        >
          {isUser ? (
            <span>{message.content}</span>
          ) : (
            <div className="space-y-1">
              {segments.map((segment, i) =>
                segment.type === "code" ? (
                  <CodeBlock key={i} code={segment.value} lang={segment.lang} />
                ) : (
                  <div key={i} className="whitespace-pre-wrap">
                    {segment.value.split("\n").map((line, lineIdx) => (
                      <span key={lineIdx}>
                        {lineIdx > 0 && <br />}
                        {renderInlineMarkdown(line)}
                      </span>
                    ))}
                  </div>
                ),
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default MessageBubble;
