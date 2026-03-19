/*
构造Https请求进行握手，并连接websocket弹幕服务器

测试房间号
92613 API
uri = wss://host.port/sub
getDanmuInfo_uri = https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo
nav_uri = https://api.bilibili.com/x/web-interface/nav

步骤：通过KeysUrl获取WbiKeys
*/
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	wbi "github.com/HakaseZ/BiliLiveWatcher/utils"
)

func getDanmuInfo(roomId string) {
	/*
		生成请求参数，调用EncWbi函数进行加密，发送请求并解析响应
		请求URL：https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo

		Args:
			roomId (string): 直播间ID

		Returns:
			danmuInfo (DanmuInfo): 包含弹幕服务器地址和其他相关信息的结构体
	*/
	params := make(map[string]interface{})
	params["id"] = roomId
	params["type"] = "0"
	params["web_location"] = "444.8"

	params["wts"] = fmt.Sprintf("%d", time.Now().Unix()) // 添加时间戳

	w_rid := wbi.EncWbi(params) // 调用encWbi函数生成w_rid并添加到参数中
	params["w_rid"] = w_rid

	query := url.Values{}
	for key, value := range params {
		query.Add(key, fmt.Sprintf("%v", value))
	}
	// 构造请求URL和参数
	baseURL := "https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo"
	reqURL := fmt.Sprintf("%s?%s", baseURL, query.Encode())

	fmt.Println("Request URL: ", reqURL) // 打印请求URL以供调试

	// 构造请求头
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		// handle error
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// 发送HTTP GET请求
	response, err := http.Get(reqURL)
	if err != nil {
		// handle error
	}
	defer response.Body.Close()

	// 测试逻辑，解析响应并打印结果
	var danmuInfoResponse struct {
		Code int `json:"code"`
		Data struct {
			Token    string `json:"token"`
			HostList []struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			} `json:"host_list"`
		} `json:"data"`
	}
	json.NewDecoder(response.Body).Decode(&danmuInfoResponse)
	if danmuInfoResponse.Code == 0 {
		fmt.Println("Token: ", danmuInfoResponse.Data.Token)
		for _, host := range danmuInfoResponse.Data.HostList {
			fmt.Printf("Host: %s, Port: %d\n", host.Host, host.Port)
		}
	} else {
		fmt.Printf("Error fetching danmu info, code: %d\n", danmuInfoResponse.Code)
	}
}

func main() {
	getDanmuInfo("15122413") // 测试房间号，可以替换为其他房间ID进行测试
}
