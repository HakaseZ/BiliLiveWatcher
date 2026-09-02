package manager

import (
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

func Manager(c chan model.Metric, nconfig model.LiveWatcher) {
	var n notifier.Bark
	n.Servers = nconfig.Notifier.Bark
	onStream, streamerUID, lastTitle := room_get_info(nconfig.Roomid)
	streamerName, streamerIcon := get_master_info(streamerUID)
	for m := range c {
		switch m.Mtype {
		case "LIVE":
			if onStream {
				slog.Warn("重复收到开播事件")
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
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send()
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
				n.Send()
			} else {
				slog.Warn("重复收到下播事件")
			}
		case "INTERACT_WORD_V2":
			msg := m.Msg.(model.EnterRoomMsg)
			if slices.Contains(nconfig.Event.INTER, msg.User.UID) {
				n.Icon = msg.Face
				n.Title = fmt.Sprintf("%s来了", msg.Username)
				n.BodyText = fmt.Sprintf("%s进入了%s的直播间。", msg.Username, streamerName)
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send()
			}
		case "DANMU_MSG":
			msg := m.Msg.(model.DanmuMsg)
			if slices.Contains(nconfig.Event.DANMU_MSG, msg.User.UID) {
				n.Icon = msg.Face
				n.Title = fmt.Sprintf("%s在%s的直播间发了一条弹幕", msg.Username, streamerName)
				n.BodyText = msg.Danmu
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send()
			}
		case "ROOM_CHANGE":
			msg := m.Msg.(model.RoomChangeMsg)
			if msg.Title == "" || msg.Title == lastTitle {
				break
			}
			// 记录标题变更
			slog.Info("直播间标题变更", "RoomID", nconfig.Roomid, "旧标题", lastTitle, "新标题", msg.Title)
			lastTitle = msg.Title
			// 配置关闭开播通知(LIVE=false)或房间未开播(PREPARING)时不发送标题变更通知
			if !nconfig.Event.LIVE || !onStream {
				break
			}
			n.Icon = streamerIcon
			n.Title = fmt.Sprintf("%s正在直播", msg.Title)
			n.BodyText = fmt.Sprintf("%s", msg.Title)
			n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
			slog.Info("发送通知", "通知内容", n.Title)
			n.Send()
		}
	}
}

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
