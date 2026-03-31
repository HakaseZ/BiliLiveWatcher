package model

type Config struct {
	Credential  Credential    `json:"credential"`
	LiveWatcher []LiveWatcher `json:"watcher"`
}

type Credential struct {
	Cookie string `json:"cookie"`
}

type LiveWatcher struct {
	Roomid   int      `json:"roomid"`
	Event    Event    `json:"event"`
	Notifier Notifier `json:"notifier"`
}

type Event struct {
	LIVE      bool  `json:"LIVE"`
	PREPARING bool  `json:"PREPARING"`
	DANMU_MSG []int `json:"DANMU_MSG"`
	INTER     []int `json:"INTER"`
}

type Notifier struct {
	Bark []string `json:"bark"`
}
