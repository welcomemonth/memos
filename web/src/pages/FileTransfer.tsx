import { useState, useEffect, useRef } from "react";
import { ArrowLeftRightIcon } from "lucide-react";
import toast from "react-hot-toast";
import copy from "copy-to-clipboard";
import { usePeerConnection } from "@/hooks/usePeerConnection";
import MobileHeader from "@/components/MobileHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import useMediaQuery from "@/hooks/useMediaQuery";
import { useTranslate } from "@/utils/i18n";
import { useSignaling, type SignalingStatus } from "@/hooks/useSignaling";

type Step = "setup" | "waiting" | "connected";
type Role = "initiator" | "joiner" | null;

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

const FileTransfer = () => {
  const t = useTranslate();
  const md = useMediaQuery("md");

  const signaling = useSignaling();
  const { status: wsStatus, roomCode } = signaling;

  const [step, setStep] = useState<Step>("setup");
  const [role, setRole] = useState<Role>(null);
  const [joinCode, setJoinCode] = useState("");

  const { connectionState, dataChannel, error: pcError } = usePeerConnection(
    step === "connected" ? role : null,
    signaling,
  );

  // —— 文件传输状态 ——
  const [file, setFile] = useState<File | null>(null);
  const [transferStatus, setTransferStatus] = useState<"idle" | "sending" | "receiving" | "complete">("idle");
  const [sentBytes, setSentBytes] = useState(0);
  const [receivedBytes, setReceivedBytes] = useState(0);
  const [fileMeta, setFileMeta] = useState<{ name: string; size: number; mimeType: string } | null>(null);
  const chunksRef = useRef<BlobPart[]>([]);

  // —— 监听 DataChannel 消息（接收方） ——
  useEffect(() => {
    if (!dataChannel) return;

    const dc = dataChannel;

    dc.onmessage = (event: MessageEvent) => {
      if (typeof event.data === "string") {
        try {
          const msg = JSON.parse(event.data);
          if (msg.type === "file-meta") {
            setFileMeta({ name: msg.name, size: msg.size, mimeType: msg.mimeType });
            setTransferStatus("receiving");
            setReceivedBytes(0);
            chunksRef.current = [];
          } else if (msg.type === "file-complete") {
            const type = fileMeta?.mimeType || "application/octet-stream";
            const blob = new Blob(chunksRef.current, { type });
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = fileMeta?.name || "download";
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            setTransferStatus("complete");
            toast.success(t("file-transfer.file-received", { name: fileMeta?.name || "" }));
          }
        } catch {
          // ignore invalid JSON
        }
      } else if (event.data instanceof ArrayBuffer) {
        chunksRef.current.push(event.data);
        setReceivedBytes((prev) => prev + event.data.byteLength);
      }
    };

    return () => {
      dc.onmessage = null;
    };
  }, [dataChannel, fileMeta, t]);

  // —— 发送文件 ——
  const handleSendFile = async () => {
    if (!dataChannel || !file) return;

    const CHUNK_SIZE = 16 * 1024;
    const meta = {
      type: "file-meta",
      name: file.name,
      size: file.size,
      mimeType: file.type || "application/octet-stream",
    };

    setTransferStatus("sending");
    setSentBytes(0);

    try {
      dataChannel.send(JSON.stringify(meta));

      const arrayBuffer = await file.arrayBuffer();
      let offset = 0;

      while (offset < arrayBuffer.byteLength) {
        const end = Math.min(offset + CHUNK_SIZE, arrayBuffer.byteLength);
        dataChannel.send(arrayBuffer.slice(offset, end));
        offset = end;
        setSentBytes(offset);

        while (dataChannel.bufferedAmount > 64 * 1024) {
          await new Promise<void>((resolve) => {
            dataChannel.onbufferedamountlow = () => {
              dataChannel.onbufferedamountlow = null;
              resolve();
            };
          });
        }
      }

      dataChannel.send(JSON.stringify({ type: "file-complete" }));
      setTransferStatus("complete");
      toast.success(t("file-transfer.file-sent"));
    } catch {
      toast.error(t("file-transfer.file-send-failed"));
      setTransferStatus("idle");
    }
  };

  // —— WebSocket 连接 ——
  useEffect(() => {
    signaling.connect();
    return () => {
      signaling.disconnect();
    };
  }, []);

  // —— 注册信令回调 ——
  useEffect(() => {
    signaling.onRoomCreated(() => {
      setStep("waiting");
      setRole("initiator");
    });

    signaling.onRoomJoined(() => {
      setStep("connected");
      setRole("joiner");
    });

    signaling.onPeerJoined(() => {
      setStep("connected");
    });

    signaling.onPeerDisconnected(() => {
      if (step !== "setup") {
        toast.error(t("file-transfer.peer-disconnected"));
        setStep("setup");
        setRole(null);
      }
    });
  }, [signaling, step, t]);

  // —— WebSocket 断线提示 ——
  useEffect(() => {
    if (wsStatus === "disconnected" && step !== "setup") {
      toast.error(t("file-transfer.signaling-disconnected"));
      setStep("setup");
      setRole(null);
    }
  }, [wsStatus, step, t]);

  // —— 创建房间 ——
  const handleCreateRoom = () => {
    signaling.createRoom();
  };

  // —— 加入房间 ——
  const handleJoinRoom = () => {
    const code = joinCode.trim();
    if (code.length !== 6 || !/^\d{6}$/.test(code)) {
      toast.error(t("file-transfer.invalid-room-code"));
      return;
    }
    signaling.joinRoom(code);
  };

  // —— 复制房间码 ——
  const handleCopyCode = () => {
    if (roomCode) {
      copy(roomCode);
      toast.success(t("file-transfer.room-code-copied"));
    }
  };

  // —— 连接状态指示器 ——
  const statusColor: Record<SignalingStatus, string> = {
    idle: "bg-gray-400",
    connecting: "bg-yellow-400",
    connected: "bg-green-400",
    disconnected: "bg-red-400",
  };

  const ft = (key: string) => t(`file-transfer.${key}` as any);

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full px-4 sm:px-6">
        <div className="w-full border border-border flex flex-col justify-start items-start rounded-xl bg-background text-foreground overflow-hidden">
          {/* 页面标题 */}
          <div className="w-full px-4 py-4 border-b border-border">
            <div className="w-full flex flex-row items-center justify-between">
              <div className="flex flex-row items-center gap-2">
                <ArrowLeftRightIcon className="w-5 h-auto text-muted-foreground" />
                <h1 className="text-xl font-semibold">{t("file-transfer.title")}</h1>
              </div>
              <div className="flex flex-row items-center gap-1.5">
                <span className={`w-2 h-2 rounded-full ${statusColor[wsStatus]}`} />
                <span className="text-xs text-muted-foreground">{ft(wsStatus)}</span>
              </div>
            </div>
          </div>

          {/* 内容区域 */}
          <div className="w-full px-4 py-6">
            {step === "setup" && (
              <div className="grid gap-6 sm:grid-cols-2">
                {/* 创建房间 */}
                <div className="rounded-lg border border-border bg-muted/40 p-6 text-center">
                  <h3 className="font-medium text-foreground mb-2">{t("file-transfer.create-room")}</h3>
                  <p className="text-sm text-muted-foreground mb-4">{t("file-transfer.create-room-desc")}</p>
                  <Button onClick={handleCreateRoom} disabled={wsStatus !== "connected"}>
                    {t("file-transfer.create")}
                  </Button>
                </div>

                {/* 加入房间 */}
                <div className="rounded-lg border border-border bg-muted/40 p-6 text-center">
                  <h3 className="font-medium text-foreground mb-2">{t("file-transfer.join-room")}</h3>
                  <p className="text-sm text-muted-foreground mb-4">{t("file-transfer.join-room-desc")}</p>
                  <div className="flex flex-col items-center gap-2">
                    <Input
                      value={joinCode}
                      onChange={(e) => setJoinCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                      placeholder={t("file-transfer.enter-room-code")}
                      className="w-48 text-center text-lg tracking-widest"
                      maxLength={6}
                      onKeyDown={(e) => e.key === "Enter" && handleJoinRoom()}
                    />
                    <Button
                      variant="outline"
                      onClick={handleJoinRoom}
                      disabled={joinCode.length !== 6 || wsStatus !== "connected"}
                    >
                      {t("file-transfer.join")}
                    </Button>
                  </div>
                </div>
              </div>
            )}

            {step === "waiting" && (
              <div className="flex flex-col items-center py-8">
                <div className="w-12 h-12 border-4 border-muted-foreground/30 border-t-muted-foreground rounded-full animate-spin mb-6" />
                <h3 className="text-lg font-medium mb-2">{t("file-transfer.waiting-for-peer")}</h3>
                <p className="text-sm text-muted-foreground mb-4">{t("file-transfer.waiting-for-peer-desc")}</p>
                <div className="flex flex-row items-center gap-3">
                  <span className="text-4xl font-mono font-bold tracking-[0.3em] select-all">
                    {roomCode}
                  </span>
                  <Button variant="outline" size="sm" onClick={handleCopyCode}>
                    {t("file-transfer.copy")}
                  </Button>
                </div>
              </div>
            )}

            {step === "connected" && (
              <div className="flex flex-col items-center py-8">
                {connectionState === "new" || connectionState === "connecting" ? (
                  <>
                    <div className="w-12 h-12 border-4 border-muted-foreground/30 border-t-muted-foreground rounded-full animate-spin mb-6" />
                    <h3 className="text-lg font-medium mb-2">{t("file-transfer.establishing-p2p")}</h3>
                    <p className="text-sm text-muted-foreground">{t("file-transfer.establishing-p2p-desc")}</p>
                  </>
                ) : connectionState === "connected" ? (
                  <>
                    <div className="w-12 h-12 bg-green-100 dark:bg-green-900/30 rounded-full flex items-center justify-center mb-4">
                      <ArrowLeftRightIcon className="w-6 h-6 text-green-600" />
                    </div>
                    <h3 className="text-lg font-medium mb-2">{t("file-transfer.p2p-connected")}</h3>

                    {/* —— 文件传输区域 —— */}
                    {transferStatus === "idle" && (
                      <div className="w-full max-w-md mt-4">
                        {role === "initiator" ? (
                          <div className="flex flex-col items-center gap-3">
                            <label className="w-full cursor-pointer">
                              <div className="border-2 border-dashed border-border rounded-lg p-6 text-center hover:border-muted-foreground transition-colors">
                                {file ? (
                                  <p className="text-sm">
                                    <span className="font-medium">{file.name}</span>
                                    <span className="text-muted-foreground ml-2">
                                      ({formatFileSize(file.size)})
                                    </span>
                                  </p>
                                ) : (
                                  <p className="text-sm text-muted-foreground">{t("file-transfer.click-to-select-file")}</p>
                                )}
                              </div>
                              <input
                                type="file"
                                className="hidden"
                                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                              />
                            </label>
                            <Button onClick={handleSendFile} disabled={!file}>
                              {t("file-transfer.send-file")}
                            </Button>
                          </div>
                        ) : (
                          <p className="text-sm text-muted-foreground">{t("file-transfer.waiting-for-file")}</p>
                        )}
                      </div>
                    )}

                    {/* 发送进度 */}
                    {transferStatus === "sending" && (
                      <div className="w-full max-w-md mt-4">
                        <p className="text-sm text-muted-foreground mb-2">
                          {t("file-transfer.sending-file", { name: file?.name || "" })}
                        </p>
                        <div className="w-full bg-muted rounded-full h-2 mb-1">
                          <div
                            className="bg-blue-500 h-2 rounded-full transition-all duration-300"
                            style={{ width: `${file?.size ? (sentBytes / file.size) * 100 : 0}%` }}
                          />
                        </div>
                        <p className="text-xs text-muted-foreground">
                          {formatFileSize(sentBytes)} / {file ? formatFileSize(file.size) : "..."}
                        </p>
                      </div>
                    )}

                    {/* 接收进度 */}
                    {transferStatus === "receiving" && fileMeta && (
                      <div className="w-full max-w-md mt-4">
                        <p className="text-sm text-muted-foreground mb-2">
                          {t("file-transfer.receiving-file", { name: fileMeta.name })}
                        </p>
                        <div className="w-full bg-muted rounded-full h-2 mb-1">
                          <div
                            className="bg-blue-500 h-2 rounded-full transition-all duration-300"
                            style={{ width: `${(receivedBytes / fileMeta.size) * 100}%` }}
                          />
                        </div>
                        <p className="text-xs text-muted-foreground">
                          {formatFileSize(receivedBytes)} / {formatFileSize(fileMeta.size)}
                        </p>
                      </div>
                    )}

                    {/* 传输完成 */}
                    {transferStatus === "complete" && (
                      <div className="mt-4 flex flex-col items-center gap-3">
                        <p className="text-sm text-green-600 font-medium">{t("file-transfer.transfer-complete")}</p>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setFile(null);
                            setFileMeta(null);
                            setTransferStatus("idle");
                            setSentBytes(0);
                            setReceivedBytes(0);
                          }}
                        >
                          {t("file-transfer.send-new-file")}
                        </Button>
                      </div>
                    )}
                  </>
                ) : connectionState === "failed" ? (
                  <>
                    <div className="w-12 h-12 bg-red-100 dark:bg-red-900/30 rounded-full flex items-center justify-center mb-4">
                      <ArrowLeftRightIcon className="w-6 h-6 text-red-600" />
                    </div>
                    <h3 className="text-lg font-medium mb-2 text-red-600">{t("file-transfer.connection-failed")}</h3>
                    <p className="text-sm text-muted-foreground mb-4">
                      {pcError || t("file-transfer.network-unsupported")}
                    </p>
                    <Button variant="outline" onClick={() => signaling.disconnect()}>
                      {t("file-transfer.retry")}
                    </Button>
                  </>
                ) : (
                  <>
                    <div className="w-12 h-12 bg-yellow-100 dark:bg-yellow-900/30 rounded-full flex items-center justify-center mb-4">
                      <ArrowLeftRightIcon className="w-6 h-6 text-yellow-600" />
                    </div>
                    <h3 className="text-lg font-medium mb-2">{t("file-transfer.connection-error")}</h3>
                    <p className="text-sm text-muted-foreground">{t("file-transfer.status")}: {connectionState}</p>
                  </>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  );
};

export default FileTransfer;
