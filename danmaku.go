// 构造Https请求进行握手，并连接websocket弹幕服务器
//
// 测试房间号
// 92613 API
// uri = wss://host.port/sub
// getDanmuInfo_uri = https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo
// nav_uri = https://api.bilibili.com/x/web-interface/nav
//
// 步骤：通过KeysUrl获取WbiKeys

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/HakaseZ/BiliLiveWatcher/utils"

	"github.com/andybalholm/brotli"
	"github.com/gorilla/websocket"
)

func main() {
	roomID := 92613
	token, hostList, _ := utils.GetDanmuInfo(roomID)
	host := hostList[0].Host // 使用第一个主机地址
	wsURL := fmt.Sprintf("wss://%s/sub", host)

	// 1. 建立连接
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Println("Dial error:", err)
	}
	defer conn.Close()

	// 2. 发送认证包
	authPacket := MakeAuthPacket(roomID, token)
	err = conn.WriteMessage(websocket.BinaryMessage, authPacket)
	if err != nil {
		fmt.Println("Send auth error:", err)
	}

	fmt.Println("认证包已发送，等待 B 站响应...")

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			break
		}

		go func() {
			ticker := time.NewTicker(30 * time.Second) // 每30秒发送一次心跳包
			defer ticker.Stop()

			// 修改后的写法
			for range ticker.C {
				heartbeatPacket := MakeHeartbeatPacket()
				err := conn.WriteMessage(websocket.BinaryMessage, heartbeatPacket)
				if err != nil {
					fmt.Println("Send heartbeat error:", err)
					return // 发生错误跳出循环，协程结束
				}
				fmt.Println("心跳包已发送")
			}
		}()

		ParsePacket(message)
		// 		// 解析消息
		// 		if len(message) < 16 {
		// 			fmt.Println("Received message too short to parse")
		// 			continue
		// 		}
		//
		// 		opCode := binary.BigEndian.Uint32(message[8:12])
		// 		switch opCode {
		// 		case 8: // Auth response
		// 			decodeAuthResponse(message)
		// 			// 启动	心跳 goroutine
		// 			go func() {
		// 				ticker := time.NewTicker(30 * time.Second) // 每30秒发送一次心跳包
		// 				defer ticker.Stop()
		//
		// 				// 修改后的写法
		// 				for range ticker.C {
		// 					heartbeatPacket := MakeHeartbeatPacket()
		// 					err := conn.WriteMessage(websocket.BinaryMessage, heartbeatPacket)
		// 					if err != nil {
		// 						fmt.Println("Send heartbeat error:", err)
		// 						return // 发生错误跳出循环，协程结束
		// 					}
		// 					fmt.Println("心跳包已发送")
		// 				}
		// 			}()
		// 		case 5: // Danmu message
		// 			decodeDanmuMessage(message)
		// 		case 3: // Heartbeat response
		// 			fmt.Println("Received heartbeat response")
		// 			// 可以在这里解析心跳响应的负载，提取在线人数等信息
		// 		default:
		// 			fmt.Printf("Received message with unknown OpCode %d\n", opCode)
		// 		}
	}
}

// BiliHeader 16字节头部结构
type BiliHeader struct {
	PacketLen uint32 // 总长度
	HeaderLen uint16 // 头部长度 (固定16)
	Ver       uint16 // 协议版本 (0:JSON, 1:心跳/认证, 2:zlib, 3:brotli)
	Op        uint32 // 操作码 (3:心跳, 5:通知, 8:认证成功)
	Seq       uint32 // 序列号
}

// ParsePacket 解析收到的原始字节流
func ParsePacket(data []byte) {
	if len(data) < 16 {
		return
	}

	header := BiliHeader{
		PacketLen: binary.BigEndian.Uint32(data[0:4]),
		HeaderLen: binary.BigEndian.Uint16(data[4:6]),
		Ver:       binary.BigEndian.Uint16(data[6:8]),
		Op:        binary.BigEndian.Uint32(data[8:12]),
		Seq:       binary.BigEndian.Uint32(data[12:16]),
	}

	payload := data[header.HeaderLen:header.PacketLen]

	switch header.Op {
	case 8:
		fmt.Println("[系统] 认证成功！")
	case 3:
		// 心跳回复，通常包含当前人气值
		popularity := binary.BigEndian.Uint32(payload)
		fmt.Printf("[人气] 当前在线人气: %d\n", popularity)
	case 5:
		// 业务通知（弹幕、礼物等）
		handleCommand(header.Ver, payload)
	default:
		fmt.Printf("[未知] Op: %d\n", header.Op)
	}

	// 粘包处理：如果 Data 长度大于当前 PacketLen，说明后面还有包
	if len(data) > int(header.PacketLen) {
		ParsePacket(data[header.PacketLen:])
	}
}

// handleCommand 处理具体指令
func handleCommand(ver uint16, payload []byte) {
	switch ver {
	case 0, 1:
		// 未压缩的 JSON
		var body map[string]interface{}
		if err := json.Unmarshal(payload, &body); err == nil {
			cmd, _ := body["cmd"].(string)
			if cmd == "DANMU_MSG" {
				// 简化提取：通常 info[1] 是内容，info[2][1] 是昵称
				fmt.Printf("[弹幕] %v\n", body["info"])
			} else {
				fmt.Printf("[消息] 收到命令: %s\n", cmd)
			}
		}
	case 3:
		fmt.Println("[系统] 收到 Brotli 压缩包，正在解压...")
		// Brotli 压缩
		reader := brotli.NewReader(bytes.NewReader(payload))
		decompressed, _ := io.ReadAll(reader)
		// 注意：解压后的内容是完整的 B 站协议包（Header+Payload），需要递归解析
		ParsePacket(decompressed)
	case 2:
		// zlib 压缩 (逻辑同上，使用 compress/zlib)
		fmt.Println("[警告] 收到 zlib 压缩包，请自行实现解压逻辑")
	}
}

// AuthPayload 认证包的 JSON 结构
type AuthPayload struct {
	UID      int    `json:"uid"`      // 游客传 0
	RoomID   int    `json:"roomid"`   // 房间号
	ProtoVer int    `json:"protover"` // 建议传 3 (支持最全)
	Platform string `json:"platform"` // 固定 "web"
	Type     int    `json:"type"`     // 固定 2
	Key      string `json:"key"`      // 就是你拿到的那个 Token
}

// MakeAuthPacket 构造 B 站二进制认证包
func MakeAuthPacket(roomID int, token string) []byte {
	// 1. 准备 Payload
	payload := AuthPayload{
		UID:      0,
		RoomID:   roomID,
		ProtoVer: 3,
		Platform: "web",
		Type:     2,
		Key:      token,
	}
	payloadBytes, _ := json.Marshal(payload)

	// 2. 构造 Header (16 字节)

	header := make([]byte, 16)
	totalLen := uint32(len(payloadBytes) + 16)

	binary.BigEndian.PutUint32(header[0:4], totalLen) // 总长度
	binary.BigEndian.PutUint16(header[4:6], 16)       // 头部长度
	binary.BigEndian.PutUint16(header[6:8], 1)        // 协议版本
	binary.BigEndian.PutUint32(header[8:12], 7)       // 操作码: 7 (Auth)
	binary.BigEndian.PutUint32(header[12:16], 1)      // 序列号

	// 3. 拼接 Header + Payload
	var packet bytes.Buffer
	packet.Write(header)
	packet.Write(payloadBytes)

	return packet.Bytes()
}

func decodeAuthResponse(data []byte) {
	// 解析认证响应包，提取状态码和消息
	if len(data) < 16 {
		fmt.Println("Invalid auth response: too short")
		return
	}

	payload := data[16:]

	var authResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	err := json.Unmarshal(payload, &authResp)
	if err != nil {
		fmt.Println("Failed to parse auth response payload:", err)
		return
	}

	fmt.Printf("Auth Response Payload - Code: %d, Msg: %s\n", authResp.Code, authResp.Msg)
}

func decodeDanmuMessage(data []byte) {
	fmt.Printf("Received danmu message. \n")
}

func MakeHeartbeatPacket() []byte {
	// 构造 B 站二进制心跳包
	header := make([]byte, 16)
	binary.BigEndian.PutUint32(header[0:4], 16)  // 总长度
	binary.BigEndian.PutUint16(header[4:6], 16)  // 头部长度
	binary.BigEndian.PutUint16(header[6:8], 1)   // 协议版本
	binary.BigEndian.PutUint32(header[8:12], 2)  // 操作码: 2 (Heartbeat)
	binary.BigEndian.PutUint32(header[12:16], 1) // 序列号

	return header
}
