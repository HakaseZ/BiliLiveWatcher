package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Host struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	WSSPort int    `json:"wss_port"`
	WSPort  int    `json:"ws_port"`
}

func GetDanmuInfo(roomID int) (string, []Host, error) {
	// 获取弹幕服务器配置
	//
	// Args:
	// 	roomID (int64): 直播间ID
	//
	// Returns:
	// 	Token (string): 连接弹幕服务器的Token
	// 	HostList ([]Host): 弹幕服务器列表，包含Host、Port等信息

	type danmuInfoResponse struct {
		Code int `json:"code"`
		Data struct {
			Token    string `json:"token"`
			HostList []Host `json:"host_list"`
		} `json:"data"`
	}

	params := make(map[string]interface{})
	params["id"] = roomID
	params["type"] = "0"
	params["web_location"] = "444.8"
	params["wts"] = fmt.Sprintf("%d", time.Now().Unix()) // 添加时间戳

	w_rid := EncWbi(params) // 调用EncWbi函数生成w_rid并添加到参数中
	params["w_rid"] = w_rid

	query := url.Values{}
	for key, value := range params {
		query.Add(key, fmt.Sprintf("%v", value))
	}
	// 构造请求URL和参数
	baseURL := "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo"
	reqURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	fmt.Println("Request URL: ", reqURL) // 打印请求URL以供调试

	// 发送HTTP请求

	request, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		// handle error
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	request.Header.Set("Referer", "https://www.bilibili.com")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		// handle error
	}

	// 解析响应
	var danmuInfoRes danmuInfoResponse
	json.NewDecoder(response.Body).Decode(&danmuInfoRes)

	defer response.Body.Close()

	if danmuInfoRes.Code == 0 {
		fmt.Println("Token: ", danmuInfoRes.Data.Token)
		for _, host := range danmuInfoRes.Data.HostList {
			fmt.Printf("Host: %s, Port: %d, WSSPort: %d, WSPort: %d\n", host.Host, host.Port, host.WSSPort, host.WSPort)
		}
	} else {
		fmt.Printf("Error fetching danmu info, code: %d\n", danmuInfoRes.Code)
	}

	return danmuInfoRes.Data.Token, danmuInfoRes.Data.HostList, nil
}
