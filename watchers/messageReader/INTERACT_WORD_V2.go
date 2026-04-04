package message

import (
	"encoding/base64"
	"log/slog"

	"github.com/HakaseZ/BiliLiveWatcher/model"
	mpb "github.com/HakaseZ/BiliLiveWatcher/watchers/messageReader/pb"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/proto"
)

type INTERACT_WORD_V2 struct {
	Cmd  string `mapstructure:"cmd"`
	Data struct {
		Dmscore int    `mapstructure:"dmscore"`
		Pb      string `mapstructure:"pb"`
	} `mapstructure:"data"`
}

func deal_INTERACT_WORD_V2(msg map[string]interface{}) model.EnterRoomMsg {
	var body INTERACT_WORD_V2
	mapstructure.Decode(msg, &body)

	pb, _ := base64.StdEncoding.DecodeString(body.Data.Pb)
	info := &mpb.INTERACT_WORD_V2{}
	proto.Unmarshal(pb, info)

	username := info.UserInfo.Base.Uname
	UID := int64(info.Uid)

	var m model.EnterRoomMsg
	m.User.UID = UID
	m.User.Username = username
	m.User.Face = info.UserInfo.Base.Face

	slog.Info("INTERACT_WORD_V2", "username", username, "UID", UID, "face", info.UserInfo.Base.Face)

	return m
}
