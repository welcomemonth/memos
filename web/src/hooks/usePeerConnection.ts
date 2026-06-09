import { useState, useEffect, useRef } from "react";
import { useSignaling } from "@/hooks/useSignaling";

export function usePeerConnection(
  role: "initiator" | "joiner" | null,
  signaling: ReturnType<typeof useSignaling>
) {
  const [connectionState, setConnectionState] = useState<RTCPeerConnectionState>("new");
  const [dataChannel, setDataChannel] = useState<RTCDataChannel | null>(null);
  const [error, setError] = useState<string | null>(null);

  const pcRef = useRef<RTCPeerConnection | null>(null);
  const dcRef = useRef<RTCDataChannel | null>(null);

  useEffect(() => {
    // 没有角色就不创建连接
    if (!role) return;

    const config: RTCConfiguration = {
      iceServers: [{ urls: "stun:stun.meetmonth.top:3478" }],
    };

    const pc = new RTCPeerConnection(config);
    pcRef.current = pc;

    // —— 连接状态监听 ——
    pc.onconnectionstatechange = () => {
      setConnectionState(pc.connectionState);
      // STUN 穿透失败 = 网络不支持 P2P
      if (pc.connectionState === "failed") {
        setError("P2P connection failed: network may be behind symmetric NAT or firewall");
      }
    };

    // —— 连接超时（20 秒） ——
    const connectionTimeout = setTimeout(() => {
      if (pc.connectionState !== "connected" && dcRef.current?.readyState !== "open") {
        setError("Connection timed out: P2P may not be supported in this network environment");
        setConnectionState("failed");
      }
    }, 20000);

    // —— ICE 候选转发 ——
    // 每发现一个候选（本机 IP、STUN 映射地址等）就发给对方
    pc.onicecandidate = (event) => {
      if (event.candidate) {
        signaling.sendIceCandidate(event.candidate.toJSON());
      }
    };

    // —— ICE 候选接收 ——
    signaling.onIceCandidate((candidate) => {
      pc.addIceCandidate(new RTCIceCandidate(candidate)).catch(() => {
        // 部分无效候选可忽略
      });
    });

    // ========== 发起方流程 ==========
    if (role === "initiator") {
      // ① 创建数据通道（发起方主动创建）
      const channel = pc.createDataChannel("file-transfer");
      dcRef.current = channel;
      channel.onopen = () => {
        setDataChannel(channel);
      };

      // ②-③ 创建 Offer → 设置本地描述 → 发送
      pc.createOffer()
        .then((offer) => pc.setLocalDescription(offer))
        .then(() => {
          signaling.sendOffer(pc.localDescription!.sdp!);
        })
        .catch((err) => {
          setError(`Failed to create offer: ${err.message}`);
        });

      // ⑧ 接收 Answer → 设置远程描述
      signaling.onAnswer((sdp) => {
        pc.setRemoteDescription(new RTCSessionDescription({ type: "answer", sdp }))
          .catch((err) => {
            setError(`Failed to set remote description: ${err.message}`);
          });
      });
    }

    // ========== 接收方流程 ==========
    if (role === "joiner") {
      // ④ 监听远程 DataChannel
      pc.ondatachannel = (event) => {
        const channel = event.channel;
        dcRef.current = channel;
        channel.onopen = () => {
          setDataChannel(channel);
        };
      };

      // ⑤-⑦ 接收 Offer → 设置远程描述 → 创建 Answer → 设置本地描述 → 发送
      signaling.onOffer((sdp) => {
        pc.setRemoteDescription(new RTCSessionDescription({ type: "offer", sdp }))
          .then(() => pc.createAnswer())
          .then((answer) => pc.setLocalDescription(answer))
          .then(() => {
            signaling.sendAnswer(pc.localDescription!.sdp!);
          })
          .catch((err) => {
            setError(`Failed to handle offer: ${err.message}`);
          });
      });
    }

    // Cleanup：组件卸载或 role 变更时关闭连接
    return () => {
      clearTimeout(connectionTimeout);
      if (dcRef.current) {
        dcRef.current.close();
        dcRef.current = null;
      }
      if (pcRef.current) {
        pcRef.current.close();
        pcRef.current = null;
      }
      setDataChannel(null);
      setConnectionState("new");
      setError(null);
    };
  }, [role]); // role 变更时重建连接

  return {
    connectionState,
    dataChannel,
    error,
  };
}
