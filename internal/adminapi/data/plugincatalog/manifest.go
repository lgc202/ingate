package plugincatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/mod/semver"

	"github.com/lgc202/ingate/internal/adminapi/biz/wasmplugin"
	resource "github.com/lgc202/ingate/internal/pkg/apis/gateway/v1"
	"github.com/lgc202/ingate/internal/pkg/httpurl"
	"github.com/lgc202/ingate/internal/pkg/resourceconfig"
	"github.com/lgc202/ingate/internal/pkg/wasmconfig"
)

const (
	maxCatalogPlugins          = 512
	maxPluginReleases          = 128
	maxCatalogCategoryBytes    = 128
	maxCatalogDescriptionBytes = 4 << 10
	maxCatalogProviderBytes    = 256
	maxCatalogLicenseBytes     = 128
)

type manifest struct {
	Plugins []manifestPlugin `json:"plugins"`
}

type manifestPlugin struct {
	Package     string            `json:"package"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Description string            `json:"description"`
	Provider    string            `json:"provider"`
	License     string            `json:"license"`
	SourceURL   string            `json:"sourceUrl"`
	Releases    []manifestRelease `json:"releases"`
}

type manifestRelease struct {
	Version          string           `json:"version"`
	MinIngateVersion string           `json:"minIngateVersion"`
	Artifact         manifestArtifact `json:"artifact"`
}

type manifestArtifact struct {
	Repository string `json:"repository"`
	SHA256     string `json:"sha256"`
}

func parseManifest(
	data []byte,
	definition sourceDefinition,
	ingateVersion string,
) (sourceState, error) {
	var document manifest
	if err := json.Unmarshal(data, &document); err != nil {
		return sourceState{}, err
	}
	if len(document.Plugins) > maxCatalogPlugins {
		return sourceState{}, fmt.Errorf(
			"plugin catalog contains more than %d plugins",
			maxCatalogPlugins,
		)
	}
	state := sourceState{
		definition: definition,
		items:      make([]wasmplugin.CatalogItem, 0, len(document.Plugins)),
		specs:      make(map[string]resource.WasmPluginSpec, len(document.Plugins)),
	}
	for _, plugin := range document.Plugins {
		if err := addManifestPlugin(&state, plugin, ingateVersion); err != nil {
			return sourceState{}, err
		}
	}
	return state, nil
}

func addManifestPlugin(
	state *sourceState,
	plugin manifestPlugin,
	ingateVersion string,
) error {
	packageName := strings.TrimSpace(plugin.Package)
	if packageName == "" {
		return errors.New("plugin catalog entry requires package")
	}
	if !resource.IsSupportedWasmPluginPackage(packageName) {
		// 目录可以先于当前 Ingate 版本发布新插件；旧版本不解释未知插件的元数据和版本结构。
		return nil
	}
	displayName := strings.TrimSpace(plugin.Name)
	if displayName == "" || len(plugin.Releases) == 0 {
		return fmt.Errorf("plugin %q requires name and releases", packageName)
	}
	if len(plugin.Releases) > maxPluginReleases {
		return fmt.Errorf(
			"plugin %q contains more than %d releases",
			packageName,
			maxPluginReleases,
		)
	}
	if !resourceconfig.IsValidDisplayName(displayName) {
		return fmt.Errorf("plugin %q has invalid name", packageName)
	}
	if _, exists := state.specs[packageName]; exists {
		return fmt.Errorf("plugin catalog contains duplicate package %q", packageName)
	}
	category, valid := catalogText(plugin.Category, maxCatalogCategoryBytes)
	if !valid {
		return fmt.Errorf("plugin %q has invalid category", packageName)
	}
	description, valid := catalogText(plugin.Description, maxCatalogDescriptionBytes)
	if !valid {
		return fmt.Errorf("plugin %q has invalid description", packageName)
	}
	provider, valid := catalogText(plugin.Provider, maxCatalogProviderBytes)
	if !valid {
		return fmt.Errorf("plugin %q has invalid provider", packageName)
	}
	license, valid := catalogText(plugin.License, maxCatalogLicenseBytes)
	if !valid {
		return fmt.Errorf("plugin %q has invalid license", packageName)
	}
	sourceURL := strings.TrimSpace(plugin.SourceURL)
	if sourceURL != "" && !httpurl.IsValid(sourceURL) {
		return fmt.Errorf("plugin %q has invalid source URL", packageName)
	}
	release, exists, err := latestCompatibleRelease(plugin.Releases, ingateVersion)
	if err != nil {
		return fmt.Errorf("validate plugin %q releases: %w", packageName, err)
	}
	if !exists {
		return nil
	}
	state.items = append(state.items, wasmplugin.CatalogItem{
		SourceID:    state.definition.id,
		SourceName:  state.definition.displayName,
		Package:     packageName,
		Name:        displayName,
		Version:     release.Version,
		Category:    category,
		Description: description,
		Provider:    provider,
		License:     license,
		SourceURL:   sourceURL,
	})
	state.specs[packageName] = resource.WasmPluginSpec{
		SourceID:    state.definition.id,
		DisplayName: displayName,
		Package:     packageName,
		Version:     release.Version,
		URL:         releaseArtifactURL(release),
		SHA256: strings.TrimPrefix(
			strings.TrimSpace(release.Artifact.SHA256),
			"sha256:",
		),
		PullPolicy: resource.WasmPluginPullIfNotPresent,
	}
	return nil
}

func latestCompatibleRelease(
	releases []manifestRelease,
	ingateVersion string,
) (manifestRelease, bool, error) {
	var selected manifestRelease
	seenVersions := make(map[string]bool, len(releases))
	for _, release := range releases {
		release.Version = strings.TrimPrefix(strings.TrimSpace(release.Version), "v")
		release.MinIngateVersion = canonicalVersion(strings.TrimSpace(release.MinIngateVersion))
		release.Artifact.Repository = strings.TrimSpace(release.Artifact.Repository)
		release.Artifact.SHA256 = strings.TrimPrefix(
			strings.TrimSpace(release.Artifact.SHA256),
			"sha256:",
		)
		releaseVersion := canonicalVersion(release.Version)
		if !semver.IsValid(releaseVersion) || release.Artifact.Repository == "" {
			return manifestRelease{}, false, errors.New(
				"release requires a semantic version and artifact repository",
			)
		}
		if seenVersions[releaseVersion] {
			return manifestRelease{}, false, fmt.Errorf(
				"release version %q is duplicated",
				release.Version,
			)
		}
		seenVersions[releaseVersion] = true
		if !wasmconfig.IsValidArtifactURL(releaseArtifactURL(release)) {
			return manifestRelease{}, false, fmt.Errorf(
				"release %q has invalid artifact repository",
				release.Version,
			)
		}
		if !wasmconfig.IsValidSHA256Digest(release.Artifact.SHA256) {
			return manifestRelease{}, false, fmt.Errorf(
				"release %q has invalid SHA-256 digest",
				release.Version,
			)
		}
		if release.MinIngateVersion != "" && !semver.IsValid(release.MinIngateVersion) {
			return manifestRelease{}, false, fmt.Errorf(
				"release %q has invalid minimum Ingate version",
				release.Version,
			)
		}
		if !compatibleWithIngate(ingateVersion, release.MinIngateVersion) {
			continue
		}
		if selected.Version == "" ||
			semver.Compare(releaseVersion, canonicalVersion(selected.Version)) > 0 {
			selected = release
		}
	}
	return selected, selected.Version != "", nil
}

func compatibleWithIngate(current, minimum string) bool {
	if minimum == "" || strings.Contains(current, "unknown") {
		return true
	}
	current = canonicalVersion(current)
	return semver.IsValid(current) && semver.Compare(current, minimum) >= 0
}

func canonicalVersion(value string) string {
	if value == "" || value[0] == 'v' {
		return value
	}
	return "v" + value
}

func releaseArtifactURL(release manifestRelease) string {
	return fmt.Sprintf(
		"oci://%s:v%s",
		release.Artifact.Repository,
		release.Version,
	)
}

func catalogText(value string, maxBytes int) (string, bool) {
	value = strings.TrimSpace(value)
	return value, len(value) <= maxBytes &&
		utf8.ValidString(value) &&
		strings.IndexFunc(value, unicode.IsControl) == -1
}
