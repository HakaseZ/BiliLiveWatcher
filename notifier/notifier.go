package notifier

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
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

// 带超时的客户端,避免通知请求卡住阻塞整个事件处理链
var httpClient = &http.Client{Timeout: 10 * time.Second}

func (b Bark) Send() {
	for _, server := range b.Servers {
		url := fmt.Sprintf("%s/%s/%s?icon=%s&url=%s", server, url.PathEscape(b.Title), url.PathEscape(b.BodyText), url.QueryEscape(b.Icon), url.QueryEscape(b.URL))
		slog.Info("消息已发送", "URL", url)
		resp, err := httpClient.Get(url)
		if err != nil {
			slog.Error("发送通知失败", "Error", err, "Server", server)
			continue
		}
		resp.Body.Close()
	}
}
