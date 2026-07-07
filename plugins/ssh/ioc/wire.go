//go:build wireinject

package ioc

import (
	"github.com/Duke1616/ecmdb-plugins/pkg/bootstrap"
	"github.com/google/wire"
)

func InitApp() (*bootstrap.PluginApp, error) {
	wire.Build(
		WebSet,
	)
	return nil, nil
}
