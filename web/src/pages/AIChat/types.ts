export type MessageRole = "user" | "assistant";

export type SidebarTab = "chat" | "knowledge" | "agents" | "skills" | "mcp";

export interface ToolCallInfo {
  id: string;
  name: string;
  arguments: string;
  result?: string;
  status: "running" | "completed" | "failed";
  durationMs?: number;
}

/** A single step in the ReAct trace: either reasoning text or a tool call. */
export type StreamingStep =
  | { type: "reasoning"; content: string }
  | { type: "tool_call"; toolCall: ToolCallInfo };

export interface ChatMessage {
  id: string;
  role: MessageRole;
  content: string;
  reasoning?: string;
  timestamp: number;
  model?: string;
  toolCalls?: ToolCallInfo[];
}

export interface Conversation {
  id: string;
  /** Full API name: "ai/conversations/<uid>" */
  name: string;
  title: string;
  model?: string;
  updatedAt: number;
  messages: ChatMessage[];
}

export interface SidebarTabItem {
  id: SidebarTab;
  icon: React.ReactNode;
  label: string;
  enabled: boolean;
}
