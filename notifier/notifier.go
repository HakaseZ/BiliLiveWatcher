package notifier

import (
	"context"
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

func (b Bark) Send(ctx context.Context) {
	for _, server := range b.Servers {
		url := fmt.Sprintf("%s/%s/%s?icon=%s&url=%s", server, url.PathEscape(b.Title), url.PathEscape(b.BodyText), url.QueryEscape(b.Icon), url.QueryEscape(b.URL))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			slog.Error("构造通知请求失败", "Error", err)
			continue
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			slog.Error("发送通知失败", "Error", err, "Server", server)
			continue
		}
		if err := resp.Body.Close(); err != nil {
			slog.Error("关闭通知响应失败", "Error", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			slog.Error("发送通知返回异常状态码", "StatusCode", resp.StatusCode, "Server", server)
			continue
		}
		slog.Info("消息已发送", "URL", url)
	}
}
