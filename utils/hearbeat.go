package utils

import (
	"encoding/binary"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

func SendHeartbeatPacket(conn *websocket.Conn) {
	// 发送心跳包的协程
	ticker := time.NewTicker(30 * time.Second) // 每30秒发送一次心跳包
	defer ticker.Stop()

	// 修改后的写法
	for range ticker.C {
		heartbeatPacket := MakeHeartbeatPacket()
		err := conn.WriteMessage(websocket.BinaryMessage, heartbeatPacket)
		if err != nil {
			slog.Error("Send Hearbeat Error", "err", err)
			return // 发生错误跳出循环，协程结束
		} else {
			slog.Info("心跳包已发送")
		}
	}
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
