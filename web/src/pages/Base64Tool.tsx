import { useState, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowLeftIcon, CopyIcon, Trash2Icon, ArrowUpDownIcon } from "lucide-react";
import toast from "react-hot-toast";
import copy from "copy-to-clipboard";
import MobileHeader from "@/components/MobileHeader";
import { Button } from "@/components/ui/button";
import useMediaQuery from "@/hooks/useMediaQuery";
import { useTranslate } from "@/utils/i18n";

type Mode = "encode" | "decode";

const Base64Tool = () => {
  const t = useTranslate();
  const md = useMediaQuery("md");
  const navigate = useNavigate();

  const [mode, setMode] = useState<Mode>("encode");
  const [input, setInput] = useState("");
  const [output, setOutput] = useState("");
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const bt = (key: string) => t(`base64.${key}` as any);

  // Encode: text → Base64
  const handleEncode = () => {
    if (!input.trim()) {
      toast.error(bt("empty-input"));
      return;
    }
    try {
      const encoded = btoa(unescape(encodeURIComponent(input)));
      setOutput(encoded);
    } catch {
      toast.error(bt("encode-failed"));
    }
  };

  // Decode: Base64 → text
  const handleDecode = () => {
    if (!input.trim()) {
      toast.error(bt("empty-input"));
      return;
    }
    try {
      const decoded = decodeURIComponent(escape(atob(input.trim())));
      setOutput(decoded);
    } catch {
      toast.error(bt("decode-failed"));
    }
  };

  const handleConvert = () => {
    if (mode === "encode") {
      handleEncode();
    } else {
      handleDecode();
    }
  };

  const handleCopy = () => {
    if (!output) {
      toast.error(bt("empty-output"));
      return;
    }
    copy(output);
    toast.success(bt("copied"));
  };

  const handleClear = () => {
    setInput("");
    setOutput("");
    inputRef.current?.focus();
  };

  const handleSwapMode = () => {
    setMode((prev) => (prev === "encode" ? "decode" : "encode"));
    // Swap input and output
    setInput(output);
    setOutput("");
  };

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full px-4 sm:px-6">
        <div className="w-full border border-border flex flex-col justify-start items-start rounded-xl bg-background text-foreground overflow-hidden">
          {/* 页面标题 */}
          <div className="w-full px-4 py-4 border-b border-border">
            <div className="w-full flex flex-row items-center justify-between">
              <div className="flex flex-row items-center gap-2">
                <button
                  onClick={() => navigate(-1)}
                  className="inline-flex items-center justify-center rounded-md hover:bg-muted transition-colors p-1"
                  aria-label={bt("back")}
                >
                  <ArrowLeftIcon className="w-5 h-auto text-muted-foreground" />
                </button>
                <h1 className="text-xl font-semibold">{bt("title")}</h1>
              </div>
              {/* 模式切换 */}
              <div className="flex flex-row items-center border border-border rounded-md overflow-hidden">
                <button
                  onClick={() => setMode("encode")}
                  className={`px-3 py-1.5 text-sm transition-colors cursor-pointer ${
                    mode === "encode"
                      ? "bg-foreground text-background"
                      : "bg-transparent text-muted-foreground hover:bg-muted"
                  }`}
                >
                  {bt("encode")}
                </button>
                <button
                  onClick={() => setMode("decode")}
                  className={`px-3 py-1.5 text-sm transition-colors cursor-pointer ${
                    mode === "decode"
                      ? "bg-foreground text-background"
                      : "bg-transparent text-muted-foreground hover:bg-muted"
                  }`}
                >
                  {bt("decode")}
                </button>
              </div>
            </div>
          </div>

          {/* 内容区域 */}
          <div className="w-full px-4 py-6 flex flex-col gap-4">
            {/* 输入区域 */}
            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium text-foreground">
                {mode === "encode" ? bt("input-text") : bt("input-base64")}
              </label>
              <textarea
                ref={inputRef}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder={mode === "encode" ? bt("input-placeholder-encode") : bt("input-placeholder-decode")}
                className="w-full min-h-[120px] rounded-md border border-border bg-background px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-ring resize-y"
                onKeyDown={(e) => {
                  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                    e.preventDefault();
                    handleConvert();
                  }
                }}
              />
            </div>

            {/* 操作按钮 */}
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
            </div>

            {/* 输出区域 */}
            <div className="flex flex-col gap-2">
              <label className="text-sm font-medium text-foreground">
                {mode === "encode" ? bt("output-base64") : bt("output-text")}
              </label>
              <textarea
                value={output}
                readOnly
                placeholder={mode === "encode" ? bt("output-placeholder-encode") : bt("output-placeholder-decode")}
                className="w-full min-h-[120px] rounded-md border border-border bg-muted/40 px-3 py-2 text-sm font-mono focus:outline-none resize-y"
              />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Base64Tool;
