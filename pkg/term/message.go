package term

import "encoding/json"

// Message 定义所有 Web 交互式终端的标准消息协议（包含 SSH、K8s Exec、Docker 等通用协议）
type Message struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// ParseMessage 从原始 JSON 字节流中反序列化终端消息
func ParseMessage(value []byte) (Message, error) {
	m := Message{}
	err := json.Unmarshal(value, &m)
	return NewMessage(m.Operation, m.Data, m.Cols, m.Rows), err
}

// NewMessage 构造标准终端消息实例
func NewMessage(operation string, data string, cols, rows int) Message {
	return Message{Operation: operation, Data: data, Cols: cols, Rows: rows}
}
