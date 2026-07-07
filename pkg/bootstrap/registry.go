package bootstrap

import "github.com/Duke1616/eiam/pkg/web/capability"

// NewRegistry 统一生成满足 EIAM 鉴权要求的插件权限注册表，固定系统模块前缀为 "cmdb"
func NewRegistry(name, description string) capability.IRegistry {
	return capability.NewRegistry("cmdb", name, description)
}
