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
	"strings"
	//"github.com/HakaseZ/BiliLiveWatcher/utils/network"
)

type NavResponse struct {
	// NavResponse 用于解析nav接口的响应

	Data struct {
		Wbi struct {
			ImgUrl string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

var mixin_shuffle_map = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43,
	5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16,
	24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59,
	6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

func getWbiKeys() string {
	// 通过访问nav接口获取wbi_img字段中的img_url和sub_url，解析出其中的imgKey和subKey并拼接为wbiKey

	response, err := http.Get("https://api.bilibili.com/x/web-interface/nav")
	if err != nil {
		//handle err
	}
	var navRes NavResponse
	json.NewDecoder(response.Body).Decode(&navRes)
	defer response.Body.Close()
	wbiKey := extract_Nav_Key(navRes.Data.Wbi.ImgUrl) + extract_Nav_Key(navRes.Data.Wbi.SubURL)

	return wbiKey
}

func get_Wbi_Mixin_Key(wbiKey string) string {
	// 根据List中的索引顺序，从wbiKey中提取对应的字符并拼接成mixinKey

	var mixinKey strings.Builder
	for _, v := range mixin_shuffle_map {
		mixinKey.WriteByte(wbiKey[v])
	}
	return mixinKey.String()[:32]
}

func extract_Nav_Key(rawUrl string) string {
	// 解析Nav接口返回的img_url和sub_url，提取其中的文件名（不带后缀）作为key

	parts := strings.Split(rawUrl, "/")    // 按斜杠分割字符串
	filename := parts[len(parts)-1]        // 取最后一部分，即 "文件名.后缀"
	return strings.Split(filename, ".")[0] // 按点分割，取第一部分即文件名
}

func main() {
	fmt.Println("Wbi_Mixin_Key: ", get_Wbi_Mixin_Key(getWbiKeys()))
}

// 测试使用上述wbiMixinKey拿取指定的直播间的弹幕服务器信息

func get_Chat_Info(roomId string) {
	params := url.Values{}
	params.Add("id", 15122413)
	params.Add("type", 0)
	params.Add("web_location", "444.8")
	response, err := http.Get("https://api.live.bilibili.com/xlive/web-room/v1/index/getDanmuInfo?room_id=" + roomId + "&wbi_key=" + get_Wbi_Mixin_Key(getWbiKeys()))
	if err != nil {
		//handle err
	}
}
