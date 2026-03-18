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
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
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

func getDanmuInfo(roomId string) {
	/*
		生成请求参数，调用encWbi函数进行加密，发送请求并解析响应
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
	params["wts"] = fmt.Sprintf("%d", time.Now().Unix())

	w_rid := encWbi(params) // 调用encWbi函数生成w_rid并添加到参数中
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

func encWbi(params map[string]interface{}) string {

	/*
		实现WBI加密算法，生成w_rid并添加到params中

		Args:
			params (map[string]interface{}) 请求的原始参数

		Returns:
			w_rid (string): WBI加密生成的签名参数
	*/

	// 按key重排参数并过滤value中的 "!'()*" 字符
	params = filterParams(params)

	query := url.Values{}
	for key, value := range params {
		query.Add(key, fmt.Sprintf("%v", value))
	}
	w_rid := fmt.Sprintf("%x", md5.Sum([]byte(query.Encode()+get_Wbi_Mixin_Key(getWbiKeys()))))

	return w_rid
}

func filterParams(params map[string]interface{}) map[string]interface{} {
	// 按key重排参数并过滤value中的 "!'()*" 字符

	filteredParams := make(map[string]interface{})
	for key, value := range params {
		strValue := fmt.Sprintf("%v", value)
		strValue = strings.ReplaceAll(strValue, "!", "")
		strValue = strings.ReplaceAll(strValue, "'", "")
		strValue = strings.ReplaceAll(strValue, "(", "")
		strValue = strings.ReplaceAll(strValue, ")", "")
		strValue = strings.ReplaceAll(strValue, "*", "")
		filteredParams[key] = strValue
	}
	return filteredParams
}

func main() {
	getDanmuInfo("15122413") // 测试房间号，可以替换为其他房间ID进行测试
}
