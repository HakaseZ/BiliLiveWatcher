package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"time"

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
	logfile, _ := os.OpenFile(".tmp/BiliLiveWatcher.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	logger := slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, logfile), &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)
	slog.Info("系统初始化完成")
}

func main() {
	// 读取配置
	rConfig, err := os.ReadFile(".config/config.json")
	if err != nil {
		slog.Error("Error load config file.", "Error", err)
	}
	var conf model.Config
	err = json.Unmarshal(rConfig, &conf)
	if err != nil {
		slog.Error("Error Unmarshal config file.", "Error", err)
	}

	// 实例化消息chan

	// 实例化
	for _, wconf := range conf.LiveWatcher {
		// wconf := wconf
		// 去重并注册 Notifier
		// 构造 Watcher-Notifier map
		// 实例化 Watcher 协程
		// var w watchers.LiveWatcher
		watcher := watchers.NewWatcher(wconf.Roomid, conf.Credential.Cookie)
		go watcher.Run()
	}

	select {}
	// 消息处理逻辑

	// for _, watcher := range conf.LiveWatcher {
	// 	go watcher.InitWatcher(conf.Credential.Cookie)
	// }

	// for _, notifier := range conf.Notifier {
	//
	// }
}
