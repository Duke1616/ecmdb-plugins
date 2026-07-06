package ioc

import (
	"github.com/Duke1616/etask/pkg/grpc/registry"
	"github.com/Duke1616/etask/pkg/grpc/registry/etcd"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// InitRegistry 初始化统一的服务发现注册中心（全插件共享公共 IoC 依赖）
func InitRegistry(etcdClient *clientv3.Client) registry.Registry {
	r, err := etcd.NewRegistry(etcdClient)
	if err != nil {
		panic(err)
	}
	return r
}
