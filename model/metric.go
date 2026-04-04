package model

type Metric struct {
	Mtype    string
	M_RoomID int64
	Msg      interface{}
}

type User struct {
	UID      int64
	Username string
	Face     string
}

type DanmuMsg struct {
	User
	Danmu string
}

type EnterRoomMsg struct {
	User
}

type LiveMsg struct {
	RoomID int64
}

type PreparingMsg struct {
	RoomID int64
}
