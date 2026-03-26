package message

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/andybalholm/brotli"
)

var cstZone = time.FixedZone("CST", 8*3600)
var debugLogger *slog.Logger

type BiliHeader struct {
	PacketLen uint32 // 总长度
	HeaderLen uint16 // 头部长度 (固定16)
	Ver       uint16 // 协议版本 (0:JSON, 1:心跳/认证, 2:zlib, 3:brotli)
	Op        uint32 // 操作码 (3:心跳, 5:通知, 8:认证成功)
	Seq       uint32 // 序列号
}

func init() {
	// 初始化时间设置
	var err error
	time.Local, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		time.Local = time.FixedZone("CST", 8*3600)
	}

	//初始化全局logger
	debuglogfile, _ := os.OpenFile(".tmp/debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	debugLogger = slog.New(slog.NewTextHandler(debuglogfile, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func ParsePacket(packet []byte, packetId int) {
	// 调试过程中分析每个数据包
	// Args:
	// 	packet: 原始字节流数据包
	// 	packetId: 数据包编号（用于调试和日志记录）

	// packet存在几种情况
	// 1，长度小于16字节：无法解析头部，直接丢弃
	// 2，单个数据包
	// 3，粘包：一个数据包后面紧跟着另一个数据包，需要递归解析剩余数据
	// 4，分包（不确定是否存在这种情况）：一个数据包被分成多次发送，假设存在分包，打印出存在分包的提示，暂不处理分包的内容

	// 处理逻辑：
	// 1. 解析头部信息，判断数据包的总长度、头部长度、协议版本、操作码和序列号
	// 2. 根据操作码判断业务类型，分别处理心跳回复、业务通知和认证响应
	// 3. 如果数据包长度大于当前包的总长度，说明存在粘包，递归解析剩余数据
	// 4. 如果数据包长度小于头部信息中指定的总长度，说明当前数据包被分成多次发送，打印出存在分包的提示，暂不处理分包的内容

	header := BiliHeader{
		PacketLen: binary.BigEndian.Uint32(packet[0:4]),
		HeaderLen: binary.BigEndian.Uint16(packet[4:6]),
		Ver:       binary.BigEndian.Uint16(packet[6:8]),
		Op:        binary.BigEndian.Uint32(packet[8:12]),
		Seq:       binary.BigEndian.Uint32(packet[12:16]),
	}
	if len(packet) > int(header.PacketLen) {
		// fmt.Printf("[%s] 粘包, Header.PacketLen:%d, RealPacketLen:%d\n", time.Now().In(cstZone).Format("2006-01-02 15:04:05"), header.PacketLen, len(packet))
		ParsePacket(packet[:header.PacketLen], packetId)
		ParsePacket(packet[header.PacketLen:], packetId*100)
	} else if len(packet) == int(header.PacketLen) {
		switch header.Ver {
		case 0:
			// 解析json
			// fmt.Printf("[%s] Json, Header.PacketLen:%d, RealPacketLen:%d\n", time.Now().In(cstZone).Format("2006-01-02 15:04:05"), header.PacketLen, len(packet))
			ParseJson(packet[header.HeaderLen:header.PacketLen])
		case 1:
			// 心跳包，丢弃
			// fmt.Printf("[%s] 响应包, Header.PacketLen:%d, RealPacketLen:%d\n", time.Now().In(cstZone).Format("2006-01-02 15:04:05"), header.PacketLen, len(packet))
			debugLogger.Debug("响应包", "header.PacketLen", header.PacketLen, "len(packet)", len(packet))
		case 2:
			// zlib压缩包
			// fmt.Printf("[%s] zlib, Header.PacketLen:%d, RealPacketLen:%d\n", time.Now().In(cstZone).Format("2006-01-02 15:04:05"), header.PacketLen, len(packet))
			debugLogger.Debug("zlib", "header.PacketLen", header.PacketLen, "len(packet)", len(packet))
		case 3:
			// Brotli压缩包
			// fmt.Printf("[%s] Brotli, Header.PacketLen:%d, RealPacketLen:%d\n", time.Now().In(cstZone).Format("2006-01-02 15:04:05"), header.PacketLen, len(packet))
			reader := brotli.NewReader(bytes.NewReader(packet[header.HeaderLen:header.PacketLen]))
			decompressed, _ := io.ReadAll(reader)
			ParsePacket(decompressed, packetId)
		}

		// PrasePayload(packet[header.HeaderLen:header.PacketLen])

	} else {
		fmt.Printf("[%s] 分包, Header.PacketLen:%d, RealPacketLen:%d\n", time.Now().In(cstZone).Format("2006-01-02 15:04:05"), header.PacketLen, len(packet))
	}

}

func ParseJson(payload []byte) {
	var body map[string]interface{}
	if err := json.Unmarshal(payload, &body); err == nil {
		cmd, _ := body["cmd"].(string)
		switch cmd {
		case "DANMU_MSG":
			// 弹幕消息
			// fmt.Printf("[%s] [%s] %s(%f): %s\n", time.Now().In(cstZone).Format("2006-01-02 15:04:05"), cmd, body["info"].([]interface{})[2].([]interface{})[1], body["info"].([]interface{})[2].([]interface{})[0], body["info"].([]interface{})[1])
			slog.Info(cmd, body["info"].([]interface{})[2].([]interface{})[1], body["info"].([]interface{})[1])
			debugLogger.Debug(cmd, "body[info]", body["info"])
		case "INTERACT_WORD_V2":
			// 进入直播间
			debugLogger.Debug(cmd, "body", body)
		case "ENTRY_EFFECT":
			// 进场特效（不处理）
			debugLogger.Debug(cmd)
		case "LIVE":
			// 开播
			debugLogger.Debug(cmd, "body", body)
		case "PREPARING":
			// 下播
			debugLogger.Debug(cmd, "body", body)
		case "SEND_GIFT":
			// 礼物
			debugLogger.Debug(cmd, "body", body)
		case "PREPASUPER_CHAT_MESSAGERING":
			// SC
			debugLogger.Debug(cmd, "body", body)
		default:
			// fmt.Printf("[%s] [%s]\n", time.Now().In(cstZone).Format("2006-01-02 15:04:05"), cmd)
			debugLogger.Debug(cmd)
		}
	} else {
		slog.Error("Parse Json Error", "err", err)
	}
}
