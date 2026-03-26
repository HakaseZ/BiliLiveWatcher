package message

import (
	"encoding/base64"
	"log/slog"
	"os"
	"time"

	"github.com/mitchellh/mapstructure"
)

type INTERACT_WORD_V2 struct {
	Cmd  string `mapstructure:"cmd"`
	Data struct {
		Dmscore int    `mapstructure:"dmscore"`
		Pb      string `mapstructure:"pb"`
	} `mapstructure:"data"`
}

func init() {
	// 初始化时间设置
	var err error
	time.Local, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		time.Local = time.FixedZone("CST", 8*3600)
	}

	//初始化全局logger
	debuglogfile, _ := os.OpenFile(".tmp/debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	debugLogger = slog.New(slog.NewTextHandler(debuglogfile, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func deal_INTERACT_WORD_V2(msg map[string]interface{}) {
	var body INTERACT_WORD_V2
	mapstructure.Decode(msg, &body)

	pb, _ := base64.StdEncoding.DecodeString(body.Data.Pb)
	debugLogger.Debug("INTERACT_WORD_V2", "pb", string(pb))
}
