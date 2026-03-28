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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/HakaseZ/BiliLiveWatcher/message"
	"github.com/HakaseZ/BiliLiveWatcher/utils"
	"github.com/gorilla/websocket"

	"gopkg.in/yaml.v3"
)

type WatcherConfig struct {
	RoomId int      `yaml:"roomid"`
	URL    []string `yaml:"url"`
}

type Config struct {
	Credential struct {
		Cookie string `yaml:"cookie"`
	} `yaml:"credential"`
	Watchers []WatcherConfig `yaml:"watcher"`
}

func init() {
	// 初始化时间设置
	var err error
	time.Local, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		time.Local = time.FixedZone("CST", 8*3600)
	}

	//初始化全局logger
	logfile, _ := os.OpenFile(".tmp/BiliLiveWatcher.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	logger := slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, logfile), &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	slog.Info("系统初始化完成")
}

func main() {
	// 读取配置
	rawconf, _ := os.ReadFile(".config/config.yml")
	conf := &Config{}
	_ = yaml.Unmarshal(rawconf, conf)

	for _, watcher := range conf.Watchers {
		// 补全配置信息
		// 启动通知管理器
		// 启动监听器
		roomID, UID := utils.GetRoomPlayInfo(watcher.RoomId)
		go utils.Notifier(roomID, UID)
		token, hostList, _ := utils.GetDanmuInfo(roomID, conf.Credential.Cookie)
		connectWebSocket(roomID, conf.Credential.Cookie, token, hostList[0].Host)
	}
	select {}
}

func connectWebSocket(roomID int, cookie string, token string, host string) {

	wsURL := fmt.Sprintf("wss://%s/sub", host)
	slog.Info("正在连接弹幕服务器", "URL", wsURL)

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
	authPacket := utils.MakeAuthPacket(roomID, cookie, token)
	err = conn.WriteMessage(websocket.BinaryMessage, authPacket)
	if err != nil {
		slog.Error("Send Auth Error", "err", err)
	} else {
		slog.Info("弹幕服务器认证成功")
	}

	// 发送心跳包的协程
	go utils.SendHeartbeatPacket(conn)

	// 3. 处理消息流
	messageCounter := 0
	for {
		messageCounter++
		_, packet, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			break
		}
		message.ParsePacket(packet, messageCounter)
	}
}
