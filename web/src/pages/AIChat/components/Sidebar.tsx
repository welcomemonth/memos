import { BotIcon, BookOpenIcon, MessageSquareIcon, PlusIcon, ServerIcon, SettingsIcon, SparklesIcon, WrenchIcon, XIcon } from "lucide-react";
import { Link } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Routes } from "@/router";
import { useTranslate } from "@/utils/i18n";
import type { Conversation, SidebarTab, SidebarTabItem } from "../types";

interface Props {
  activeTab: SidebarTab;
  onTabChange: (tab: SidebarTab) => void;
  conversations: Conversation[];
  activeConvId: string | undefined;
  onSelectConversation: (id: string) => void;
  onNewConversation: () => void;
  onDeleteConversation?: (id: string) => void;
}

const groupByDate = (conversations: Conversation[]) => {
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const yesterday = today - 86400000;
  const sevenDaysAgo = today - 7 * 86400000;

  const groups: { label: string; items: Conversation[] }[] = [];
  const todayItems = conversations.filter((c) => c.updatedAt >= today);
  const yesterdayItems = conversations.filter((c) => c.updatedAt >= yesterday && c.updatedAt < today);
  const weekItems = conversations.filter((c) => c.updatedAt >= sevenDaysAgo && c.updatedAt < yesterday);
  const olderItems = conversations.filter((c) => c.updatedAt < sevenDaysAgo);

  if (todayItems.length) groups.push({ label: "today", items: todayItems });
  if (yesterdayItems.length) groups.push({ label: "yesterday", items: yesterdayItems });
  if (weekItems.length) groups.push({ label: "week", items: weekItems });
  if (olderItems.length) groups.push({ label: "older", items: olderItems });

  return groups;
};

/**
 * Placeholder for tabs not yet implemented (knowledge, agents, skills).
 */
const ComingSoon = ({ icon, label }: { icon: React.ReactNode; label: string }) => (
  <div className="flex-1 flex flex-col items-center justify-center py-12 text-center">
    <div className="w-14 h-14 rounded-2xl bg-muted/60 flex items-center justify-center mb-4 text-muted-foreground">
      {icon}
    </div>
    <h3 className="text-sm font-medium text-foreground mb-1">{label}</h3>
    <Badge variant="secondary" className="text-xs mt-1">
      Coming Soon
    </Badge>
  </div>
);

const Sidebar = ({
  activeTab,
  onTabChange,
  conversations,
  activeConvId,
  onSelectConversation,
  onNewConversation,
  onDeleteConversation,
}: Props) => {
  const t = useTranslate();

  const tabs: SidebarTabItem[] = [
    { id: "chat", icon: <MessageSquareIcon className="w-4 h-4" />, label: t("ai-chat.tab-chat"), enabled: true },
    { id: "knowledge", icon: <BookOpenIcon className="w-4 h-4" />, label: t("ai-chat.tab-knowledge"), enabled: true },
    { id: "agents", icon: <BotIcon className="w-4 h-4" />, label: t("ai-chat.tab-agents"), enabled: false },
    { id: "skills", icon: <WrenchIcon className="w-4 h-4" />, label: t("ai-chat.tab-skills"), enabled: false },
    { id: "mcp", icon: <ServerIcon className="w-4 h-4" />, label: t("ai-chat.tab-mcp"), enabled: false },
  ];

  const grouped = groupByDate(conversations);

  return (
    <aside className="w-64 shrink-0 h-full flex flex-col border-r border-border bg-sidebar">
      {/* ── Tab Navigation ─────────────────────────────────── */}
      <nav className="px-3 pt-4 pb-2 space-y-0.5">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => tab.enabled && onTabChange(tab.id)}
            className={cn(
              "w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors",
              tab.id === activeTab
                ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                : tab.enabled
                  ? "text-sidebar-foreground/70 hover:bg-sidebar-accent/50 hover:text-sidebar-foreground"
                  : "text-sidebar-foreground/40 cursor-not-allowed",
            )}
          >
            {tab.icon}
            <span className="flex-1 text-left truncate">{tab.label}</span>
            {!tab.enabled && (
              <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-muted text-muted-foreground">
                {t("ai-chat.coming-soon")}
              </span>
            )}
          </button>
        ))}
      </nav>

      <div className="mx-3 my-1 border-t border-border/60" />

      {/* ── Tab Content ────────────────────────────────────── */}
      <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
        {activeTab === "chat" && (
          <>
            {/* New conversation button */}
            <div className="px-3 py-2">
              <Button
                variant="outline"
                size="sm"
                className="w-full gap-1.5 text-xs h-8"
                onClick={onNewConversation}
              >
                <PlusIcon className="w-3.5 h-3.5" />
                {t("ai-chat.new-conversation")}
              </Button>
            </div>

            {/* Conversation list */}
            <div className="flex-1 overflow-y-auto px-2 pb-2">
              {grouped.length === 0 && (
                <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
                  <MessageSquareIcon className="w-8 h-8 mb-2 opacity-30" />
                  <p className="text-xs">{t("ai-chat.no-conversations")}</p>
                </div>
              )}
              {grouped.map((group) => (
                <div key={group.label} className="mb-2">
                  <p className="px-2.5 py-1 text-[11px] font-medium text-muted-foreground uppercase tracking-wide">
                    {t(`ai-chat.date-group.${group.label}` as any)}
                  </p>
                  {group.items.map((conv) => (
                    <div
                      key={conv.id}
                      className={cn(
                        "w-full text-left px-2.5 py-2 rounded-lg text-sm transition-colors mb-0.5 flex items-center gap-2 group/item",
                        conv.id === activeConvId
                          ? "bg-sidebar-accent text-sidebar-accent-foreground"
                          : "text-sidebar-foreground/70 hover:bg-sidebar-accent/40 hover:text-sidebar-foreground",
                      )}
                    >
                      <button
                        className="flex-1 flex items-center gap-2 min-w-0 truncate"
                        onClick={() => onSelectConversation(conv.id)}
                      >
                        <MessageSquareIcon className="w-3.5 h-3.5 shrink-0 opacity-60" />
                        <span className="truncate">{conv.title}</span>
                      </button>
                      {onDeleteConversation && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            onDeleteConversation(conv.id);
                          }}
                          className="shrink-0 opacity-0 group-hover/item:opacity-60 hover:!opacity-100 transition-opacity"
                        >
                          <XIcon className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          </>
        )}

        {activeTab === "agents" && (
          <ComingSoon icon={<SparklesIcon className="w-7 h-7" />} label={t("ai-chat.tab-agents")} />
        )}
        {activeTab === "skills" && (
          <ComingSoon icon={<WrenchIcon className="w-7 h-7" />} label={t("ai-chat.tab-skills")} />
        )}
        {activeTab === "mcp" && (
          <ComingSoon icon={<ServerIcon className="w-7 h-7" />} label={t("ai-chat.tab-mcp")} />
        )}
      </div>

      {/* ── Footer ─────────────────────────────────────────── */}
      <div className="px-3 py-2 border-t border-border/60">
        <Link
          to={Routes.SETTING}
          className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-sidebar-foreground/60 hover:bg-sidebar-accent/40 hover:text-sidebar-foreground transition-colors"
        >
          <SettingsIcon className="w-4 h-4" />
          <span>{t("ai-chat.settings")}</span>
        </Link>
      </div>
    </aside>
  );
};

export default Sidebar;
