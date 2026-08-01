// Package configurationstatus 定义配置状态聚合接口的响应模型
package configurationstatus

import (
	consoledto "github.com/lgc202/ingate/internal/console/dto"
	configurationstatusservice "github.com/lgc202/ingate/internal/console/service/configurationstatus"
)

// NewGetConfigurationStatusResp 将配置状态聚合用例结果转换为控制台响应
func NewGetConfigurationStatusResp(report *configurationstatusservice.Report) GetConfigurationStatusResp {
	items := make([]Item, 0, len(report.Items))
	for _, item := range report.Items {
		items = append(items, Item{
			Kind:   item.Kind,
			ID:     item.ID,
			Name:   item.Name,
			Status: consoledto.NewResourceStatus(item.Status),
		})
	}
	return GetConfigurationStatusResp{
		Summary: Summary{
			Total:    report.Summary.Total,
			Ready:    report.Summary.Ready,
			Pending:  report.Summary.Pending,
			Error:    report.Summary.Error,
			Disabled: report.Summary.Disabled,
		},
		Items: items,
	}
}
