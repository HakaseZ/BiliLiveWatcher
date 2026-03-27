package message

import (
	"log/slog"

	"github.com/mitchellh/mapstructure"
)

type DANMU_MSG struct {
	Cmd   string        `mapstructure:"cmd"`
	Dm_v2 string        `mapstructure:"dm_v2"`
	Info  []interface{} `mapstructure:"info"`
}

func deal_DANMU_MSG(msg map[string]interface{}) {
	var body DANMU_MSG
	mapstructure.Decode(msg, &body)

	username := body.Info[2].([]interface{})[1]
	UID := int64(body.Info[2].([]interface{})[0].(float64))

	slog.Info("DANMU_MSG", "username", username, "UID", UID, "danmu", body.Info[1])
}
