package common

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

type TableRowFn func(obj runtime.Object) ([]interface{}, error)

type TableConvertor struct {
	resource schema.GroupResource
	columns  []metav1.TableColumnDefinition
	rowFn    TableRowFn
}

func NewTableConvertor(resource schema.GroupResource, columns []metav1.TableColumnDefinition, rowFn TableRowFn) rest.TableConvertor {
	return TableConvertor{resource: resource, columns: columns, rowFn: rowFn}
}

func (c TableConvertor) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	noHeaders, err := noHeadersFrom(tableOptions)
	if err != nil {
		return nil, err
	}

	table := &metav1.Table{}
	appendRow := func(obj runtime.Object) error {
		cells, err := c.rowFn(obj)
		if err != nil {
			resource := c.resource
			if info, ok := genericapirequest.RequestInfoFrom(ctx); ok {
				resource = schema.GroupResource{Group: info.APIGroup, Resource: info.Resource}
			}
			return errNotAcceptable{resource: resource, err: err}
		}
		table.Rows = append(table.Rows, metav1.TableRow{Cells: cells, Object: runtime.RawExtension{Object: obj}})
		return nil
	}

	if meta.IsListType(object) {
		if err := meta.EachListItem(object, appendRow); err != nil {
			return nil, err
		}
	} else {
		if err := appendRow(object); err != nil {
			return nil, err
		}
	}

	if listAccessor, err := meta.ListAccessor(object); err == nil {
		table.ResourceVersion = listAccessor.GetResourceVersion()
		table.Continue = listAccessor.GetContinue()
		table.RemainingItemCount = listAccessor.GetRemainingItemCount()
	} else if commonAccessor, err := meta.CommonAccessor(object); err == nil {
		table.ResourceVersion = commonAccessor.GetResourceVersion()
	}

	if !noHeaders {
		table.ColumnDefinitions = c.columns
	}

	return table, nil
}

func noHeadersFrom(tableOptions runtime.Object) (bool, error) {
	if tableOptions == nil {
		return false, nil
	}
	options, ok := tableOptions.(*metav1.TableOptions)
	if !ok {
		return false, fmt.Errorf("unrecognized table options type %T", tableOptions)
	}
	if options == nil {
		return false, nil
	}
	return options.NoHeaders, nil
}

type errNotAcceptable struct {
	resource schema.GroupResource
	err      error
}

func (e errNotAcceptable) Error() string {
	if e.err != nil {
		return fmt.Sprintf("the resource %s cannot be converted to a Table: %v", e.resource, e.err)
	}
	return fmt.Sprintf("the resource %s cannot be converted to a Table", e.resource)
}

func (e errNotAcceptable) Status() metav1.Status {
	return metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    406,
		Reason:  metav1.StatusReason("NotAcceptable"),
		Message: e.Error(),
	}
}

func FormatTimestampAge(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}
	return formatDuration(time.Since(timestamp.Time))
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	seconds := int(duration.Seconds())
	const (
		minute = 60
		hour   = 60 * minute
		day    = 24 * hour
	)
	switch {
	case seconds < minute:
		return fmt.Sprintf("%ds", seconds)
	case seconds < hour:
		return fmt.Sprintf("%dm", seconds/minute)
	case seconds < day:
		return fmt.Sprintf("%dh", seconds/hour)
	default:
		return fmt.Sprintf("%dd", seconds/day)
	}
}
