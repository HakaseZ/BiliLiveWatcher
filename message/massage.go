package message

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/andybalholm/brotli"
)

// BiliHeader 16字节头部结构
type BiliHeader struct {
	PacketLen uint32 // 总长度
	HeaderLen uint16 // 头部长度 (固定16)
	Ver       uint16 // 协议版本 (0:JSON, 1:心跳/认证, 2:zlib, 3:brotli)
	Op        uint32 // 操作码 (3:心跳, 5:通知, 8:认证成功)
	Seq       uint32 // 序列号
}

func ParsePacket(packet []byte, packetId int) {
	// 解析并处理的原始字节流

	// 包的长度至少要有16字节的头部
	if len(packet) < 16 {
		return
	} else {
		header := BiliHeader{
			PacketLen: binary.BigEndian.Uint32(packet[0:4]),
			HeaderLen: binary.BigEndian.Uint16(packet[4:6]),
			Ver:       binary.BigEndian.Uint16(packet[6:8]),
			Op:        binary.BigEndian.Uint32(packet[8:12]),
			Seq:       binary.BigEndian.Uint32(packet[12:16]),
		}

		// 粘包处理：如果 packet 长度大于当前 PacketLen，说明后面还有包
		if len(packet) > int(header.PacketLen) {
			fmt.Printf("处理粘包\n")
			go ParsePacket(packet[header.PacketLen:], packetId)
		}

		payload := packet[header.HeaderLen:header.PacketLen]

		switch header.Op {
		// 根据操作码判断业务类型
		//
		// op3：
		// 	心跳回复
		// op5：
		// 	业务通知
		// op8:
		// 	认证响应

		case 3:
		case 5:
			handleCommand(header.Ver, payload)
		case 8:
		default:
			fmt.Printf("收到未知操作码: %d\n", header.Op)
			// 将未知操作码以及对应的包保存至本地文件，方便后续分析
			f, err := os.OpenFile(fmt.Sprintf("./.tmp/[%v]unknown_ops.log", time.Now().Unix()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Println("无法打开日志文件:", err)
				return
			}
			defer f.Close()
			logEntry := fmt.Sprintf("Op: %d, Ver: %d, Payload: %s\n", header.Op, header.Ver, string(payload))
			if _, err := f.WriteString(logEntry); err != nil {
				fmt.Println("无法写入日志文件:", err)
			}
		}

	}
}
func handleCommand(ver uint16, payload []byte) {
	switch ver {
	// 根据协议版本判断 payload 的处理方式
	//
	// ver0/1:
	// 	未压缩的 JSON
	// ver2:
	// 	zlib 压缩
	// ver3:
	// 	brotli 压缩

	case 0, 1:
		// 未压缩的 JSON
		handleJSON(payload)
	case 2:
		//fmt.Println("[系统] 收到 zlib 压缩包，正在解压...")
		// zlib 压缩
		reader, _ := zlib.NewReader(bytes.NewReader(payload))
		decompressed, _ := io.ReadAll(reader)
		reader.Close()
		// 注意：解压后的内容是完整的 B 站协议包（Header+Payload），需要递归解析
		ParsePacket(decompressed)
	case 3:
		//fmt.Println("[系统] 收到 Brotli 压缩包，正在解压...")
		// Brotli 压缩
		reader := brotli.NewReader(bytes.NewReader(payload))
		decompressed, _ := io.ReadAll(reader)
		// 注意：解压后的内容是完整的 B 站协议包（Header+Payload），需要递归解析
		ParsePacket(decompressed)
	default:
		fmt.Printf("收到未知协议版本: %d\n", ver)
		// 将未知协议版本以及对应的包保存至本地文件，方便后续分析
		f, err := os.OpenFile(fmt.Sprintf("./.tmp/[%v]unknown_vers.log", time.Now().Unix()), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("无法打开日志文件:", err)
			return
		}
		defer f.Close()
		logEntry := fmt.Sprintf("Ver: %d, Payload: %s\n", ver, string(payload))
		if _, err := f.WriteString(logEntry); err != nil {
			fmt.Println("无法写入日志文件:", err)
		}
	}
}
