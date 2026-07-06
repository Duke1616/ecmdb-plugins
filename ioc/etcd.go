package ioc

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// InitEtcdClient 初始化 etcd 客户端连接（全插件共享公共 IoC 依赖）
func InitEtcdClient() *clientv3.Client {
	var cfg clientv3.Config

	if err := viper.UnmarshalKey("etcd", &cfg); err != nil {
		panic(fmt.Errorf("unable to decode into structure: %v", err))
	}

	cfg.DialTimeout = 5 * time.Second

	client, err := clientv3.New(cfg)
	if err != nil {
		panic(fmt.Errorf("failed to connect to etcd: %v", err))
	}

	// 连接性 Ping 测试
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Status(ctx, client.Endpoints()[0])
	if err != nil {
		panic(fmt.Errorf("failed to ping etcd: %v", err))
	}

	return client
}
