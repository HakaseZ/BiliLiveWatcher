package notifier

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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

func (b Bark) Send() {
	for _, server := range b.Servers {
		url := fmt.Sprintf("%s/%s/%s?icon=%s&url=%s", server, url.PathEscape(b.Title), url.PathEscape(b.BodyText), url.QueryEscape(b.Icon), url.QueryEscape(b.URL))
		slog.Info("消息已发送", "URL", url)
		http.Get(url)
	}
}
