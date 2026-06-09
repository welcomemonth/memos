import { useState, useEffect, useRef, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowLeftIcon, CopyIcon, RefreshCwIcon, ClockIcon } from "lucide-react";
import toast from "react-hot-toast";
import copy from "copy-to-clipboard";
import MobileHeader from "@/components/MobileHeader";
import { Button } from "@/components/ui/button";
import useMediaQuery from "@/hooks/useMediaQuery";
import { useTranslate } from "@/utils/i18n";

// —— 常用时区列表 ——
const TIMEZONES: { label: string; value: string }[] = [
  { label: "UTC", value: "UTC" },
  { label: "UTC+8 (北京/上海/香港)", value: "Asia/Shanghai" },
  { label: "UTC+9 (东京/首尔)", value: "Asia/Tokyo" },
  { label: "UTC+5:30 (印度)", value: "Asia/Kolkata" },
  { label: "UTC+3 (莫斯科)", value: "Europe/Moscow" },
  { label: "UTC+1 (巴黎/柏林)", value: "Europe/Paris" },
  { label: "UTC+0 (伦敦)", value: "Europe/London" },
  { label: "UTC-5 (纽约)", value: "America/New_York" },
  { label: "UTC-6 (芝加哥)", value: "America/Chicago" },
  { label: "UTC-8 (洛杉矶)", value: "America/Los_Angeles" },
  { label: "UTC+10 (悉尼)", value: "Australia/Sydney" },
  { label: "UTC+12 (奥克兰)", value: "Pacific/Auckland" },
];

type TimestampUnit = "s" | "ms";

// —— 工具函数 ——

/** 获取时区在当前时刻的 UTC 偏移量（分钟） */
function getTimezoneOffset(timeZone: string, date: Date = new Date()): number {
  try {
    const parts = new Intl.DateTimeFormat("en-US", {
      timeZone,
      timeZoneName: "longOffset",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).formatToParts(date);

    const offsetStr = parts.find((p) => p.type === "timeZoneName")?.value || "GMT";
    const match = offsetStr.match(/GMT([+-]\d{1,2})(?::(\d{2}))?/);
    if (!match) return 0;
    const hours = parseInt(match[1], 10);
    const minutes = parseInt(match[2] || "0", 10);
    return hours >= 0 ? hours * 60 + minutes : hours * 60 - minutes;
  } catch {
    return 0;
  }
}

/** 时间戳 → 格式化日期时间 */
function timestampToDatetime(ts: number, timeZone: string, unit: TimestampUnit): string {
  const ms = unit === "s" ? ts * 1000 : ts;
  return new Intl.DateTimeFormat("sv-SE", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(ms));
}

/** 日期时间 → 时间戳（毫秒） */
function datetimeToTimestamp(dateTimeLocalStr: string, timeZone: string): number {
  const [datePart, timePart] = dateTimeLocalStr.split("T");
  const [year, month, day] = datePart.split("-").map(Number);
  const [hour, minute] = timePart.split(":").map(Number);

  // 将用户输入当作 UTC 时间算出毫秒值
  const asUTC = Date.UTC(year, month - 1, day, hour, minute, 0);
  // 减去目标时区的偏移量，得到真正的 UTC 时间戳
  const offsetMinutes = getTimezoneOffset(timeZone, new Date(asUTC));
  return asUTC - offsetMinutes * 60000;
}

const TimestampTool = () => {
  const t = useTranslate();
  const md = useMediaQuery("md");
  const navigate = useNavigate();

  const bt = useCallback((key: string) => t(`timestamp.${key}` as any), [t]);

  // 当前时间
  const [now, setNow] = useState(Date.now());
  const userTimezone = useRef(
    Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
  ).current;

  // 时间戳 → 日期时间
  const [tsInput, setTsInput] = useState("");
  const [tsUnit, setTsUnit] = useState<TimestampUnit>("s");
  const [tsTimezone, setTsTimezone] = useState(userTimezone);
  const [tsResult, setTsResult] = useState("");

  // 日期时间 → 时间戳
  const [dtInput, setDtInput] = useState("");
  const [dtTimezone, setDtTimezone] = useState(userTimezone);
  const [dtUnit, setDtUnit] = useState<TimestampUnit>("s");
  const [dtResult, setDtResult] = useState("");

  // —— 实时时钟 ——
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const nowSeconds = Math.floor(now / 1000);
  const nowMs = now;
  const nowFormatted = timestampToDatetime(nowMs, userTimezone, "ms");

  // —— 复制 ——
  const copyNow = (val: string | number) => {
    copy(String(val));
    toast.success(bt("copied"));
  };

  // —— 时间戳 → 日期时间 ——
  const handleTsToDt = () => {
    const val = tsInput.trim();
    if (!val || !/^\d+$/.test(val)) {
      toast.error(bt("invalid-timestamp"));
      return;
    }
    try {
      const ts = parseInt(val, 10);
      setTsResult(timestampToDatetime(ts, tsTimezone, tsUnit));
    } catch {
      toast.error(bt("conversion-failed"));
    }
  };

  // —— 日期时间 → 时间戳 ——
  const handleDtToTs = () => {
    if (!dtInput) {
      toast.error(bt("please-select-datetime"));
      return;
    }
    try {
      const ts = datetimeToTimestamp(dtInput, dtTimezone);
      setDtResult(String(dtUnit === "s" ? Math.floor(ts / 1000) : ts));
    } catch {
      toast.error(bt("conversion-failed"));
    }
  };

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8 gap-6">
      {!md && <MobileHeader />}

      {/* ========= Card 1: 工具主体 ========= */}
      <div className="w-full px-4 sm:px-6">
        <div className="w-full border border-border flex flex-col justify-start items-start rounded-xl bg-background text-foreground overflow-hidden shadow-sm">
          {/* 标题栏 */}
          <div className="w-full px-4 py-4 border-b border-border bg-muted/20">
            <div className="w-full flex flex-row items-center gap-2">
              <button
                onClick={() => navigate(-1)}
                className="inline-flex items-center justify-center rounded-md hover:bg-muted transition-colors p-1 shrink-0"
                aria-label={bt("back")}
              >
                <ArrowLeftIcon className="w-5 h-auto text-muted-foreground" />
              </button>
              <h1 className="text-xl font-semibold truncate">{bt("title")}</h1>
            </div>
          </div>

          <div className="w-full px-4 sm:px-6 py-6 flex flex-col gap-8">
            {/* ===== 当前时间戳 ===== */}
            <div className="rounded-lg border border-border bg-muted/10 p-5">
              <div className="flex flex-row items-center gap-2 mb-4">
                <ClockIcon className="w-4 h-4 text-muted-foreground" />
                <h3 className="text-sm font-medium text-muted-foreground uppercase tracking-wide">
                  {bt("current-timestamp")}
                </h3>
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                {/* 秒 */}
                <div className="flex flex-col gap-1.5">
                  <span className="text-xs text-muted-foreground">{bt("seconds")}</span>
                  <div className="flex flex-row items-center gap-2">
                    <code className="flex-1 text-2xl font-mono font-bold text-foreground tracking-tight tabular-nums">
                      {nowSeconds.toLocaleString()}
                    </code>
                    <button
                      onClick={() => copyNow(nowSeconds)}
                      className="shrink-0 p-1.5 rounded-md hover:bg-muted transition-colors"
                      title={bt("copy")}
                    >
                      <CopyIcon className="w-4 h-4 text-muted-foreground" />
                    </button>
                  </div>
                </div>

                {/* 毫秒 */}
                <div className="flex flex-col gap-1.5 sm:border-l sm:border-border sm:pl-4">
                  <span className="text-xs text-muted-foreground">{bt("milliseconds")}</span>
                  <div className="flex flex-row items-center gap-2">
                    <code className="flex-1 text-2xl font-mono font-bold text-foreground tracking-tight tabular-nums">
                      {nowMs.toLocaleString()}
                    </code>
                    <button
                      onClick={() => copyNow(nowMs)}
                      className="shrink-0 p-1.5 rounded-md hover:bg-muted transition-colors"
                      title={bt("copy")}
                    >
                      <CopyIcon className="w-4 h-4 text-muted-foreground" />
                    </button>
                  </div>
                </div>
              </div>

              {/* 格式化显示 */}
              <div className="mt-4 pt-4 border-t border-border flex flex-row items-center gap-2 text-sm text-muted-foreground">
                <span>{nowFormatted}</span>
                <span className="text-xs opacity-60">({userTimezone})</span>
              </div>
            </div>

            {/* ===== 时间戳 → 日期时间 ===== */}
            <div className="flex flex-col gap-3">
              <h3 className="text-sm font-medium text-foreground">{bt("timestamp-to-datetime")}</h3>
              <div className="flex flex-row gap-2 flex-wrap">
                <input
                  type="text"
                  value={tsInput}
                  onChange={(e) => setTsInput(e.target.value.replace(/\D/g, ""))}
                  placeholder={bt("enter-timestamp")}
                  className="flex-1 min-w-[180px] rounded-lg border border-border bg-background px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-ring/50"
                  onKeyDown={(e) => e.key === "Enter" && handleTsToDt()}
                />
                <select
                  value={tsUnit}
                  onChange={(e) => setTsUnit(e.target.value as TimestampUnit)}
                  className="rounded-lg border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50"
                >
                  <option value="s">{bt("seconds")}</option>
                  <option value="ms">{bt("milliseconds")}</option>
                </select>
                <select
                  value={tsTimezone}
                  onChange={(e) => setTsTimezone(e.target.value)}
                  className="rounded-lg border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50 max-w-[200px] truncate"
                >
                  {TIMEZONES.map((tz) => (
                    <option key={tz.value} value={tz.value}>
                      {tz.label}
                    </option>
                  ))}
                </select>
                <Button onClick={handleTsToDt} size="sm">
                  <RefreshCwIcon className="w-4 h-4 mr-1" />
                  {bt("convert")}
                </Button>
              </div>
              {tsResult && (
                <div className="flex flex-row items-center gap-2 rounded-lg border border-border bg-muted/20 px-3 py-2.5">
                  <code className="flex-1 text-sm font-mono text-foreground">{tsResult}</code>
                  <button
                    onClick={() => { copy(tsResult); toast.success(bt("copied")); }}
                    className="shrink-0 p-1 rounded hover:bg-muted transition-colors"
                  >
                    <CopyIcon className="w-3.5 h-3.5 text-muted-foreground" />
                  </button>
                </div>
              )}
            </div>

            {/* ===== 分隔 ===== */}
            <div className="border-t border-border" />

            {/* ===== 日期时间 → 时间戳 ===== */}
            <div className="flex flex-col gap-3">
              <h3 className="text-sm font-medium text-foreground">{bt("datetime-to-timestamp")}</h3>
              <div className="flex flex-row gap-2 flex-wrap">
                <input
                  type="datetime-local"
                  value={dtInput}
                  onChange={(e) => setDtInput(e.target.value)}
                  className="flex-1 min-w-[200px] rounded-lg border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50"
                  step="1"
                />
                <select
                  value={dtTimezone}
                  onChange={(e) => setDtTimezone(e.target.value)}
                  className="rounded-lg border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50 max-w-[200px] truncate"
                >
                  {TIMEZONES.map((tz) => (
                    <option key={tz.value} value={tz.value}>
                      {tz.label}
                    </option>
                  ))}
                </select>
                <select
                  value={dtUnit}
                  onChange={(e) => setDtUnit(e.target.value as TimestampUnit)}
                  className="rounded-lg border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring/50"
                >
                  <option value="s">{bt("seconds")}</option>
                  <option value="ms">{bt("milliseconds")}</option>
                </select>
                <Button onClick={handleDtToTs} size="sm">
                  <RefreshCwIcon className="w-4 h-4 mr-1" />
                  {bt("convert")}
                </Button>
              </div>
              {dtResult && (
                <div className="flex flex-row items-center gap-2 rounded-lg border border-border bg-muted/20 px-3 py-2.5">
                  <code className="flex-1 text-sm font-mono text-foreground">{dtResult}</code>
                  <button
                    onClick={() => { copy(dtResult); toast.success(bt("copied")); }}
                    className="shrink-0 p-1 rounded hover:bg-muted transition-colors"
                  >
                    <CopyIcon className="w-3.5 h-3.5 text-muted-foreground" />
                  </button>
                </div>
              )}
            </div>
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
              <h3 className="text-base font-semibold text-foreground mb-3">{bt("principle-issue-title")}</h3>
              <p className="whitespace-pre-line">{bt("principle-issue-desc")}</p>
            </section>
          </article>
        </div>
      </div>
    </section>
  );
};

export default TimestampTool;
