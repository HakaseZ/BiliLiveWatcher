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

	for m := range c {

		switch m.Mtype {
		case "LIVE":
			msg := m.Msg.(model.LiveMsg)
			n.Title = fmt.Sprintf("直播间%d开播了！", msg.RoomID)
			n.BodyText = ""
			n.URL = fmt.Sprintf("bilibili://live/%d", msg.RoomID)
			n.Send()
		case "PREPARING":
		case "INTERACT_WORD_V2":
			msg := m.Msg.(model.EnterRoomMsg)
			if slices.Contains(nconfig.Event.INTER, msg.User.UID) {
				n.Icon = msg.Face
				n.Title = fmt.Sprintf("%s进入了直播间%d", msg.Username, m.M_RoomID)
				n.BodyText = ""
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send()
			}
		case "DANMU_MSG":
			msg := m.Msg.(model.DanmuMsg)
			if slices.Contains(nconfig.Event.DANMU_MSG, msg.User.UID) {
				n.Icon = msg.Face
				n.Title = fmt.Sprintf("%s在直播间%d发了一条弹幕", msg.Username, m.M_RoomID)
				n.BodyText = msg.Danmu
				n.URL = fmt.Sprintf("bilibili://live/%d", m.M_RoomID)
				slog.Info("发送通知", "通知内容", n.Title)
				n.Send()
			}
		}
	}
	// m := <-c
	// switch m.Type {
	// case "INTERACT_WORD_V2":
	//
	// case "LIVE":
	// case "PREPARING":
	// case "DANMU_MSG":
	// }
}
