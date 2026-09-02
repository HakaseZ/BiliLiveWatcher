package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/HakaseZ/BiliLiveWatcher/capture"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	slog.Info("系统初始化完成")
}

func main() {
	// 读取配置
	rConfig, err := os.ReadFile("config.json")
	if err != nil {
		slog.Error("Error load config file.", "Error", err)
		os.Exit(1)
	}
	var conf model.Config
	err = json.Unmarshal(rConfig, &conf)
	if err != nil {
		slog.Error("Error Unmarshal config file.", "Error", err)
		os.Exit(1)
	}

	// 实例化监听器
	// 根据配置设置日志级别,默认 Info(生产环境避免刷屏)
	level := slog.LevelInfo
	switch strings.ToLower(conf.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	// 初始化原始数据包捕获
	capture.Init(conf.Capture.Path, conf.Capture.Enabled, conf.Capture.MaxSizeMB, conf.Capture.MaxBackups)

	// 	for {
	// 		ticker := time.NewTicker(24 * time.Hour)
	// 		ctx, cancel := context.WithCancel(context.Background())
	// 		for _, wconf := range conf.LiveWatcher {
	//
	// 			// 1.初始化chan
	// 			// 2.初始化Manager
	// 			// 3.初始化Watcher
	// 			c := make(chan model.Metric)
	// 			// wconf := wconf
	// 			// 去重并注册 Notifier
	// 			// 构造 Watcher-Notifier map
	// 			// 实例化 Watcher 协程
	// 			// var w watchers.LiveWatcher
	// 			go manager.Manager(c, wconf)
	// 			watcher := watchers.NewWatcher(wconf.Roomid, conf.Credential.Cookie)
	// 			go watcher.Run(ctx, c)
	// 		}
	// 		<-ticker.C
	// 		slog.Info("正在重新启动监听器")
	// 		cancel()
	// 		ticker.Stop()
	// 		// time.Sleep(5 * time.Second) // 等待5秒钟，确保所有监听器都已关闭
	// 	}

	// #12

	for _, wconf := range conf.LiveWatcher {
		go func() { // 分别启动循环计时器协程
			for { // 每24小时重启一次监听器
				ticker := time.NewTicker(24 * time.Hour)
				ctx, cancel := context.WithCancel(context.Background())
				c := make(chan model.Metric)
				// m := manager.NewManager()
				w := watchers.NewWatcher(wconf.Roomid, conf.Credential.Cookie)
				// go m.Run()

				go manager.Manager(c, wconf)
				go w.Run(ctx, c)

				<-ticker.C // 阻塞等待24小时
				slog.Info("正在重新启动监听器", "RoomID", wconf.Roomid)
				cancel() // 取消当前监听器的上下文，通知其退出，进入下个循环重新启动监听器
				ticker.Stop()
			}
		}()
	}

	// 等待退出信号(SIGINT/SIGTERM),优雅关闭捕获后退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	capture.Close()
}
