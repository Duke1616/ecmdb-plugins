package web

import (
	"fmt"

	"github.com/Duke1616/ecmdb-plugins/plugins/ssh/internal/define"
)

type ConnectType string

const (
	ConnectTypeSSH     ConnectType = "Web Shell"
	ConnectTypeWebSftp ConnectType = "Web Sftp"
	ConnectTypeRDP     ConnectType = "RDP"
	ConnectTypeVNC     ConnectType = "VNC"
)

type ConnectReq struct {
	Type       ConnectType `json:"type"`
	ResourceId int64       `json:"resource_id"`
}

type connectSpec struct {
	action     string
	successMsg string
}

func (c ConnectType) spec() (connectSpec, error) {
	switch c {
	case ConnectTypeSSH:
		return connectSpec{action: define.ActionTerminal, successMsg: "SSH 连接成功"}, nil
	case ConnectTypeWebSftp:
		return connectSpec{action: define.ActionSFTP, successMsg: "SFTP 连接成功"}, nil
	case ConnectTypeRDP:
		return connectSpec{}, fmt.Errorf("暂不支持 RDP 协议")
	case ConnectTypeVNC:
		return connectSpec{}, fmt.Errorf("暂不支持 VNC 协议")
	default:
		return connectSpec{}, fmt.Errorf("不支持的连接类型: %s", c)
	}
}
