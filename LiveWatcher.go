package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
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

	// 共享根上下文:收到退出信号时可取消所有 watcher
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for _, wconf := range conf.LiveWatcher {
		wg.Add(1)
		go func(wconf model.LiveWatcher) { // 分别启动循环计时器协程
			defer wg.Done()
			for { // 每24小时重启一次监听器
				iterCtx, iterCancel := context.WithCancel(ctx)
				ticker := time.NewTicker(24 * time.Hour)
				c := make(chan model.Metric)
				w := watchers.NewWatcher(wconf.Roomid, conf.Credential.Cookie)

				var inner sync.WaitGroup
				inner.Add(2)
				go func() { defer inner.Done(); manager.Manager(iterCtx, c, wconf) }()
				go func() { defer inner.Done(); w.Run(iterCtx, c) }()

				select {
				case <-ticker.C: // 24小时到期,重启监听器
					slog.Info("正在重新启动监听器", "RoomID", wconf.Roomid)
					iterCancel()
					inner.Wait() // 等待当前 watcher 与 manager 退出后再重启
					ticker.Stop()
					if ctx.Err() != nil { // 取消与到期同时发生时不再重启
						return
					}
				case <-ctx.Done(): // 收到退出信号,结束循环
					iterCancel()
					inner.Wait() // 等待 watcher 退出(关闭channel)与 manager 排空
					ticker.Stop()
					return
				}
			}
		}(wconf)
	}

	// 等待退出信号(SIGINT/SIGTERM):取消所有 watcher,全部结束后关闭捕获
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	cancel()
	wg.Wait()
	capture.Close()
}
