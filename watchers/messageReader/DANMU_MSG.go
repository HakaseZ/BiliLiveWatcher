package message

import (
	"encoding/json"
	"log/slog"

	"github.com/HakaseZ/BiliLiveWatcher/model"
	"github.com/mitchellh/mapstructure"
)

type MessageData struct {
	UserInfo struct {
		BaseInfo struct {
			Face string `json:"face"`
			Name string `json:"name"`
		} `json:"base"`
		UID float64 `json:"uid"`
	} `json:"user"`
}

type DANMU_MSG struct {
	Cmd   string        `mapstructure:"cmd"`
	Dm_v2 string        `mapstructure:"dm_v2"`
	Info  []interface{} `mapstructure:"info"`
}

func deal_DANMU_MSG(msg map[string]interface{}) model.DanmuMsg {
	var body DANMU_MSG
	mapstructure.Decode(msg, &body)

	data15 := body.Info[0].([]interface{})[15].(map[string]interface{})
	dataJson, err := json.Marshal(data15)
	if err != nil {
		slog.Error("Failed to marshal DANMU_MSG data", "Error", err)
	}
	var dmsg MessageData
	json.Unmarshal(dataJson, &dmsg)
	if err != nil {
		slog.Error("Failed to unmarshal DANMU_MSG data", "Error", err)
	}
	var m model.DanmuMsg
	m.Danmu = body.Info[1].(string)
	m.User.UID = int64(dmsg.UserInfo.UID)
	m.User.Username = dmsg.UserInfo.BaseInfo.Name
	m.User.Face = dmsg.UserInfo.BaseInfo.Face

	// slog.Info("DANMU_MSG", "username", dmsg.UserInfo.BaseInfo.Name, "UID", int64(dmsg.UserInfo.UID), "face", dmsg.UserInfo.BaseInfo.Face, "danmu", body.Info[1])

	return m
}
