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
	rander "math/rand"
	"net/http"
	"os"

	tmp "github.com/HakaseZ/BiliLiveWatcher/.tmp"
	"github.com/HakaseZ/BiliLiveWatcher/utils"

	"github.com/andybalholm/brotli"
	"github.com/gorilla/websocket"
)

func main() {
	roomID := 92613
	token, hostList, _ := utils.GetDanmuInfo(roomID)
	// for _, host := range hostList {
	// 	go connectWebSocket(roomID, token, host.Host)
	// }
	connectWebSocket(roomID, token, hostList[0].Host)
	select {}
}
func connectWebSocket(roomID int, token string, host string) {

	wsURL := fmt.Sprintf("wss://%s/sub", host)
	fmt.Printf("正在连接弹幕服务器:%s\n", wsURL)

	// 1. 建立连接
	header := http.Header{}
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	header.Set("Referer", "https://www.bilibili.com")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		fmt.Println("Dial error:", err)
	}
	defer conn.Close()

	// 2. 发送认证包
	authPacket := utils.MakeAuthPacket(roomID, token)
	err = conn.WriteMessage(websocket.BinaryMessage, authPacket)
	if err != nil {
		fmt.Println("Send auth error:", err)
	}
	fmt.Println("认证包已发送，等待 B 站响应...")

	// 发送心跳包的协程
	go utils.SendHeartbeatPacket(conn)

	// 3. 处理消息流
	messageCounter := 0
	for {
		messageCounter++
		_, packet, err := conn.ReadMessage()
		// fmt.Printf("收到原始包: %X\n", packet[:16])
		if err != nil {
			fmt.Println("Read error:", err)
			break
		}
		tmp.ParsePacket(packet, messageCounter)
		// fmt.Printf("包编号%d\n", messageCounter)
		// ParsePacket(message, mes)
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
func ParsePacket(data []byte, seqchan int) {
	seq := rander.Intn(100)
	// fmt.Printf("收到数据包，长度: %d 字节，编号: %d，来自: %d\n", len(data), seq, seqchan)
	if len(data) < 16 {
		fmt.Printf("Len(data) < 16")
		return
	} else {
		header := BiliHeader{
			PacketLen: binary.BigEndian.Uint32(data[0:4]),
			HeaderLen: binary.BigEndian.Uint16(data[4:6]),
			Ver:       binary.BigEndian.Uint16(data[6:8]),
			Op:        binary.BigEndian.Uint32(data[8:12]),
			Seq:       binary.BigEndian.Uint32(data[12:16]),
		}
		// 粘包处理：如果 Data 长度大于当前 PacketLen，说明后面还有包
		if len(data) > int(header.PacketLen) {
			go ParsePacket(data[header.PacketLen:], seq)
			//fmt.Printf("[粘包] 发现粘包，继续解析剩余数据...\n")
		}
		payload := data[header.HeaderLen:header.PacketLen]

		switch header.Op {
		case 8:
			fmt.Println("[系统] 认证成功！")
		case 3:
			// 心跳回复
			fmt.Printf("收到心跳回复\n")
		case 5:
			// 业务通知（弹幕、礼物等）
			handleCommand(header.Ver, payload)
		default:
			fmt.Printf("[未知] Op: %d\n", header.Op)
		}

	}
}

func PraseDANMU_MSG(info []interface{}) {
	// info[1] 是用户信息，info[2] 是弹幕内容
	userInfo := info[1].([]interface{})
	danmuContent := info[1].(string)
	fmt.Printf("[弹幕] %s: %s\n", userInfo[1], danmuContent)
}

// handleCommand 处理具体指令
func handleCommand(ver uint16, payload []byte) {
	switch ver {
	case 0, 1:
		// 未压缩的 JSON
		// savePayloadToFile(payload)
		var body map[string]interface{}
		if err := json.Unmarshal(payload, &body); err == nil {
			cmd, _ := body["cmd"].(string)
			if cmd == "DANMU_MSG" {
				// 提取出“用户：弹幕内容”
				// fmt.Printf("[弹幕] %s\n", body["info"])
				// fmt.Printf("[弹幕] %s\n", body["info"].([]interface{})[1])
				// fmt.Printf("[弹幕] %s\n", body["info"].([]interface{})[2])
				fmt.Printf("[弹幕] %s(%f): %s\n", body["info"].([]interface{})[2].([]interface{})[1], body["info"].([]interface{})[2].([]interface{})[0], body["info"].([]interface{})[1])

			} else {
				// fmt.Printf("[消息] 收到命令: %s\n", cmd)
			}
		}
	case 3:
		//fmt.Println("[系统] 收到 Brotli 压缩包，正在解压...")
		// Brotli 压缩
		reader := brotli.NewReader(bytes.NewReader(payload))
		decompressed, _ := io.ReadAll(reader)
		// 注意：解压后的内容是完整的 B 站协议包（Header+Payload），需要递归解析
		ParsePacket(decompressed, 0)
	case 2:
		// zlib 压缩 (逻辑同上，使用 compress/zlib)
		fmt.Println("[警告] 收到 zlib 压缩包，请自行实现解压逻辑")
	}
}

func savePayloadToFile(payload []byte) {
	// os.O_APPEND: 追加模式
	// os.O_CREATE: 文件不存在则创建
	// os.O_WRONLY: 只写模式
	f, err := os.OpenFile("danmu_log.xml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	// 直接写入字节流，并在末尾加个换行符，方便后续按行读取
	f.Write(payload)
	f.WriteString("\n")
}
