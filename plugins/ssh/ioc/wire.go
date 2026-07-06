//go:build wireinject

package ioc

import "github.com/google/wire"

func InitApp() (*App, error) {
	wire.Build(
		WebSet,
		wire.Struct(new(App), "*"),
	)
	return nil, nil
}
