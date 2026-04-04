package manager

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/HakaseZ/BiliLiveWatcher/model"
	"github.com/HakaseZ/BiliLiveWatcher/notifier"
)

func Manager(c chan model.Metric, nconfig model.LiveWatcher) {
	var n notifier.Bark
	n.Servers = nconfig.Notifier.Bark
	var onStream bool
	onStream = false
	for m := range c {
		switch m.Mtype {
		case "LIVE":
			if onStream {
				slog.Warn("重复收到开播事件")
			} else {
				onStream = true
				n.Title = fmt.Sprintf("%s开播了！", nconfig.Streamer)
				n.BodyText = ""
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send()
			}
		case "PREPARING":
			if onStream {
				onStream = false
				n.Title = fmt.Sprintf("%s下播了。", nconfig.Streamer)
				n.BodyText = ""
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
