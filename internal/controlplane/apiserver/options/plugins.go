package options

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	genericoptions "k8s.io/apiserver/pkg/server/options"

	reservedmetadata "github.com/lgc202/ingate/internal/controlplane/apiserver/admission/plugin/reservedmetadata"
)

var AllOrderedPlugins = []string{
	reservedmetadata.PluginName,
}

func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	reservedmetadata.Register(plugins)
}

func DefaultOffAdmissionPlugins() sets.Set[string] {
	return sets.New[string]()
}

func EnabledAdmissionPluginNames(opts *genericoptions.AdmissionOptions) []string {
	allOffPlugins := append(sets.List[string](opts.DefaultOffPlugins), opts.DisablePlugins...)
	disabledPlugins := sets.NewString(allOffPlugins...)
	enabledPlugins := sets.NewString(opts.EnablePlugins...)
	disabledPlugins = disabledPlugins.Difference(enabledPlugins)

	orderedPlugins := make([]string, 0, len(opts.RecommendedPluginOrder))
	for _, pluginName := range opts.RecommendedPluginOrder {
		if !disabledPlugins.Has(pluginName) {
			orderedPlugins = append(orderedPlugins, pluginName)
		}
	}

	return orderedPlugins
}
