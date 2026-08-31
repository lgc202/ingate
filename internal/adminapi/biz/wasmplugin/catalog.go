package wasmplugin

import "golang.org/x/mod/semver"

// CatalogItem 描述插件市场向用户展示的一项插件。
// 制品地址和校验信息只供安装流程使用，不通过管理 API 暴露。
type CatalogItem struct {
	SourceID    string
	SourceName  string
	Package     string
	Name        string
	Version     string
	Category    string
	Description string
	Provider    string
	License     string
	SourceURL   string
}

// CatalogInfo 保存已安装插件在当前目录中的来源和升级信息。
type CatalogInfo struct {
	SourceName       string
	LatestVersion    string
	UpgradeAvailable bool
}

func newerVersion(current, candidate string) bool {
	current = canonicalVersion(current)
	candidate = canonicalVersion(candidate)
	return semver.IsValid(current) && semver.IsValid(candidate) && semver.Compare(candidate, current) > 0
}

func canonicalVersion(value string) string {
	if value == "" || value[0] == 'v' {
		return value
	}
	return "v" + value
}
