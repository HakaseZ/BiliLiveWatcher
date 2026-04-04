package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/HakaseZ/BiliLiveWatcher/manager"
	"github.com/HakaseZ/BiliLiveWatcher/model"
	"github.com/HakaseZ/BiliLiveWatcher/watchers"
)

func init() {
	// 初始化时间设置
	var err error
	time.Local, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		time.Local = time.FixedZone("CST", 8*3600)
	}

	//初始化全局logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	slog.Info("系统初始化完成")
}

func main() {
	// 读取配置
	rConfig, err := os.ReadFile("config.json")
	if err != nil {
		slog.Error("Error load config file.", "Error", err)
	}
	var conf model.Config
	err = json.Unmarshal(rConfig, &conf)
	if err != nil {
		slog.Error("Error Unmarshal config file.", "Error", err)
	}

	// 实例化监听器
	for {
		ticker := time.NewTicker(24 * time.Hour)
		ctx, cancel := context.WithCancel(context.Background())
		for _, wconf := range conf.LiveWatcher {

			// 1.初始化chan
			// 2.初始化Manager
			// 3.初始化Watcher
			c := make(chan model.Metric)
			// wconf := wconf
			// 去重并注册 Notifier
			// 构造 Watcher-Notifier map
			// 实例化 Watcher 协程
			// var w watchers.LiveWatcher
			go manager.Manager(c, wconf)
			watcher := watchers.NewWatcher(wconf.Roomid, conf.Credential.Cookie)
			go watcher.Run(ctx, c)
		}
		<-ticker.C
		slog.Info("正在重新启动监听器")
		cancel()
		ticker.Stop()
		time.Sleep(5 * time.Second) // 等待5秒钟，确保所有监听器都已关闭
	}
	select {}
}
