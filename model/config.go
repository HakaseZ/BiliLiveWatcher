package model

type Config struct {
	Credential  Credential    `json:"credential"`
	LiveWatcher []LiveWatcher `json:"watcher"`
	LogLevel    string        `json:"logLevel"`
	Capture     CaptureConfig `json:"capture"`
}

type Credential struct {
	Cookie string `json:"cookie"`
}

// CaptureConfig 原始数据包捕获配置
type CaptureConfig struct {
	Enabled    bool   `json:"enabled"`
	Path       string `json:"path"`
	MaxSizeMB  int    `json:"maxSizeMB"`
	MaxBackups int    `json:"maxBackups"`
}

type LiveWatcher struct {
	Roomid   int      `json:"roomid"`
	Event    Event    `json:"event"`
	Streamer string   `json:"streamer"`
	Notifier Notifier `json:"notifier"`
}

type Event struct {
	LIVE      bool    `json:"LIVE"`
	PREPARING bool    `json:"PREPARING"`
	DANMU_MSG []int64 `json:"DANMU_MSG"`
	INTER     []int64 `json:"INTER"`
}

type Notifier struct {
	Bark []string `json:"bark"`
}
