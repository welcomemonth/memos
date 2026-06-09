import { useState, useRef, useCallback, useEffect } from "react";

// 信令消息类型（与后端 Message 结构对应）
interface SignalingMessage {
  type: string;
  roomCode?: string;
  sdp?: string;
  candidate?: RTCIceCandidateInit;
  message?: string;
}

export type SignalingStatus = "idle" | "connecting" | "connected" | "disconnected";

// 回调函数类型
type Callbacks = {
  onRoomCreated: ((code: string) => void) | null;
  onRoomJoined: (() => void) | null;
  onPeerJoined: (() => void) | null;
  onPeerDisconnected: (() => void) | null;
  onOffer: ((sdp: string) => void) | null;
  onAnswer: ((sdp: string) => void) | null;
  onIceCandidate: ((candidate: RTCIceCandidateInit) => void) | null;
};

export function useSignaling() {
  const [status, setStatus] = useState<SignalingStatus>("idle");
  const [roomCode, setRoomCode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const heartbeatRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const callbacksRef = useRef<Callbacks>({
    onRoomCreated: null,
    onRoomJoined: null,
    onPeerJoined: null,
    onPeerDisconnected: null,
    onOffer: null,
    onAnswer: null,
    onIceCandidate: null,
  });

  // 消息分发
  const handleMessage = useCallback((msg: SignalingMessage) => {
    switch (msg.type) {
      case "room-created":
        if (msg.roomCode) {
          setRoomCode(msg.roomCode);
          callbacksRef.current.onRoomCreated?.(msg.roomCode);
        }
        break;
      case "room-joined":
        if (msg.roomCode) setRoomCode(msg.roomCode);
        callbacksRef.current.onRoomJoined?.();
        break;
      case "peer-joined":
        callbacksRef.current.onPeerJoined?.();
        break;
      case "peer-disconnected":
        callbacksRef.current.onPeerDisconnected?.();
        break;
      case "offer":
        if (msg.sdp) callbacksRef.current.onOffer?.(msg.sdp);
        break;
      case "answer":
        if (msg.sdp) callbacksRef.current.onAnswer?.(msg.sdp);
        break;
      case "ice-candidate":
        if (msg.candidate) callbacksRef.current.onIceCandidate?.(msg.candidate);
        break;
      case "pong":
        // 心跳响应，无需处理
        break;
      case "error":
        setError(msg.message ?? "Unknown error");
        break;
    }
  }, []);

  // 发送消息（仅在 WebSocket 就绪时）
  const send = useCallback((msg: SignalingMessage) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg));
    }
  }, []);

  // 启动心跳
  const startHeartbeat = useCallback(() => {
    heartbeatRef.current = setInterval(() => {
      send({ type: "ping" });
    }, 30000); // 30 秒
  }, [send]);

  // 停止心跳
  const stopHeartbeat = useCallback(() => {
    if (heartbeatRef.current) {
      clearInterval(heartbeatRef.current);
      heartbeatRef.current = null;
    }
  }, []);

  // 建立 WebSocket 连接
  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    setStatus("connecting");
    setError(null);

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/api/v1/webrtc/signal`;
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      setStatus("connected");
      startHeartbeat();
    };

    ws.onmessage = (event) => {
      try {
        const msg: SignalingMessage = JSON.parse(event.data);
        handleMessage(msg);
      } catch {
        // 忽略无效 JSON
      }
    };

    ws.onerror = () => {
      setError("WebSocket connection failed");
      setStatus("disconnected");
    };

    ws.onclose = () => {
      stopHeartbeat();
      setStatus("disconnected");
    };

    wsRef.current = ws;
  }, [handleMessage, startHeartbeat, stopHeartbeat]);

  // 断开连接
  const disconnect = useCallback(() => {
    stopHeartbeat();
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setStatus("idle");
    setRoomCode(null);
    setError(null);
  }, [stopHeartbeat]);

  // 组件卸载时自动清理
  useEffect(() => {
    return () => {
      stopHeartbeat();
      wsRef.current?.close();
    };
  }, [stopHeartbeat]);

  // —— 信令操作 ——
  const createRoom = useCallback(() => {
    send({ type: "create-room" });
  }, [send]);

  const joinRoom = useCallback((code: string) => {
    send({ type: "join-room", roomCode: code });
  }, [send]);

  const sendOffer = useCallback((sdp: string) => {
    send({ type: "offer", sdp });
  }, [send]);

  const sendAnswer = useCallback((sdp: string) => {
    send({ type: "answer", sdp });
  }, [send]);

  const sendIceCandidate = useCallback((candidate: RTCIceCandidateInit) => {
    send({ type: "ice-candidate", candidate });
  }, [send]);

  // —— 回调注册 ——
  // 使用 setter 函数模式，每次设置覆盖前一个回调
  // （每个事件只有一个处理者，因为当前房间只有一个对等端）
  const registerCallback = useCallback(
    (name: keyof Callbacks, cb: Callbacks[keyof Callbacks]) => {
      callbacksRef.current[name] = cb as any;
    },
    []
  );

  return {
    status,
    roomCode,
    error,
    connect,
    disconnect,
    createRoom,
    joinRoom,
    sendOffer,
    sendAnswer,
    sendIceCandidate,
    onRoomCreated: (cb: (code: string) => void) => registerCallback("onRoomCreated", cb),
    onRoomJoined: (cb: () => void) => registerCallback("onRoomJoined", cb),
    onPeerJoined: (cb: () => void) => registerCallback("onPeerJoined", cb),
    onPeerDisconnected: (cb: () => void) => registerCallback("onPeerDisconnected", cb),
    onOffer: (cb: (sdp: string) => void) => registerCallback("onOffer", cb),
    onAnswer: (cb: (sdp: string) => void) => registerCallback("onAnswer", cb),
    onIceCandidate: (cb: (candidate: RTCIceCandidateInit) => void) => registerCallback("onIceCandidate", cb),
  };
}
