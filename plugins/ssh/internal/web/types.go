package web

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
