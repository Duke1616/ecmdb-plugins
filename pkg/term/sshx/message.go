package sshx

import "github.com/Duke1616/ecmdb-plugins/pkg/term"

// TerminalMessage 兼容别名，底层统一引用 term.Message
type TerminalMessage = term.Message

// ParseTerminalMessage 解析终端消息
func ParseTerminalMessage(value []byte) (TerminalMessage, error) {
	return term.ParseMessage(value)
}

// NewMessage 构造终端消息
func NewMessage(operation string, data string, cols, rows int) TerminalMessage {
	return term.NewMessage(operation, data, cols, rows)
}
