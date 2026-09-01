package message

import (
	"bytes"
	"compress/zlib"
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

//	[]model.Metric: 解析出的全部业务事件(压缩包内可能包含多个消息)
//
// B 站服务器下发的压缩包内可能拼接多个 16 字节头的消息(粘包),本函数
// 循环解析全部消息;数据长度不足或异常时丢弃剩余部分。
func ParsePacket(packet []byte) []model.Metric {
	return parseStream(packet)
}

// parseStream 循环解析字节流中的全部数据包
func parseStream(data []byte) []model.Metric {
	var metrics []model.Metric
	for len(data) >= 16 {
		packetLen := binary.BigEndian.Uint32(data[0:4])
		if packetLen < 16 || int(packetLen) > len(data) {
			// 数据损坏或分包不完整,丢弃剩余
			break
		}
		metrics = append(metrics, parseOne(data[:packetLen])...)
		data = data[packetLen:]
	}
	return metrics
}

// maxDecompressedSize 单包解压结果上限,防止恶意/损坏压缩包撑爆内存
const maxDecompressedSize = 16 << 20 // 16MB

// parseOne 解析单个数据包,返回其中包含的业务事件
func parseOne(packet []byte) []model.Metric {
	if len(packet) < 16 {
		return nil
	}
	header := BiliHeader{
		PacketLen: binary.BigEndian.Uint32(packet[0:4]),
		HeaderLen: binary.BigEndian.Uint16(packet[4:6]),
		Ver:       binary.BigEndian.Uint16(packet[6:8]),
		Op:        binary.BigEndian.Uint32(packet[8:12]),
		Seq:       binary.BigEndian.Uint32(packet[12:16]),
	}
	if header.HeaderLen < 16 || uint32(header.HeaderLen) > header.PacketLen || int(header.PacketLen) > len(packet) {
		return nil
	}
	switch header.Ver {
	case 0:
		// 普通json包
		return []model.Metric{ParseJson(packet[header.HeaderLen:header.PacketLen])}
	case 1:
		// 心跳响应,无业务消息
		slog.Debug("响应包", "header.PacketLen", header.PacketLen, "len(packet)", len(packet))
		return nil
	case 2:
		// zlib压缩包
		reader, err := zlib.NewReader(bytes.NewReader(packet[header.HeaderLen:header.PacketLen]))
		if err != nil {
			slog.Error("zlib 解压失败", "Error", err)
			return nil
		}
		decompressed, err := io.ReadAll(io.LimitReader(reader, maxDecompressedSize+1))
		reader.Close()
		if err != nil {
			slog.Error("zlib 解压失败", "Error", err)
			return nil
		}
		if len(decompressed) > maxDecompressedSize {
			slog.Error("zlib 解压结果超过上限,丢弃", "size", len(decompressed))
			return nil
		}
		return parseStream(decompressed)
	case 3:
		// Brotli压缩包
		reader := brotli.NewReader(bytes.NewReader(packet[header.HeaderLen:header.PacketLen]))
		decompressed, err := io.ReadAll(io.LimitReader(reader, maxDecompressedSize+1))
		if err != nil {
			slog.Error("brotli 解压失败", "Error", err)
			return nil
		}
		if len(decompressed) > maxDecompressedSize {
			slog.Error("brotli 解压结果超过上限,丢弃", "size", len(decompressed))
			return nil
		}
		return parseStream(decompressed)
	}
	return nil
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
