package notifier

import (
	"log/slog"
	"net/http"
)

type Notifier struct {
	Type     string
	Notifier interface{}
}

type Bark struct {
	Servers  []string
	Title    string
	BodyText string
	URL      string
	Icon     string
}

// https://api.day.app/eM29e9imSrnjb34yiJs2DZ/xxx开播啦！/直播间标题?icon=https://i2.hdslb.com/bfs/face/8829c1a20fdf7bdf8c388425b883d1b94233be04.jpg&url=bilibili://live/15122413

func (b Bark) Send() {
	for _, server := range b.Servers {
		// url := "https://api.day.app/" + server + "/" + b.Title + "/" + b.BodyText + "?icon=" + b.Icon + "&url=" + b.URL
		url := server + "/" + b.Title + "/" + b.BodyText + "?icon=" + b.Icon + "&url=" + b.URL
		slog.Info("消息已发送", "URL", url)
		http.Get(url)
	}
}
