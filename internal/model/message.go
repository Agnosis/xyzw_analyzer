package model

// XYMsg 定义消息结构
type XYMsg struct {
	Ack  int    `json:"ack"`
	Body []byte `json:"body"`
	Cmd  string `json:"cmd"`
	Seq  int32  `json:"seq"`
	Time int64  `json:"time"`
}
