import { useState } from "react";
import { BotIcon, BookOpenIcon, CodeIcon, WrenchIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import UserAvatar from "@/components/UserAvatar";
import { useTranslate } from "@/utils/i18n";
import type { ChatMessage, StreamingStep, ToolCallInfo } from "../types";

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
 * Collapsible reasoning block (used in steps-based rendering).
 */
const ReasoningBlock = ({ content }: { content: string }) => {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className="flex gap-3 mb-2">
      <div className="shrink-0 mt-0.5">
        <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
          <BotIcon className="w-4 h-4 text-primary" />
        </div>
      </div>
      <div className="max-w-[75%]">
        <button
          onClick={() => setExpanded((prev) => !prev)}
          className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors mb-1"
        >
          <BotIcon className="w-3.5 h-3.5" />
          <span>思考过程</span>
          <span className="text-muted-foreground/50">{expanded ? "▼" : "▶"}</span>
        </button>
        {expanded && (
          <div className="text-xs text-muted-foreground/70 whitespace-pre-wrap break-words pl-4 border-l-2 border-border">
            {content}
          </div>
        )}
      </div>
    </div>
  );
};

/**
 * Individual tool call card with result summary.
 */
const ToolCallBlock = ({ toolCall }: { toolCall: ToolCallInfo }) => {
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

  // Extract a short preview from the result
  const resultPreview = toolCall.result
    ? toolCall.result.length > 120
      ? toolCall.result.slice(0, 120) + "…"
      : toolCall.result
    : null;

  let formattedArgs = toolCall.arguments;
  try {
    formattedArgs = JSON.stringify(JSON.parse(toolCall.arguments), null, 2);
  } catch {
    // keep original
  }

  return (
    <div className="flex gap-3 mb-2">
      <div className="shrink-0 mt-0.5">
        <div className="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center">
          <WrenchIcon className="w-4 h-4 text-blue-500" />
        </div>
      </div>
      <div className="max-w-[75%] flex-1 min-w-0">
        <div className="flex items-center gap-2 text-sm mb-0.5">
          <span className="font-medium text-foreground">{toolCall.name}</span>
          {statusIcon(toolCall.status)}
          {toolCall.durationMs != null && toolCall.durationMs > 0 && (
            <span className="text-xs text-muted-foreground">{toolCall.durationMs}ms</span>
          )}
        </div>
        {/* Show result summary by default */}
        {resultPreview && (
          <div className="text-xs text-muted-foreground/80 bg-muted/30 rounded-md px-2.5 py-1.5 mb-1 whitespace-pre-wrap break-all font-mono max-h-20 overflow-y-auto">
            {resultPreview}
          </div>
        )}
        <button
          onClick={() => setExpanded((prev) => !prev)}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors"
        >
          {expanded ? "收起详情 ▼" : "查看详情 ▶"}
        </button>
        {expanded && (
          <div className="mt-1.5 space-y-1.5">
            {toolCall.arguments && (
              <div>
                <div className="text-[11px] text-muted-foreground font-medium mb-0.5">参数</div>
                <pre className="bg-muted/50 rounded-md px-2.5 py-1.5 text-xs font-mono overflow-x-auto whitespace-pre-wrap break-all border border-border/50">
                  {formattedArgs}
                </pre>
              </div>
            )}
            {toolCall.result && (
              <div>
                <div className="text-[11px] text-muted-foreground font-medium mb-0.5">结果</div>
                <pre className="bg-muted/50 rounded-md px-2.5 py-1.5 text-xs font-mono overflow-x-auto whitespace-pre-wrap break-all border border-border/50 max-h-48 overflow-y-auto">
                  {toolCall.result}
                </pre>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

const MessageBubble = ({ message, avatarUrl }: Props) => {
  const isUser = message.role === "user";
  const segments = parseContent(message.content);
  const hasSteps = !isUser && message.steps && message.steps.length > 0;

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

      {/* Content */}
      <div className={cn("max-w-[75%] min-w-0", isUser ? "" : "")}>
        {/* Steps-based rendering: preserves interleaved order of reasoning + tool calls */}
        {hasSteps &&
          message.steps!.map((step: StreamingStep, idx: number) => {
            if (step.type === "reasoning") {
              return <ReasoningBlock key={`r-${idx}`} content={step.content} />;
            }
            if (step.type === "tool_call") {
              return <ToolCallBlock key={`tc-${step.toolCall.id}`} toolCall={step.toolCall} />;
            }
            return null;
          })}

        {/* Fallback: old rendering for messages without steps (e.g., user messages or old data) */}
        {!hasSteps && !isUser && message.reasoning && (
          <div className="mb-2">
            <details className="text-xs">
              <summary className="flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors cursor-pointer">
                <BotIcon className="w-3.5 h-3.5" />
                <span>查看思考过程</span>
              </summary>
              <div className="mt-1.5 text-muted-foreground/70 whitespace-pre-wrap break-words pl-4 border-l-2 border-border">
                {message.reasoning}
              </div>
            </details>
          </div>
        )}

        {/* Fallback: old tool calls rendering for messages without steps */}
        {!hasSteps && !isUser && message.toolCalls && message.toolCalls.length > 0 && (
          <div className="mb-2 text-xs text-muted-foreground">
            <WrenchIcon className="w-3.5 h-3.5 inline mr-1" />
            工具调用 ({message.toolCalls.length})
          </div>
        )}

        {/* Final answer bubble */}
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
