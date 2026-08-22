package wasmplugin

import kratoserrors "github.com/go-kratos/kratos/v3/errors"

const reasonCatalogUnavailable = "PLUGIN_CATALOG_UNAVAILABLE"

var errCatalogUnavailable = kratoserrors.ServiceUnavailable(
	reasonCatalogUnavailable,
	"plugin catalog unavailable",
).WithMetadata(map[string]string{"user_message": "插件目录暂时无法更新，请稍后重试"})

func catalogUnavailable(cause error) error {
	return errCatalogUnavailable.WithCause(cause)
}
