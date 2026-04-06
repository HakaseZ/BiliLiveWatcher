package manager

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/HakaseZ/BiliLiveWatcher/model"
	"github.com/HakaseZ/BiliLiveWatcher/notifier"
)

func Manager(c chan model.Metric, nconfig model.LiveWatcher) {
	var n notifier.Bark
	n.Servers = nconfig.Notifier.Bark
	onStream, streamerName, streamerIcon, _ := room_get_info(nconfig.Roomid)
	for m := range c {
		switch m.Mtype {
		case "LIVE":
			if nconfig.Event.LIVE {
				if onStream {
					slog.Warn("重复收到开播事件")
				} else {
					onStream = true
					n.Icon = streamerIcon
					n.Title = fmt.Sprintf("%s开播了！", streamerName)
					_, _, _, n.BodyText = room_get_info(nconfig.Roomid)
					n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
					slog.Info("发送通知", "通知内容", n.Title)
					n.Send()
				}
			}
		case "PREPARING":
			if nconfig.Event.PREPARING {
				if onStream {
					onStream = false
					n.Icon = ""
					n.Title = fmt.Sprintf("%s下播了。", streamerName)
					_, _, _, n.BodyText = room_get_info(nconfig.Roomid)
					n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
					slog.Info("发送通知", "通知内容", n.Title)
					n.Send()
				} else {
					slog.Warn("重复收到下播事件")
				}
			}
		case "INTERACT_WORD_V2":
			msg := m.Msg.(model.EnterRoomMsg)
			if slices.Contains(nconfig.Event.INTER, msg.User.UID) {
				n.Icon = msg.Face
				n.Title = fmt.Sprintf("%s进入了%s的直播间。", msg.Username, nconfig.Streamer)
				n.BodyText = ""
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send()
			}
		case "DANMU_MSG":
			msg := m.Msg.(model.DanmuMsg)
			if slices.Contains(nconfig.Event.DANMU_MSG, msg.User.UID) {
				n.Icon = msg.Face
				n.Title = fmt.Sprintf("%s在%s的直播间发了一条弹幕", msg.Username, nconfig.Streamer)
				n.BodyText = msg.Danmu
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send()
			}
		}
	}
}

func room_get_info(roomid int) (bool, string, string, string) {
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

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		slog.Error("Error room get info. ", "Errors", err)
	}

	var roomInfoRes RoomGetInfoResponse
	json.NewDecoder(response.Body).Decode(&roomInfoRes)
	defer response.Body.Close()

	type GetAnchorInRoomResponse struct {
		Code int `json:"code"`
		Info struct {
			UID      int    `json:"uid"`
			Username string `json:"uname"`
			Face     string `json:"face"`
		} `json:"info"`
	}
	baseURL = "https://api.live.bilibili.com/room/v1/Room/get_anchor_in_room"
	query = url.Values{}
	query.Add("room_id", sid)
	reqURL = fmt.Sprintf("%s?%s", baseURL, query.Encode())
	request, _ = http.NewRequest("GET", reqURL, nil)
	request.Header.Set("Referer", "https://www.bilibili.com")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	response, err = client.Do(request)
	if err != nil {
		slog.Error("Error get anchor in room info. ", "Errors", err)
	}

	var anchorInfoRes GetAnchorInRoomResponse
	json.NewDecoder(response.Body).Decode(&anchorInfoRes)
	defer response.Body.Close()
	return roomInfoRes.Data.LiveStatus == 1, anchorInfoRes.Info.Username, anchorInfoRes.Info.Face, roomInfoRes.Data.Title
}
