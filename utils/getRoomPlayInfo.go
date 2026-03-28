package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func GetRoomPlayInfo(shortId int) (roomId int, uid int) {
	type RooomPlayInfoResponse struct {
		Code int `json:"code"`
		Data struct {
			RoomID     int  `json:"room_id"`
			UID        int  `json:"uid"`
			LiveStatus bool `json:"live_status"`
		} `json:""`
	}
	baseURL := "https://api.live.bilibili.com/xlive/web-room/v1/index/getRoomPlayInfo"
	query := url.Values{}
	sid := strconv.Itoa(shortId)
	query.Add("room_id", sid)
	reqURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	request, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		// handle error
	}
	request.Header.Set("Referer", "https://www.bilibili.com")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		// handle error
	}

	// 解析响应
	var danmuInfoRes RooomPlayInfoResponse
	json.NewDecoder(response.Body).Decode(&danmuInfoRes)
	defer response.Body.Close()
	return danmuInfoRes.Data.RoomID, danmuInfoRes.Data.UID
}
