export type MessageRole = "user" | "assistant";

export type SidebarTab = "chat" | "knowledge" | "agents" | "skills";

export interface ChatMessage {
  id: string;
  role: MessageRole;
  content: string;
  reasoning?: string;
  timestamp: number;
  model?: string;
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
