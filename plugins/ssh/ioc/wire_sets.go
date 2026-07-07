package ioc

import (
	"github.com/Duke1616/ecmdb-plugins/ioc"
	"github.com/google/wire"
)

var WebSet = wire.NewSet(
	InitConfig,
	ioc.InitEtcdClient,
	ioc.InitRegistry,
	InitResolverClient,
	InitContextResolver,
	InitSSHHandler,
	InitListener,
	InitWebServer,
)
