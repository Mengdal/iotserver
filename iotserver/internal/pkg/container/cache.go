package container

import (
	"iotserver/internal/pkg/cache"
	"iotserver/internal/pkg/di"
)

var CacheFuncName = di.TypeInstanceToName((*cache.Cache)(nil))

func CacheFuncFrom(get di.Get) cache.Cache {
	client, ok := get(CacheFuncName).(cache.Cache)
	if !ok {
		return nil
	}

	return client
}
