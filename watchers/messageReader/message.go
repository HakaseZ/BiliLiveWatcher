package message

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"log/slog"
	"time"

	"github.com/HakaseZ/BiliLiveWatcher/model"
	"github.com/andybalholm/brotli"
)

var cstZone = time.FixedZone("CST", 8*3600)

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
}

// 调试过程中分析每个数据包
// Args:
//
//	packet: 原始字节流数据包
//
// Returns:
//
//	model.
//
// Packet存在几种情况(实测没有，删除相关逻辑):
//
//	1.长度小于16字节：无法解析头部，直接丢弃.
//	2.正常的单个数据包.
//	3.粘包：一个数据包后面紧跟着另一个数据包，需要递归解析剩余数据.
//	4.分包（不确定是否存在这种情况）：一个数据包被分成多次发送，假设存在分包，打印出存在分包的提示，暂不处理分包的内容.
//
// 处理逻辑：
//  1. 解析头部信息，判断数据包的总长度、头部长度、协议版本、操作码和序列号
//  2. 根据操作码判断业务类型，分别处理心跳回复、业务通知和认证响应
//  3. 如果数据包长度大于当前包的总长度，说明存在粘包，递归解析剩余数据
//  4. 如果数据包长度小于头部信息中指定的总长度，说明当前数据包被分成多次发送，打印出存在分包的提示，暂不处理分包的内容
func ParsePacket(packet []byte) model.Metric {

	header := BiliHeader{
		PacketLen: binary.BigEndian.Uint32(packet[0:4]),
		HeaderLen: binary.BigEndian.Uint16(packet[4:6]),
		Ver:       binary.BigEndian.Uint16(packet[6:8]),
		Op:        binary.BigEndian.Uint32(packet[8:12]),
		Seq:       binary.BigEndian.Uint32(packet[12:16]),
	}
	var m model.Metric
	switch header.Ver {
	case 0:
		// 普通json包
		m = ParseJson(packet[header.HeaderLen:header.PacketLen])
		return m
	case 1:
		// 心跳包，丢弃
		slog.Debug("响应包", "header.PacketLen", header.PacketLen, "len(packet)", len(packet))
		return m
	case 2:
		// zlib压缩包
		slog.Debug("zlib", "header.PacketLen", header.PacketLen, "len(packet)", len(packet))
		return m
	case 3:
		// Brotli压缩包
		reader := brotli.NewReader(bytes.NewReader(packet[header.HeaderLen:header.PacketLen]))
		decompressed, _ := io.ReadAll(reader)
		m = ParsePacket(decompressed)
		return m
	}
	return m
}

func ParseJson(payload []byte) model.Metric {
	// var res struct {
	// 	Cmd  string          `json:"cmd"`
	// 	Data json.RawMessage `json:"data"`
	// 	Info json.RawMessage `json:"info"`
	// }
	var m model.Metric
	var msg map[string]interface{}
	if err := json.Unmarshal(payload, &msg); err == nil {
		// cmd := res.Cmd
		// switch res.Cmd {
		switch msg["cmd"] {
		case "DANMU_MSG":
			// 弹幕消息
			m.Mtype = "DANMU_MSG"
			m.Msg = deal_DANMU_MSG(msg)

			slog.Debug("DANMU_MSG", "body[info]", msg)

			return m

		case "DANMU_MSG_MIRROR":
			// 弹幕消息
			m.Mtype = "DANMU_MSG_MIRROR"
			// m.Msg = deal_DANMU_MSG(msg) // 弹幕不来自目标直播间，暂不作处理
			slog.Debug("DANMU_MSG_MIRROR", "body[info]", msg)

			return m

		case "INTERACT_WORD_V2":
			// 进入直播间
			m.Mtype = "INTERACT_WORD_V2"
			m.Msg = deal_INTERACT_WORD_V2(msg)

			slog.Debug("INTERACT_WORD_V2", "body[data]", msg)

			return m

		case "LIVE":
			// 开播
			m.Mtype = "LIVE"
			// m.Msg = deal_LIVE(msg)

			slog.Debug("LIVE", "body", msg)

			return m

		case "PREPARING":
			// 下播
			m.Mtype = "PREPARING"
			// m.Msg = deal_PREPARING(msg)

			slog.Debug("PREPARING", "body", msg)

			return m

		case "SEND_GIFT":
			// 礼物
			m.Mtype = "SEND_GIFT"
			// m.Msg = deal_SEND_GIFT(msg)

			slog.Debug("SEND_GIFT", "body", msg)

			return m
		case "PREPASUPER_CHAT_MESSAGERING":
			// SuperChat
			m.Mtype = "PREPASUPER_CHAT_MESSAGERING"
			// m.Msg = deal_PREPASUPER_CHAT_MESSAGERING(msg)

			slog.Debug("PREPASUPER_CHAT_MESSAGERING", "body", msg)

			return m

		case "ROOM_CHANGE":
			// 直播间信息变更
			m.Mtype = "ROOM_CHANGE"
			m.Msg = deal_ROOM_CHANGE(msg)

			slog.Debug("ROOM_CHANGE", "body", msg)

			return m

		default:
			slog.Debug("未处理的消息类型", "cmd", msg["cmd"], "body", msg)
			return m
		}
	} else {
		slog.Error("Parse Json Error", "Error", err)
		return m
	}
}
