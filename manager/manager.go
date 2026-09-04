package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/HakaseZ/BiliLiveWatcher/model"
	"github.com/HakaseZ/BiliLiveWatcher/notifier"
)

// Manager 消费直播间事件通道并发送对应通知
//
// Args:
//
//	ctx (context.Context): 通知请求上下文,取消时中止在途通知发送
//	c (chan model.Metric): 直播间事件通道
//	nconfig (model.LiveWatcher): 直播间与通知配置
func Manager(ctx context.Context, c chan model.Metric, nconfig model.LiveWatcher) {
	var n notifier.Bark
	n.Servers = nconfig.Notifier.Bark
	onStream, streamerUID, lastTitle := room_get_info(nconfig.Roomid)
	streamerName, streamerIcon := get_master_info(streamerUID)
	for m := range c {
		switch m.Mtype {
		case "LIVE":
			if onStream {
				slog.Warn("重复收到开播事件", "主播名称", streamerName)
			} else {
				onStream = true
				// 配置关闭开播通知时仅同步流状态,不发送通知
				if !nconfig.Event.LIVE {
					break
				}
				n.Icon = streamerIcon
				n.Title = fmt.Sprintf("%s开播了", streamerName)
				_, _, n.BodyText = room_get_info(nconfig.Roomid)
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title, "直播间标题", n.BodyText)
				n.Send(ctx)
			}
		case "PREPARING":
			if onStream {
				onStream = false
				// 配置关闭下播通知时仅同步流状态,不发送通知
				if !nconfig.Event.PREPARING {
					break
				}
				n.Icon = streamerIcon
				n.Title = fmt.Sprintf("%s下播了", streamerName)
				_, _, n.BodyText = room_get_info(nconfig.Roomid)
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send(ctx)
			} else {
				slog.Warn("重复收到下播事件", "主播名称", streamerName)
			}
		case "INTERACT_WORD_V2":
			msg := m.Msg.(model.EnterRoomMsg)
			if slices.Contains(nconfig.Event.INTER, msg.User.UID) {
				n.Icon = msg.Face
				n.Title = fmt.Sprintf("%s来了", msg.Username)
				n.BodyText = fmt.Sprintf("%s进入了%s的直播间。", msg.Username, streamerName)
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title, "用户名", msg.Username)
				n.Send(ctx)
			}
		case "DANMU_MSG":
			msg := m.Msg.(model.DanmuMsg)
			if slices.Contains(nconfig.Event.DANMU_MSG, msg.User.UID) {
				n.Icon = msg.Face
				n.Title = fmt.Sprintf("%s在%s的直播间发了一条弹幕", msg.Username, streamerName)
				n.BodyText = msg.Danmu
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title, "弹幕内容", msg.Danmu)
				n.Send(ctx)
			}
		case "ROOM_CHANGE":
			msg := m.Msg.(model.RoomChangeMsg)
			if msg.Title == "" || msg.Title == lastTitle {
				break
			}
			lastTitle = msg.Title
			// 配置关闭开播通知(LIVE=false)或房间未开播(PREPARING)时不发送标题变更通知
			if !nconfig.Event.LIVE || !onStream {
				break
			}
			n.Icon = streamerIcon
			n.Title = fmt.Sprintf("%s正在直播", streamerName)
			n.BodyText = msg.Title
			n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
			slog.Info("发送通知", "通知内容", n.Title, "新标题", msg.Title)
			n.Send(ctx)
		}
	}
}

// room_get_info 查询直播间当前状态
//
// Args:
//
//	roomid (int): 直播间ID
//
// Returns:
//
//	isLive (bool): 是否正在直播
//	uid (int): 主播UID
//	title (string): 直播间标题
func room_get_info(roomid int) (bool, int, string) {
	type RoomGetInfoResponse struct {
		Code int `json:"code"`
		Data struct {
			LiveStatus int    `json:"live_status"`
			Title      string `json:"title"`
			UID        int    `json:"uid"`
		} `json:"data"`
	}
	baseURL := "https://api.live.bilibili.com/room/v1/Room/get_info"
	query := url.Values{}
	sid := strconv.Itoa(roomid)
	query.Add("room_id", sid)
	reqURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())
	request, _ := http.NewRequest("GET", reqURL, nil)
	request.Header.Set("Referer", "https://www.bilibili.com")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		slog.Error("Error room get info. ", "Errors", err)
		return false, 0, ""
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		slog.Error("Room get info returned non-2xx status. ", "StatusCode", response.StatusCode)
		return false, 0, ""
	}

	var roomInfoRes RoomGetInfoResponse
	if err := json.NewDecoder(response.Body).Decode(&roomInfoRes); err != nil {
		slog.Error("Decode room get info failed. ", "Errors", err)
		return false, 0, ""
	}
	if roomInfoRes.Code != 0 {
		slog.Error("Room get info returned error code. ", "Code", roomInfoRes.Code)
		return false, 0, ""
	}

	return roomInfoRes.Data.LiveStatus == 1, roomInfoRes.Data.UID, roomInfoRes.Data.Title
}

// get_master_info 查询主播名称与头像
//
// Args:
//
//	uid (int): 主播UID
//
// Returns:
//
//	username (string): 主播用户名
//	face (string): 主播头像URL
func get_master_info(uid int) (string, string) {
	type GetMasterInfoResponse struct {
		Code int `json:"code"`
		Data struct {
			Info struct {
				UID      int    `json:"uid"`
				Username string `json:"uname"`
				Face     string `json:"face"`
			} `json:"info"`
		} `json:"data"`
	}
	baseURL := "https://api.live.bilibili.com/live_user/v1/Master/info"
	query := url.Values{}
	sid := strconv.Itoa(uid)
	query.Add("uid", sid)
	reqURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())
	request, _ := http.NewRequest("GET", reqURL, nil)
	request.Header.Set("Referer", "https://www.bilibili.com")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		slog.Error("Error get anchor in room info. ", "Errors", err)
		return "", ""
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		slog.Error("Master info returned non-2xx status. ", "StatusCode", response.StatusCode)
		return "", ""
	}

	var masterInfoRes GetMasterInfoResponse
	if err := json.NewDecoder(response.Body).Decode(&masterInfoRes); err != nil {
		slog.Error("Decode master info failed. ", "Errors", err)
		return "", ""
	}
	if masterInfoRes.Code != 0 {
		slog.Error("Master info returned error code. ", "Code", masterInfoRes.Code)
		return "", ""
	}
	return masterInfoRes.Data.Info.Username, masterInfoRes.Data.Info.Face
}
