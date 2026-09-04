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

type ConnectResp struct {
	SessionID string `json:"session_id"`
}

type connectSpec struct {
	action     string
	successMsg string
}

// 采用注册表模式集中维护连接协议规范，扩展新协议时无需改动业务判断分支
var connectSpecs = map[ConnectType]connectSpec{
	ConnectTypeSSH:     {action: define.ActionTerminal, successMsg: "SSH 连接成功"},
	ConnectTypeWebSftp: {action: define.ActionSFTP, successMsg: "SFTP 连接成功"},
}

var unsupportedProtocols = map[ConnectType]string{
	ConnectTypeRDP: "暂不支持 RDP 协议",
	ConnectTypeVNC: "暂不支持 VNC 协议",
}

func (c ConnectType) spec() (connectSpec, error) {
	if spec, ok := connectSpecs[c]; ok {
		return spec, nil
	}
	if reason, ok := unsupportedProtocols[c]; ok {
		return connectSpec{}, fmt.Errorf("%s", reason)
	}
	return connectSpec{}, fmt.Errorf("不支持的连接类型: %s", c)
}
