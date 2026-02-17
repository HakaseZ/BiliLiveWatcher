// 构造Https请求进行握手，并连接websocket弹幕服务器

// 测试房间号
// 92613 API
// uri = wss://host.port/sub
// getDanmuInfo_uri = https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo
// getkeys_uri = https://api.bilibili.com/x/web-interface/nav

// 步骤：通过KeysUrl获取WbiKeys
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type navResponse struct {
	Data struct {
		Wbi struct {
			ImgUrl string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

func getWbiKeys() (string, string) {
	response, err := http.Get("https://api.bilibili.com/x/web-interface/nav")
	if err != nil {
		//handle err
	}
	var navRes navResponse
	json.NewDecoder(response.Body).Decode(&navRes)
	defer response.Body.Close()
	imgKey := extractKey(navRes.Data.Wbi.ImgUrl)
	subKey := extractKey(navRes.Data.Wbi.SubURL)
	return imgKey, subKey
}

func extractKey(rawUrl string) string {
	parts := strings.Split(rawUrl, "/")    // 按斜杠分割字符串
	filename := parts[len(parts)-1]        // 取最后一部分，即 "文件名.后缀"
	return strings.Split(filename, ".")[0] // 按点分割，取第一部分即文件名
}

func main() {
	imgKey, subKey := getWbiKeys()
	fmt.Println("imgKey: ", imgKey)
	fmt.Println("subKey: ", subKey)
}
