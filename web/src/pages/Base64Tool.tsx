import { useState, useRef, useCallback } from "react";
import {
  CopyIcon,
  Trash2Icon,
  ArrowUpDownIcon,
  ImageIcon,
  UploadIcon,
  FileImageIcon,
  XIcon,
} from "lucide-react";
import toast from "react-hot-toast";
import copy from "copy-to-clipboard";
import MobileHeader from "@/components/MobileHeader";
import BackButton from "@/components/BackButton";
import { Button } from "@/components/ui/button";
import useMediaQuery from "@/hooks/useMediaQuery";
import { useTranslate } from "@/utils/i18n";

// —— 类型 ——
type Mode = "encode" | "decode" | "image";


// —— 图片最大大小 10MB ——
const MAX_IMAGE_SIZE = 10 * 1024 * 1024;

const Base64Tool = () => {
  const t = useTranslate();
  const md = useMediaQuery("md");

  const [mode, setMode] = useState<Mode>("encode");
  const [input, setInput] = useState("");
  const [output, setOutput] = useState("");
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // 图片相关状态
  const [imageBase64, setImageBase64] = useState("");
  const [imagePreview, setImagePreview] = useState("");
  const [imageInfo, setImageInfo] = useState<{
    name: string;
    size: number;
    width: number;
    height: number;
    mimeType: string;
  } | null>(null);
  const [isDragOver, setIsDragOver] = useState(false);

  const bt = useCallback((key: string) => t(`base64.${key}` as any), [t]);

  // —— 编码 ——
  const handleEncode = () => {
    if (!input.trim()) {
      toast.error(bt("empty-input"));
      return;
    }
    try {
      setOutput(btoa(unescape(encodeURIComponent(input))));
    } catch {
      toast.error(bt("encode-failed"));
    }
  };

  // —— 解码 ——
  const handleDecode = () => {
    if (!input.trim()) {
      toast.error(bt("empty-input"));
      return;
    }
    try {
      setOutput(decodeURIComponent(escape(atob(input.trim()))));
    } catch {
      toast.error(bt("decode-failed"));
    }
  };

  const handleConvert = () => {
    if (mode === "encode") handleEncode();
    else if (mode === "decode") handleDecode();
  };

  const handleCopy = () => {
    const textToCopy = mode === "image" ? imageBase64 : output;
    if (!textToCopy) {
      toast.error(bt("empty-output"));
      return;
    }
    copy(textToCopy);
    toast.success(bt("copied"));
  };

  const handleClear = () => {
    setInput("");
    setOutput("");
    setImageBase64("");
    setImagePreview("");
    setImageInfo(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
    inputRef.current?.focus();
  };

  const handleSwapMode = () => {
    setMode((prev) => (prev === "encode" ? "decode" : "encode"));
    setInput(output);
    setOutput("");
  };

  // —— 图片处理 ——
  const processImageFile = (file: File) => {
    if (!file.type.startsWith("image/")) {
      toast.error(bt("not-an-image"));
      return;
    }
    if (file.size > MAX_IMAGE_SIZE) {
      toast.error(bt("image-too-large"));
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      setImageBase64(result);

      const img = new Image();
      img.onload = () => {
        setImagePreview(result);
        setImageInfo({
          name: file.name,
          size: file.size,
          width: img.naturalWidth,
          height: img.naturalHeight,
          mimeType: file.type,
        });
      };
      img.src = result;
    };
    reader.onerror = () => toast.error(bt("image-read-failed"));
    reader.readAsDataURL(file);
  };

  const handleImageSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) processImageFile(file);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragOver(false);
    const file = e.dataTransfer.files?.[0];
    if (file) processImageFile(file);
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8 gap-6">
      {!md && <MobileHeader />}

      {/* ========= Card 1: 工具主体 ========= */}
      <div className="w-full px-4 sm:px-6">
        <div className="w-full border border-border flex flex-col justify-start items-start rounded-xl bg-background text-foreground overflow-hidden shadow-sm">
          {/* 标题栏 */}
          <div className="w-full px-4 py-4 border-b border-border bg-muted/20">
            <div className="w-full flex flex-row items-center justify-between gap-3">
              <div className="flex flex-row items-center gap-2 min-w-0">
                <BackButton /> 
                <h1 className="text-xl font-semibold truncate">{bt("title")}</h1>
              </div>
              {/* 模式切换 */}
              <div className="flex flex-row items-center border border-border rounded-lg overflow-hidden bg-background shrink-0">
                {(["encode", "decode", "image"] as Mode[]).map((m) => (
                  <button
                    key={m}
                    onClick={() => {
                      setMode(m);
                      setOutput("");
                    }}
                    className={`flex flex-row items-center gap-1.5 px-3 py-1.5 text-sm font-medium transition-colors cursor-pointer ${
                      mode === m
                        ? "bg-foreground text-background"
                        : "text-muted-foreground hover:bg-muted"
                    }`}
                  >
                    {m === "image" && <ImageIcon className="w-3.5 h-3.5" />}
                    {bt(m)}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* 内容区域 */}
          <div className="w-full px-4 py-6 flex flex-col gap-4">
            {/* ===== 文本 编码/解码 模式 ===== */}
            {mode !== "image" && (
              <>
                <div className="flex flex-col gap-1.5">
                  <label className="text-sm font-medium text-foreground">
                    {mode === "encode" ? bt("input-text") : bt("input-base64")}
                  </label>
                  <textarea
                    ref={inputRef}
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    placeholder={mode === "encode" ? bt("input-placeholder-encode") : bt("input-placeholder-decode")}
                    className="w-full min-h-[140px] rounded-lg border border-border bg-background px-3.5 py-3 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-ring/50 resize-y placeholder:text-muted-foreground/60"
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                        e.preventDefault();
                        handleConvert();
                      }
                    }}
                  />
                </div>

                <div className="flex flex-row items-center gap-2 flex-wrap">
                  <Button onClick={handleConvert}>
                    {mode === "encode" ? bt("encode") : bt("decode")}
                  </Button>
                  <Button variant="outline" size="sm" onClick={handleSwapMode} disabled={!output}>
                    <ArrowUpDownIcon className="w-4 h-4 mr-1" />
                    {bt("swap")}
                  </Button>
                  <Button variant="outline" size="sm" onClick={handleCopy} disabled={!output}>
                    <CopyIcon className="w-4 h-4 mr-1" />
                    {bt("copy")}
                  </Button>
                  <Button variant="outline" size="sm" onClick={handleClear} disabled={!input && !output}>
                    <Trash2Icon className="w-4 h-4 mr-1" />
                    {bt("clear")}
                  </Button>
                  <span className="text-xs text-muted-foreground ml-auto">Ctrl + Enter</span>
                </div>

                <div className="flex flex-col gap-1.5">
                  <label className="text-sm font-medium text-foreground">
                    {mode === "encode" ? bt("output-base64") : bt("output-text")}
                  </label>
                  <textarea
                    value={output}
                    readOnly
                    placeholder={mode === "encode" ? bt("output-placeholder-encode") : bt("output-placeholder-decode")}
                    className="w-full min-h-[140px] rounded-lg border border-border bg-muted/30 px-3.5 py-3 text-sm font-mono focus:outline-none resize-y placeholder:text-muted-foreground/60"
                  />
                </div>
              </>
            )}

            {/* ===== 图片→Base64 模式 ===== */}
            {mode === "image" && (
              <>
                <div
                  onDragOver={handleDragOver}
                  onDragLeave={handleDragLeave}
                  onDrop={handleDrop}
                  onClick={() => fileInputRef.current?.click()}
                  className={`relative w-full min-h-[180px] rounded-lg border-2 border-dashed transition-all cursor-pointer flex flex-col items-center justify-center gap-3 py-10 px-4 ${
                    isDragOver
                      ? "border-foreground bg-muted/40 scale-[1.01]"
                      : imagePreview
                        ? "border-border bg-muted/10"
                        : "border-muted-foreground/30 bg-muted/10 hover:border-muted-foreground/60 hover:bg-muted/20"
                  }`}
                >
                  <input
                    ref={fileInputRef}
                    type="file"
                    accept="image/*"
                    className="hidden"
                    onChange={handleImageSelect}
                  />

                  {imagePreview ? (
                    <div className="flex flex-row items-start gap-4 w-full max-w-lg mx-auto">
                      <div className="shrink-0 w-32 h-32 rounded-lg border border-border overflow-hidden bg-muted/50 flex items-center justify-center">
                        <img
                          src={imagePreview}
                          alt={bt("image-preview")}
                          className="max-w-full max-h-full object-contain"
                        />
                      </div>
                      <div className="flex flex-col gap-1.5 min-w-0 flex-1">
                        <div className="flex flex-row items-center gap-1.5">
                          <FileImageIcon className="w-4 h-4 text-muted-foreground shrink-0" />
                          <span className="text-sm font-medium truncate">{imageInfo?.name}</span>
                        </div>
                        <div className="text-xs text-muted-foreground space-y-0.5">
                          {imageInfo && (
                            <>
                              <p>{formatFileSize(imageInfo.size)} · {imageInfo.width}×{imageInfo.height}px</p>
                              <p>{imageInfo.mimeType}</p>
                            </>
                          )}
                        </div>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            setImageBase64("");
                            setImagePreview("");
                            setImageInfo(null);
                            if (fileInputRef.current) fileInputRef.current.value = "";
                          }}
                          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-red-500 transition-colors mt-1 self-start"
                        >
                          <XIcon className="w-3 h-3" />
                          {bt("remove-image")}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <>
                      <div className="w-12 h-12 rounded-full bg-muted/50 flex items-center justify-center">
                        <UploadIcon className="w-6 h-6 text-muted-foreground" />
                      </div>
                      <div className="text-center">
                        <p className="text-sm font-medium text-foreground">{bt("drop-image-here")}</p>
                        <p className="text-xs text-muted-foreground mt-1">{bt("drop-image-hint")}</p>
                      </div>
                    </>
                  )}
                </div>

                <div className="flex flex-row items-center gap-2 flex-wrap">
                  <Button onClick={() => fileInputRef.current?.click()} variant="outline" size="sm">
                    <UploadIcon className="w-4 h-4 mr-1" />
                    {bt("select-image")}
                  </Button>
                  <Button onClick={handleCopy} disabled={!imageBase64} variant="outline" size="sm">
                    <CopyIcon className="w-4 h-4 mr-1" />
                    {bt("copy")}
                  </Button>
                  <Button onClick={handleClear} disabled={!imageBase64} variant="outline" size="sm">
                    <Trash2Icon className="w-4 h-4 mr-1" />
                    {bt("clear")}
                  </Button>
                  {imageBase64 && (
                    <span className="text-xs text-muted-foreground ml-auto">
                      {bt("base64-length")}: {imageBase64.length.toLocaleString()} chars
                    </span>
                  )}
                </div>

                {imageBase64 && (
                  <div className="flex flex-col gap-1.5">
                    <label className="text-sm font-medium text-foreground">{bt("image-base64-result")}</label>
                    <textarea
                      value={imageBase64}
                      readOnly
                      rows={5}
                      className="w-full rounded-lg border border-border bg-muted/30 px-3.5 py-3 text-xs font-mono focus:outline-none resize-y placeholder:text-muted-foreground/60 overflow-x-auto whitespace-pre"
                    />
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      </div>

      {/* ========= Card 2: 原理讲解 ========= */}
      <div className="w-full px-4 sm:px-6">
        <div className="w-full border border-border rounded-xl bg-background text-foreground overflow-hidden shadow-sm">
          <div className="w-full px-6 py-4 border-b border-border bg-muted/20">
            <h2 className="text-lg font-semibold">{bt("principle-title")}</h2>
          </div>

          <article className="w-full max-w-3xl px-6 py-8 text-sm text-muted-foreground leading-relaxed space-y-6">
            <section>
              <h3 className="text-base font-semibold text-foreground mb-3">{bt("principle-what-title")}</h3>
              <p className="whitespace-pre-line">{bt("principle-what-desc")}</p>
            </section>

            <section>
              <h3 className="text-base font-semibold text-foreground mb-3">{bt("principle-why-title")}</h3>
              <p className="whitespace-pre-line">{bt("principle-why-desc")}</p>
            </section>

            <section>
              <h3 className="text-base font-semibold text-foreground mb-3">{bt("principle-how-title")}</h3>
              <p className="whitespace-pre-line">{bt("principle-how-desc")}</p>
            </section>
          </article>
        </div>
      </div>
    </section>
  );
};

export default Base64Tool;
