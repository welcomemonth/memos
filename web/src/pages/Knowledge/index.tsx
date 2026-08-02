import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  ArrowLeftIcon,
  BookOpenIcon,
  FileIcon,
  LoaderIcon,
  PlusIcon,
  TrashIcon,
  UploadIcon,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Routes } from "@/router";
import {
  createKnowledgeBase,
  deleteDocument,
  deleteKnowledgeBase,
  listDocuments,
  listKnowledgeBases,
  uploadDocument,
  type Document,
  type KnowledgeBase,
} from "@/lib/knowledge";
import { Document_Status } from "@/types/proto/api/v1/knowledge_service_pb";
import { useTranslate } from "@/utils/i18n";

const Knowledge = () => {
  const t = useTranslate();
  const navigate = useNavigate();
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>([]);
  const [selectedKB, setSelectedKB] = useState<KnowledgeBase | null>(null);
  const [documents, setDocuments] = useState<Document[]>([]);
  const [loadingKBs, setLoadingKBs] = useState(true);
  const [loadingDocs, setLoadingDocs] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Create KB form state
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [formTitle, setFormTitle] = useState("");
  const [formDescription, setFormDescription] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Load knowledge bases
  const loadKnowledgeBases = useCallback(async () => {
    try {
      setLoadingKBs(true);
      const list = await listKnowledgeBases();
      setKnowledgeBases(list);
    } catch (err) {
      console.error("Failed to load knowledge bases:", err);
    } finally {
      setLoadingKBs(false);
    }
  }, []);

  // Load documents for selected KB
  const loadDocuments = useCallback(async (kbName: string) => {
    try {
      setLoadingDocs(true);
      const docs = await listDocuments(kbName);
      setDocuments(docs);
    } catch (err) {
      console.error("Failed to load documents:", err);
    } finally {
      setLoadingDocs(false);
    }
  }, []);

  useEffect(() => {
    loadKnowledgeBases();
  }, [loadKnowledgeBases]);

  useEffect(() => {
    if (selectedKB) {
      loadDocuments(selectedKB.name);
    } else {
      setDocuments([]);
    }
  }, [selectedKB, loadDocuments]);

  const handleSelectKB = (kb: KnowledgeBase) => {
    setSelectedKB(kb);
    setShowCreateForm(false);
  };

  const handleCreateKB = async () => {
    if (!formTitle.trim()) return;
    setSubmitting(true);
    try {
      const kb = await createKnowledgeBase(formTitle.trim(), formDescription.trim());
      setKnowledgeBases((prev) => [...prev, kb]);
      setSelectedKB(kb);
      setShowCreateForm(false);
      setFormTitle("");
      setFormDescription("");
    } catch (err) {
      console.error("Failed to create knowledge base:", err);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteKB = async (kb: KnowledgeBase) => {
    if (!confirm(t("ai-chat.kb-delete-confirm"))) return;
    try {
      await deleteKnowledgeBase(kb.name);
      setKnowledgeBases((prev) => prev.filter((k) => k.name !== kb.name));
      if (selectedKB?.name === kb.name) {
        setSelectedKB(null);
      }
    } catch (err) {
      console.error("Failed to delete knowledge base:", err);
    }
  };

  const handleUploadFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !selectedKB) return;
    try {
      const buffer = await file.arrayBuffer();
      await uploadDocument(selectedKB.name, file.name, new Uint8Array(buffer));
      loadDocuments(selectedKB.name);
      // Refresh KB list to update document count
      loadKnowledgeBases();
    } catch (err) {
      console.error("Failed to upload document:", err);
    }
    // Reset file input
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleDeleteDoc = async (doc: Document) => {
    if (!confirm("确定要删除这个文档吗？")) return;
    try {
      await deleteDocument(doc.name);
      if (selectedKB) {
        loadDocuments(selectedKB.name);
        loadKnowledgeBases();
      }
    } catch (err) {
      console.error("Failed to delete document:", err);
    }
  };

  const getStatusBadge = (status: Document_Status) => {
    switch (status) {
      case Document_Status.READY:
        return <Badge variant="default" className="text-[10px]">就绪</Badge>;
      case Document_Status.PROCESSING:
        return (
          <Badge variant="secondary" className="text-[10px] gap-1">
            <LoaderIcon className="w-3 h-3 animate-spin" />
            处理中
          </Badge>
        );
      case Document_Status.PENDING:
        return <Badge variant="outline" className="text-[10px]">待处理</Badge>;
      case Document_Status.FAILED:
        return <Badge variant="destructive" className="text-[10px]">失败</Badge>;
      default:
        return <Badge variant="outline" className="text-[10px]">未知</Badge>;
    }
  };

  const formatFileSize = (bytes: bigint) => {
    const num = Number(bytes);
    if (num < 1024) return `${num} B`;
    if (num < 1024 * 1024) return `${(num / 1024).toFixed(1)} KB`;
    return `${(num / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <section className="w-full h-svh flex overflow-hidden bg-background">
      {/* ── Left Sidebar: Knowledge Base List ─────────────── */}
      <aside className="w-64 shrink-0 h-full flex flex-col border-r border-border bg-sidebar">
        {/* Header */}
        <div className="px-3 pt-4 pb-2">
          <Button
            variant="ghost"
            size="sm"
            className="w-full justify-start gap-2 text-muted-foreground mb-2"
            onClick={() => navigate(Routes.AI_CHAT)}
          >
            <ArrowLeftIcon className="w-4 h-4" />
            返回对话
          </Button>
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-sm font-semibold text-foreground flex items-center gap-2">
              <BookOpenIcon className="w-4 h-4 text-primary" />
              {t("ai-chat.kb-title")}
            </h2>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="w-full gap-1.5 text-xs h-8"
            onClick={() => {
              setSelectedKB(null);
              setShowCreateForm(true);
            }}
          >
            <PlusIcon className="w-3.5 h-3.5" />
            {t("ai-chat.kb-create")}
          </Button>
        </div>

        {/* KB List */}
        <div className="flex-1 overflow-y-auto px-2 pb-2">
          {loadingKBs ? (
            <div className="flex items-center justify-center py-8">
              <LoaderIcon className="w-5 h-5 animate-spin text-muted-foreground" />
            </div>
          ) : knowledgeBases.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
              <BookOpenIcon className="w-8 h-8 mb-2 opacity-30" />
              <p className="text-xs">{t("ai-chat.kb-no-items")}</p>
            </div>
          ) : (
            knowledgeBases.map((kb) => (
              <div
                key={kb.name}
                className={`w-full text-left px-2.5 py-2 rounded-lg text-sm transition-colors mb-0.5 flex items-center gap-2 group/kb ${
                  selectedKB?.name === kb.name
                    ? "bg-sidebar-accent text-sidebar-accent-foreground"
                    : "text-sidebar-foreground/70 hover:bg-sidebar-accent/40 hover:text-sidebar-foreground"
                }`}
              >
                <button
                  className="flex-1 flex items-center gap-2 min-w-0 truncate"
                  onClick={() => handleSelectKB(kb)}
                >
                  <BookOpenIcon className="w-3.5 h-3.5 shrink-0 opacity-60" />
                  <span className="truncate flex-1 text-left">{kb.title}</span>
                  <span className="text-[10px] text-muted-foreground">{kb.documentCount}</span>
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleDeleteKB(kb);
                  }}
                  className="shrink-0 opacity-0 group-hover/kb:opacity-60 hover:!opacity-100 transition-opacity"
                >
                  <TrashIcon className="w-3.5 h-3.5" />
                </button>
              </div>
            ))
          )}
        </div>
      </aside>

      {/* ── Right Content Area ────────────────────────────── */}
      <div className="flex-1 flex flex-col min-h-0 bg-background">
        {/* Create KB Form */}
        {showCreateForm && !selectedKB && (
          <>
            <header className="shrink-0 px-6 py-4 border-b border-border">
              <h2 className="text-lg font-semibold text-foreground">{t("ai-chat.kb-create")}</h2>
            </header>
            <div className="flex-1 overflow-y-auto px-6 py-6">
              <div className="max-w-md mx-auto space-y-4">
                <div className="flex flex-col gap-2">
                  <label className="text-sm font-medium text-foreground">{t("ai-chat.kb-name")}</label>
                  <Input
                    value={formTitle}
                    onChange={(e) => setFormTitle(e.target.value)}
                    placeholder={t("ai-chat.kb-name-placeholder")}
                  />
                </div>
                <div className="flex flex-col gap-2">
                  <label className="text-sm font-medium text-foreground">{t("ai-chat.kb-description")}</label>
                  <Textarea
                    value={formDescription}
                    onChange={(e) => setFormDescription(e.target.value)}
                    placeholder={t("ai-chat.kb-description-placeholder")}
                    rows={3}
                  />
                </div>
                <div className="flex gap-2 pt-2">
                  <Button
                    variant="outline"
                    onClick={() => {
                      setShowCreateForm(false);
                      setFormTitle("");
                      setFormDescription("");
                    }}
                  >
                    取消
                  </Button>
                  <Button onClick={handleCreateKB} disabled={!formTitle.trim() || submitting}>
                    {submitting ? "创建中..." : "创建"}
                  </Button>
                </div>
              </div>
            </div>
          </>
        )}

        {/* Document List for Selected KB */}
        {!showCreateForm && selectedKB && (
          <>
            <header className="shrink-0 px-6 py-4 border-b border-border flex items-center justify-between">
              <div>
                <h2 className="text-lg font-semibold text-foreground">{selectedKB.title}</h2>
                {selectedKB.description && (
                  <p className="text-sm text-muted-foreground mt-0.5">{selectedKB.description}</p>
                )}
              </div>
              <Button
                size="sm"
                className="gap-1.5"
                onClick={() => fileInputRef.current?.click()}
              >
                <UploadIcon className="w-4 h-4" />
                上传文档
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                className="hidden"
                onChange={handleUploadFile}
              />
            </header>
            <div className="flex-1 overflow-y-auto px-6 py-4">
              {loadingDocs ? (
                <div className="flex items-center justify-center py-12">
                  <LoaderIcon className="w-5 h-5 animate-spin text-muted-foreground" />
                </div>
              ) : documents.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-16 text-center">
                  <div className="w-16 h-16 rounded-2xl bg-muted/60 flex items-center justify-center mb-4">
                    <FileIcon className="w-8 h-8 text-muted-foreground" />
                  </div>
                  <h3 className="text-sm font-medium text-foreground mb-1">暂无文档</h3>
                  <p className="text-xs text-muted-foreground mb-4">上传文档开始构建知识库</p>
                  <Button
                    variant="outline"
                    size="sm"
                    className="gap-1.5"
                    onClick={() => fileInputRef.current?.click()}
                  >
                    <UploadIcon className="w-3.5 h-3.5" />
                    上传文档
                  </Button>
                </div>
              ) : (
                <div className="space-y-2">
                  {documents.map((doc) => (
                    <div
                      key={doc.name}
                      className="flex items-center gap-3 rounded-xl border border-border bg-card p-4 hover:bg-accent/50 transition-colors"
                    >
                      <div className="shrink-0 w-10 h-10 rounded-lg bg-muted flex items-center justify-center">
                        <FileIcon className="w-5 h-5 text-muted-foreground" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-foreground truncate">{doc.filename}</p>
                        <div className="flex items-center gap-2 mt-0.5">
                          <span className="text-[11px] text-muted-foreground">
                            {formatFileSize(doc.fileSize)}
                          </span>
                          {getStatusBadge(doc.status)}
                          {doc.errorMessage && (
                            <span className="text-[11px] text-destructive">{doc.errorMessage}</span>
                          )}
                        </div>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 shrink-0 text-destructive hover:text-destructive"
                        onClick={() => handleDeleteDoc(doc)}
                      >
                        <TrashIcon className="w-3.5 h-3.5" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )}

        {/* Empty State: No KB selected and not creating */}
        {!showCreateForm && !selectedKB && (
          <div className="flex-1 flex flex-col items-center justify-center text-center px-4">
            <div className="w-16 h-16 rounded-2xl bg-primary/10 flex items-center justify-center mb-4">
              <BookOpenIcon className="w-8 h-8 text-primary" />
            </div>
            <h3 className="text-lg font-semibold text-foreground mb-2">{t("ai-chat.kb-title")}</h3>
            <p className="text-sm text-muted-foreground mb-4">
              选择一个知识库查看文档，或创建新的知识库
            </p>
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5"
              onClick={() => setShowCreateForm(true)}
            >
              <PlusIcon className="w-3.5 h-3.5" />
              {t("ai-chat.kb-create")}
            </Button>
          </div>
        )}
      </div>
    </section>
  );
};

export default Knowledge;
