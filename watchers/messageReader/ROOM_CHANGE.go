package message

import "github.com/HakaseZ/BiliLiveWatcher/model"

// deal_ROOM_CHANGE 解析 ROOM_CHANGE 消息,提取直播间 ID 与标题
func deal_ROOM_CHANGE(msg map[string]interface{}) model.RoomChangeMsg {
	var m model.RoomChangeMsg
	if data, ok := msg["data"].(map[string]interface{}); ok {
		if roomID, ok := data["room_id"].(float64); ok {
			m.RoomID = int64(roomID)
		}
		if title, ok := data["title"].(string); ok {
			m.Title = title
		}
	}
	return m
}