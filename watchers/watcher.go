package watchers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/HakaseZ/BiliLiveWatcher/model"
	"github.com/HakaseZ/BiliLiveWatcher/utils"
	m "github.com/HakaseZ/BiliLiveWatcher/watchers/messageReader"
	"github.com/gorilla/websocket"
)

type LiveWatcher struct {
	roomID  int
	UID     int
	wsHosts []wsHost
	Token   string
	conn    *websocket.Conn
	cookie  string
}

type wsHost struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	WSSPort int    `json:"wss_port"`
	WSPort  int    `json:"ws_port"`
}

// 初始化LiveWatcher
//
// Args:
//
//	roomid (int): 直播间ID
//	cookie (string): 认证用cookie
//
// Returns:
//
//	watcher (LiveWatcher): LiveWatcher对象
func NewWatcher(roomid int, cookie string) LiveWatcher {
	var w LiveWatcher
	w.cookie = cookie
	w.roomID, w.UID = w.getRoomPlayInfo(roomid)
	w.Token, w.wsHosts = w.getDanmuInfo()
	return w
}

func (w LiveWatcher) Run(ctx context.Context, c chan model.Metric) {
	wsURL := fmt.Sprintf("wss://%s/sub", w.wsHosts[0].Host) //使用默认服务器
	slog.Info("正在连接弹幕服务器", "RoomID", w.roomID, "URL", wsURL)

	// 建立连接
	header := http.Header{}
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	header.Set("Referer", "https://www.bilibili.com")

	var err error
	w.conn, _, err = websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		slog.Error("Error dialer ws server", "Error", err)
	}
	defer w.conn.Close()
	// 发送认证和心跳包
	w.sendAuthPacket()
	go w.sendHeartbeatPacket()

	go func() {
		<-ctx.Done()
		slog.Info("收到重连信号，关闭当前WebSocket连接")
		sleepTime := time.Duration(rand.Intn(10)) * time.Second
		time.Sleep(sleepTime)
		w.conn.Close()
	}()
	defer close(c)
	for {
		_, packet, err := w.conn.ReadMessage()
		if err != nil {
			slog.Error("Error read message.", "Error", err)
		}
		messgae := m.ParsePacket(packet)
		if messgae.Mtype != "" {
			messgae.M_RoomID = int64(w.roomID)
			c <- messgae
		}
	}
}

// 获取直播间详情
func (w LiveWatcher) getRoomPlayInfo(shortId int) (roomId int, uid int) {
	type RooomPlayInfoResponse struct {
		Code int `json:"code"`
		Data struct {
			RoomID     int  `json:"room_id"`
			UID        int  `json:"uid"`
			LiveStatus bool `json:"live_status"`
		} `json:""`
	}
	baseURL := "https://api.live.bilibili.com/xlive/web-room/v1/index/getRoomPlayInfo"
	query := url.Values{}
	sid := strconv.Itoa(shortId)
	query.Add("room_id", sid)
	reqURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	request, _ := http.NewRequest("GET", reqURL, nil)
	request.Header.Set("Referer", "https://www.bilibili.com")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		slog.Error("Error get room play info. ", "Errors", err)
	}

	// 解析响应
	var danmuInfoRes RooomPlayInfoResponse
	json.NewDecoder(response.Body).Decode(&danmuInfoRes)
	defer response.Body.Close()
	return danmuInfoRes.Data.RoomID, danmuInfoRes.Data.UID
}

// 获取弹幕服务器配置
//
// Args:
//
//	roomID (int64): 直播间ID
//	cookie (string): 用户cookie，若为空则匿名访问
//
// Returns:
//
//	Token (string): 连接弹幕服务器的Token
//	HostList ([]WsHost): 弹幕服务器列表，包含Host、Port等信息
func (w LiveWatcher) getDanmuInfo() (string, []wsHost) {

	type DanmuInfoResponse struct {
		Code int `json:"code"`
		Data struct {
			Token    string   `json:"token"`
			HostList []wsHost `json:"host_list"`
		} `json:"data"`
	}

	params := make(map[string]interface{})
	params["id"] = w.roomID
	params["type"] = "0"
	params["web_location"] = "444.8"
	params["wts"] = fmt.Sprintf("%d", time.Now().Unix()) // 添加时间戳

	w_rid := utils.EncWbi(params) // 调用EncWbi函数生成w_rid并添加到参数中
	params["w_rid"] = w_rid

	query := url.Values{}
	for key, value := range params {
		query.Add(key, fmt.Sprintf("%v", value))
	}
	// 构造请求URL和参数
	baseURL := "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo"
	reqURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	// 发送HTTP请求

	request, _ := http.NewRequest("GET", reqURL, nil)
	request.Header.Set("Cookie", w.cookie)
	request.Header.Set("Referer", "https://www.bilibili.com")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		slog.Error("Error get DanmuInfo.", "Error", err)
	}

	// 解析响应
	var danmuInfoRes DanmuInfoResponse
	json.NewDecoder(response.Body).Decode(&danmuInfoRes)

	defer response.Body.Close()

	if danmuInfoRes.Code != 0 {
		slog.Error("Error fetching DanmuInfo", "Code", danmuInfoRes.Code)
	}

	return danmuInfoRes.Data.Token, danmuInfoRes.Data.HostList
}

// 发送认证包
func (w LiveWatcher) sendAuthPacket() {
	type AuthPayload struct {
		UID      int    `json:"uid"`      // 游客传 0
		RoomID   int    `json:"roomid"`   // 房间号
		ProtoVer int    `json:"protover"` // 建议传 3 (支持最全)
		Platform string `json:"platform"` // 固定 "web"
		Type     int    `json:"type"`     // 固定 2
		Key      string `json:"key"`      // Token
		BuvID    string `json:"buvid"`    // buvid3
	}
	// 解析Cookie
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Cookie", w.cookie)

	DedeUserID, _ := req.Cookie("DedeUserID")
	uid, _ := strconv.ParseInt(DedeUserID.Value, 10, 64)
	buvid3, _ := req.Cookie("buvid3")

	payload := AuthPayload{
		UID:      int(uid),
		RoomID:   w.roomID,
		ProtoVer: 3,
		Platform: "web",
		Type:     2,
		Key:      w.Token,
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

	var err error
	err = w.conn.WriteMessage(websocket.BinaryMessage, packet.Bytes())
	if err != nil {
		slog.Error("Error send auth.", "Error", err)
	} else {
		slog.Info("认证包已发送")
	}
}

// 发送心跳包
func (w LiveWatcher) sendHeartbeatPacket() {

	heartbeatPacket := make([]byte, 16)
	binary.BigEndian.PutUint32(heartbeatPacket[0:4], 16)  // 总长度
	binary.BigEndian.PutUint16(heartbeatPacket[4:6], 16)  // 头部长度
	binary.BigEndian.PutUint16(heartbeatPacket[6:8], 1)   // 协议版本
	binary.BigEndian.PutUint32(heartbeatPacket[8:12], 2)  // 操作码: 2 (Heartbeat)
	binary.BigEndian.PutUint32(heartbeatPacket[12:16], 1) // 序列号

	// 发送心跳包的协程
	ticker := time.NewTicker(30 * time.Second) // 每30秒发送一次心跳包
	defer ticker.Stop()

	// 循环发送
	for range ticker.C {
		err := w.conn.WriteMessage(websocket.BinaryMessage, heartbeatPacket)
		if err != nil {
			slog.Error("Send Hearbeat Error", "err", err)
			return // 发生错误跳出循环，协程结束
		} else {
			slog.Info("心跳包已发送")
		}
	}
}
