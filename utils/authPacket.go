package utils

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// AuthPayload 认证包的 JSON 结构
type AuthPayload struct {
	UID      int    `json:"uid"`      // 游客传 0
	RoomID   int    `json:"roomid"`   // 房间号
	ProtoVer int    `json:"protover"` // 建议传 3 (支持最全)
	Platform string `json:"platform"` // 固定 "web"
	Type     int    `json:"type"`     // 固定 2
	Key      string `json:"key"`      // Token
	BuvID    string `json:"buvid"`    // ？
}

// MakeAuthPacket 构造 B 站二进制认证包
func MakeAuthPacket(roomID int, cookie string, token string) []byte {
	// 构造心跳包
	//
	// Args:
	// 	roomID: 直播间 ID
	// 	token: 认证 Token
	//
	// Returns:
	// 	[]byte: 构造好的二进制认证包，可以直接发送到 WebSocket服务器

	// 1. 准备 Payload
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Cookie", cookie)

	DedeUserID, _ := req.Cookie("DedeUserID")
	uid, _ := strconv.ParseInt(DedeUserID.Value, 10, 64)
	buvid3, _ := req.Cookie("buvid3")

	fmt.Println(DedeUserID.Value)
	fmt.Println(buvid3.Value)

	payload := AuthPayload{
		UID:      int(uid),
		RoomID:   roomID,
		ProtoVer: 3,
		Platform: "web",
		Type:     2,
		Key:      token,
		BuvID:    buvid3.Value,
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
