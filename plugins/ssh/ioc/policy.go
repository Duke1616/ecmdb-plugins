package ioc

import (
	"github.com/Duke1616/eiam/pkg/web/capability"
	"github.com/Duke1616/eiam/pkg/web/sdk"
)

func InitPolicySDK() *sdk.SDK {
	return sdk.NewSDK()
}

func InitPermSyncer() capability.Syncer {
	return capability.NewSyncer(capability.NewHttpReporter())
}

func InitProviders() []capability.PermissionProvider {
	return nil
}
