package model

// BaseResource 基础资产模型定义，所有 ECMDB 插件业务资产均应匿名组合嵌入此结构体
// 继承 CMDB 天然内置的资产名称 (name) 基础属性
type BaseResource struct {
	Name string `plugin:"name,label=资产名称"` // 资产名称 (CMDB 内置基础属性)
}
