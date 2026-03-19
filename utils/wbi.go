package utils

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func EncWbi(params map[string]interface{}) string {
	/*
		实现WBI加密算法，生成w_rid并添加到params中

		Args:
			params (map[string]interface{}) 请求的原始参数

		Returns:
			w_rid (string): WBI加密生成的签名参数
	*/

	params = filterParams(params) // 过滤value中的 "!'()*" 字符

	query := url.Values{} // 构建查询字符串

	for key, value := range params {
		query.Add(key, fmt.Sprintf("%v", value))
	}

	w_rid := fmt.Sprintf("%x", md5.Sum([]byte(query.Encode()+getMixinKey()))) //query.Encode()会自动对参数进行按key排序
	return w_rid
}

func filterParams(params map[string]interface{}) map[string]interface{} {
	/*
		过滤value中的 "!'()*" 字符

		Args:
			params (map[string]interface{}): 原始参数

		Returns:
			filteredParams (map[string]interface{}): 过滤后的参数
	*/
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

type navResponse struct {
	// navResponse 用于解析nav接口的响应

	Data struct {
		Wbi struct {
			ImgUrl string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}

var mixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35, 27, 43,
	5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13, 37, 48, 7, 16,
	24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4, 22, 25, 54, 21, 56, 59,
	6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

func getMixinKey() string {
	/*
		获取WBI加密的mixin_key

		*** 当前的请求不带用户登陆状态，仅能获取匿名用户的mixin_key，可能会导致部分接口请求失败 ***

		请求接口URL：
			https://api.bilibili.com/x/web-interface/nav

		Returns:
			mixin_key (string): 混淆后的mixin_key
	*/

	response, err := http.Get("https://api.bilibili.com/x/web-interface/nav")
	if err != nil {
		//handle err 在日志中记录错误、发起重试、响应异常等
	}
	var navRes navResponse
	json.NewDecoder(response.Body).Decode(&navRes)
	defer response.Body.Close()
	navKey := extractNavKey(navRes.Data.Wbi.ImgUrl) + extractNavKey(navRes.Data.Wbi.SubURL) // 从img_url和sub_url中提取key并拼接

	var mixinKey strings.Builder
	for _, v := range mixinKeyEncTab {
		// 根据List中的索引顺序，从navKey中提取mixinKey
		mixinKey.WriteByte(navKey[v])
	}

	return mixinKey.String()[:32] // 取前32个字符作为最终的mixin_key

}

func extractNavKey(rawUrl string) string {
	// 解析Nav接口返回的img_url和sub_url，提取其中的文件名（不带后缀）作为key

	parts := strings.Split(rawUrl, "/")    // 按斜杠分割字符串
	filename := parts[len(parts)-1]        // 取最后一部分，即 "文件名.后缀"
	return strings.Split(filename, ".")[0] // 按点分割，取第一部分即文件名
}
