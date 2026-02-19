package network

type NavResponse struct {
	Data struct {
		Wbi struct {
			ImgUrl string `json:"img_url"`
			SubURL string `json:"sub_url"`
		} `json:"wbi_img"`
	} `json:"data"`
}
