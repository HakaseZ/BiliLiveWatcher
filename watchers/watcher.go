package watchers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/HakaseZ/BiliLiveWatcher/capture"
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
	if w.roomID == 0 || w.UID == 0 {
		slog.Error("获取直播间信息失败,停止初始化监听器", "RoomID", roomid)
		return w
	}
	w.Token, w.wsHosts = w.getDanmuInfo()
	return w
}

func (w LiveWatcher) Run(ctx context.Context, c chan model.Metric) {
	// 初始化失败(房间信息或弹幕服务器配置获取失败)时直接退出,避免空 host 索引 panic
	if w.roomID == 0 || len(w.wsHosts) == 0 {
		slog.Error("监听器初始化失败,停止运行", "RoomID", w.roomID)
		close(c)
		return
	}
	// 断线后指数退避重连:5s → 10s → 20s → 40s → 60s(封顶)
	backoff := 5 * time.Second
	for {
		err := w.runOnce(ctx, c)
		if ctx.Err() != nil {
			slog.Info("WebSocket连接已关闭，停止监听直播间事件", "RoomID", w.roomID)
			close(c) // 监听器退出时关闭通道，通知Manager进行清理
			return
		}
		slog.Error("连接异常断开，等待重连", "Error", err, "RoomID", w.roomID, "退避", backoff)
		select {
		case <-ctx.Done():
			slog.Info("WebSocket连接已关闭，停止监听直播间事件", "RoomID", w.roomID)
			close(c)
			return
		case <-time.After(backoff):
		}
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}

// runOnce 建立一次弹幕服务器连接并监听,连接断开或上下文取消时返回
func (w LiveWatcher) runOnce(ctx context.Context, c chan model.Metric) error {
	wsURL := fmt.Sprintf("wss://%s/sub", w.wsHosts[0].Host) //使用默认服务器
	slog.Info("正在连接弹幕服务器", "RoomID", w.roomID, "URL", wsURL)

	// 建立连接
	header := http.Header{}
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	header.Set("Referer", "https://www.bilibili.com")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("dial ws server: %w", err)
	}
	w.conn = conn
	defer w.conn.Close()

	// 会话级取消:上下文取消或本会话结束时主动关闭连接,解除 ReadMessage 阻塞。
	// 会话结束即终止监听 goroutine,避免其残留到下一会话误关新连接。
	sessionDone := make(chan struct{})
	defer close(sessionDone)
	go func() {
		select {
		case <-ctx.Done():
			w.conn.Close()
		case <-sessionDone:
		}
	}()

	// 发送认证和心跳包
	w.sendAuthPacket()
	hbCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go w.sendHeartbeatPacket(hbCtx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, packet, err := w.conn.ReadMessage()
		if err != nil {
			return err
		}
		capture.Write(packet)
		for _, messgae := range m.ParsePacket(packet) {
			if messgae.Mtype != "" {
				messgae.M_RoomID = int64(w.roomID)
				c <- messgae
			}
		}
	}
}

// 获取直播间详情
func (w LiveWatcher) getRoomPlayInfo(shortId int) (roomId int, uid int) {
	type RooomPlayInfoResponse struct {
		Code int `json:"code"`
		Data struct {
			RoomID     int `json:"room_id"`
			UID        int `json:"uid"`
			LiveStatus int `json:"live_status"`
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

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		slog.Error("Error get room play info. ", "Error", err)
		return 0, 0
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		slog.Error("Get room play info returned HTTP error. ", "StatusCode", response.StatusCode)
		return 0, 0
	}

	// 解析响应
	var danmuInfoRes RooomPlayInfoResponse
	if err := json.NewDecoder(response.Body).Decode(&danmuInfoRes); err != nil {
		slog.Error("Decode room play info failed. ", "Error", err)
		return 0, 0
	}
	if danmuInfoRes.Code != 0 {
		slog.Error("Get room play info returned error code. ", "Code", danmuInfoRes.Code)
		return 0, 0
	}
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
	// 解析Cookie(缺失字段按游客处理,避免 nil 解引用)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Set("Cookie", w.cookie)

	uid := int64(0)
	if DedeUserID, err := req.Cookie("DedeUserID"); err == nil {
		uid, _ = strconv.ParseInt(DedeUserID.Value, 10, 64)
	}
	buvid3 := ""
	if c, err := req.Cookie("buvid3"); err == nil {
		buvid3 = c.Value
	}

	payload := AuthPayload{
		UID:      int(uid),
		RoomID:   w.roomID,
		ProtoVer: 3,
		Platform: "web",
		Type:     2,
		Key:      w.Token,
		BuvID:    buvid3,
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
func (w LiveWatcher) sendHeartbeatPacket(ctx context.Context) {

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

	for {
		select {
		case <-ctx.Done():
			slog.Info("收到停止心跳包的信号，退出心跳包发送协程", "RoomID", w.roomID)
			return
		case <-ticker.C:
			err := w.conn.WriteMessage(websocket.BinaryMessage, heartbeatPacket)
			if err != nil {
				select {
				case <-ctx.Done():
					slog.Info("WebSocket连接已关闭，停止发送心跳包", "RoomID", w.roomID)
				default:
					slog.Error("Send Hearbeat Error", "Error", err, "RoomID", w.roomID)
				}
				w.conn.Close() // 关闭连接,触发主循环感知断开并重连
				return         // 发生错误跳出循环，协程结束
			}
		}
	}
}
