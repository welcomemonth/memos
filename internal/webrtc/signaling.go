package webrtc

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
)

const maxPeersPerRoom = 2

// WebSocket 读取超时时间（90 秒无消息视为僵尸连接）
const wsReadTimeout = 90 * time.Second

// 心跳间隔
const pingInterval = 30 * time.Second

const (
	MsgTypeCreateRoom       = "create-room"
	MsgTypeJoinRoom         = "join-room"
	MsgTypeRoomCreated      = "room-created"
	MsgTypeRoomJoined       = "room-joined"
	MsgTypePeerJoined       = "peer-joined"
	MsgTypePeerDisconnected = "peer-disconnected"
	MsgTypeOffer            = "offer"
	MsgTypeAnswer           = "answer"
	MsgTypeIceCandidate     = "ice-candidate"
	MsgTypeError            = "error"
	MsgTypePing             = "ping"
	MsgTypePong             = "pong"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Message struct {
	Type      string `json:"type"`
	RoomCode  string `json:"roomCode"`
	SDP       string `json:"sdp"`
	Candidate any    `json:"candidate"`
	Message   string `json:"message"`
}

type Peer struct {
	ID       string
	Conn     *websocket.Conn
	WriteMu  sync.Mutex
	RoomCode string
}

type Room struct {
	Code      string
	Peers     []*Peer
	CreatedAt time.Time
	mu        sync.Mutex
}

type Hub struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
	}
}

func generateRoomCode() (string, error) {
	// big.NewInt(1000000): 范围 [0, 999999]
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate room code: %w", err)
	}
	// %06d: 用前导零补齐到 6 位
	// 比如 n=48215 → "048215"，n=915 → "000915"
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// 为什么需要 WriteMu？ gorilla/websocket 的连接不支持并发写。如果一个 goroutine 在写 Offer 消息，另一个 goroutine 同时在写 ICE 候选消息，会 panic。WriteMu 保证同一时刻只有一个 goroutine 在写。
// 注意：读操作（ReadJSON）不需要加锁，因为 WebSocket 读循环只在一个 goroutine 中运行。
func SendJSON(peer *Peer, msg any) error {
	peer.WriteMu.Lock()
	defer peer.WriteMu.Unlock()
	return peer.Conn.WriteJSON(msg)
}

// — 调 generateRoomCode()，创建 Room，存入 h.rooms[code]，返回 room
func (h *Hub) CreateRoom() (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for {
		code, err := generateRoomCode()
		if err != nil {
			return nil, err
		}
		if _, exist := h.rooms[code]; !exist {
			room := &Room{
				Code:      code,
				Peers:     make([]*Peer, 0, maxPeersPerRoom),
				CreatedAt: time.Now(),
			}
			h.rooms[code] = room
			return room, nil
		}
	}
}

// — RLock 读取 为什么用 RLock 而不是 Lock？ 读取 map 不修改数据。多个 goroutine 可以同时持有读锁，不会互相阻塞。如果用写锁，一次只能一个 goroutine 读，会成为性能瓶颈。这就是 RWMutex 的作用——读读不互斥，读写互斥。
func (h *Hub) GetRoom(code string) (*Room, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	room, ok := h.rooms[code]
	return room, ok
}

// — Lock，delete(h.rooms, code)
func (h *Hub) RemoveRoom(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.rooms, code)
}

// — 获取房间，Lock，检查 len(room.Peers) >= maxPeersPerRoom → 返回 fmt.Errorf("room is full")，否则 append
func (h *Hub) AddPeerToRoom(code string, peer *Peer) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	room, ok := h.rooms[code]
	if !ok {
		return fmt.Errorf("room %s not found", code)
	}
	room.mu.Lock()
	defer room.mu.Unlock()
	if len(room.Peers) >= maxPeersPerRoom {
		return fmt.Errorf("room is full")
	}
	room.Peers = append(room.Peers, peer)
	peer.RoomCode = code
	return nil
}

// — 获取房间，Lock，遍历 Peers 移除匹配 ID 的那个，如果 Peers 为空则调 h.RemoveRoom(code)
func (h *Hub) RemovePeerFromRoom(code string, peerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[code]
	if !ok {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	// 遍历 Peers，移除匹配 ID 的那一个
	for i, p := range room.Peers {
		if p.ID == peerID {
			// 用 append 技巧删除元素（保持顺序）
			room.Peers = append(room.Peers[:i], room.Peers[i+1:]...)
			break
		}
	}

	// 房间空了就删除
	if len(room.Peers) == 0 {
		delete(h.rooms, code)
	}
}

func (h *Hub) HandleSignaling(c *echo.Context) error {
	// ① 升级 HTTP → WebSocket
	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return err
	}
	// ② 创建 Peer
	peerID := uuid.New().String()
	peer := &Peer{
		ID:   peerID,
		Conn: conn,
	}
	// ③ 整个生命周期结束后的清理（后进先出执行）
	defer func() {
		if peer.RoomCode != "" {
			h.RemovePeerFromRoom(peer.RoomCode, peerID)
			// 通知对等端
			h.RelayToOtherPeer(peer.RoomCode, peerID, Message{
				Type: MsgTypePeerDisconnected,
			})
		}
		conn.Close()
		log.Printf("Peer %s disconnected", peerID)
	}()

	// ④ 初始读超时
	conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	// ⑤ 启动心跳 goroutine
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := SendJSON(peer, Message{Type: MsgTypePing}); err != nil {
				return // 写失败则退出（连接已断）
			}
		}
	}()

	// ⑥ 消息循环
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			// 正常关闭 vs 异常断开
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("Peer %s disconnected normally", peerID)
			} else {
				log.Printf("Peer %s read error: %v", peerID, err)
			}
			break // 退出循环，触发 defer 清理
		}

		// 每次收到消息刷新读超时
		conn.SetReadDeadline(time.Now().Add(wsReadTimeout))

		// 消息分发
		h.handleMessage(peer, msg)
	}
	return nil
}

func (h *Hub) handleMessage(peer *Peer, msg Message) {
	switch msg.Type {
	case MsgTypeCreateRoom:
		room, err := h.CreateRoom()
		if err != nil {
			SendJSON(peer, Message{Type: MsgTypeError, Message: err.Error()})
			return
		}
		if err := h.AddPeerToRoom(room.Code, peer); err != nil {
			SendJSON(peer, Message{Type: MsgTypeError, Message: err.Error()})
			return
		}
		SendJSON(peer, Message{Type: MsgTypeRoomCreated, RoomCode: room.Code})

	case MsgTypeJoinRoom:
		if err := h.AddPeerToRoom(msg.RoomCode, peer); err != nil {
			SendJSON(peer, Message{Type: MsgTypeError, Message: err.Error()})
			return
		}
		SendJSON(peer, Message{Type: MsgTypeRoomJoined, RoomCode: msg.RoomCode})
		// 通知房间内的另一 Peer
		h.RelayToOtherPeer(msg.RoomCode, peer.ID, Message{Type: MsgTypePeerJoined})

	case MsgTypeOffer, MsgTypeAnswer, MsgTypeIceCandidate:
		// 这三种消息统一转发
		h.RelayToOtherPeer(peer.RoomCode, peer.ID, msg)

	case MsgTypePing:
		SendJSON(peer, Message{Type: MsgTypePong})

	default:
		SendJSON(peer, Message{Type: MsgTypeError, Message: "unknown message type: " + msg.Type})
	}
}

func (h *Hub) RelayToOtherPeer(roomCode string, fromPeerID string, msg any) {
	room, ok := h.GetRoom(roomCode)
	if !ok {
		return
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	for _, p := range room.Peers {
		if p.ID != fromPeerID {
			SendJSON(p, msg)
			return // 只有一个对等端，找到了就结束
		}
	}
}
