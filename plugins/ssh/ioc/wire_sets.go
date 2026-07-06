package ioc

import (
	"github.com/Duke1616/ecmdb-plugins/ioc"
	"github.com/google/wire"
)

var WebSet = wire.NewSet(
	InitConfig,
	InitGinMiddlewares,
	InitPolicySDK,
	InitPermSyncer,
	InitProviders,
	ioc.InitEtcdClient,
	ioc.InitRegistry,
	InitResolverClient,
	InitContextResolver,
	InitSSHHandler,
	InitListener,
	InitWebServer,
)
