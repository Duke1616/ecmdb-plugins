package config

import "github.com/Duke1616/ecmdb/pkg/plugin"

type Config struct {
	Upstream       string
	Resolver       plugin.ContextResolver
	TimeoutSeconds int
}
