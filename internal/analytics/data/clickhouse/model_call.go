package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/lgc202/ingate/internal/analytics/biz/request"
	aiprotocol "github.com/lgc202/ingate/internal/pkg/aiextproc"
	"github.com/lgc202/ingate/internal/pkg/requestrecord"
	"github.com/lgc202/ingate/internal/pkg/routeconfig"
)

const modelCallColumns = `
    request_record_id,
    started_at,
    gateway_id,
    route_id,
    upstream_id,
    caller_id,
    access_key_id,
    status_class,
    client_model,
    upstream_model,
    upstream_protocol,
    response_model,
    finish_reason,
    input_tokens,
    output_tokens,
    total_tokens`

const modelCallSelectColumns = `
    request_record_id,
    client_model,
    upstream_model,
    upstream_protocol,
    response_model,
    finish_reason,
    input_tokens,
    output_tokens,
    total_tokens`

type modelCallRow struct {
	requestRecordID string
	call            request.ModelCall
}

// saveModelCall 只保存已经选中并尝试模型 Service 的调用。
//
// AI Route 在选路前也可能写入客户端模型元数据，但这种本地拒绝没有产生模型调用，
// 因此不能进入用量事实表。
func (s *Store) saveModelCall(ctx context.Context, record request.Record) error {
	call := record.ModelCall
	if call == nil || record.UpstreamID == "" {
		return nil
	}

	statement := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.modelCallTable,
		modelCallColumns,
	)
	if err := s.connection.Exec(
		asyncInsertContext(ctx, record.ID),
		statement,
		record.ID,
		record.StartedAt,
		record.GatewayID,
		record.RouteID,
		record.UpstreamID,
		record.CallerID,
		record.AccessKeyID,
		uint8(record.StatusClass),
		call.ClientModel,
		call.UpstreamModel,
		call.UpstreamProtocol,
		call.ResponseModel,
		call.FinishReason,
		call.InputTokens,
		call.OutputTokens,
		call.TotalTokens,
	); err != nil {
		return fmt.Errorf("insert model call for request record %q: %w", record.ID, err)
	}
	return nil
}

// listModelCalls 批量补充当前请求页的模型调用，避免请求列表执行大表 JOIN。
func (s *Store) listModelCalls(
	ctx context.Context,
	records []request.Summary,
) (calls map[string]*request.ModelCall, err error) {
	calls = make(map[string]*request.ModelCall, len(records))
	if len(records) == 0 {
		return calls, nil
	}

	ids := make([]string, 0, len(records))
	requested := make(map[string]bool, len(records))
	minStartedAt := records[0].StartedAt
	maxStartedAt := records[0].StartedAt
	for i := range records {
		recordID := records[i].ID
		ids = append(ids, recordID)
		requested[recordID] = true
		if records[i].StartedAt.Before(minStartedAt) {
			minStartedAt = records[i].StartedAt
		}
		if records[i].StartedAt.After(maxStartedAt) {
			maxStartedAt = records[i].StartedAt
		}
	}

	statement := fmt.Sprintf(`
SELECT %s
FROM %s FINAL
WHERE started_at >= fromUnixTimestamp64Nano(?)
  AND started_at <= fromUnixTimestamp64Nano(?)
  AND request_record_id IN (?)
ORDER BY request_record_id`, modelCallSelectColumns, s.modelCallTable)
	rows, err := s.connection.Query(
		ctx,
		statement,
		minStartedAt.UnixNano(),
		maxStartedAt.UnixNano(),
		ids,
	)
	if err != nil {
		return nil, fmt.Errorf("query model calls for request page: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close model call rows: %w", closeErr))
		}
	}()

	for rows.Next() {
		row, scanErr := scanModelCallRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if !requested[row.requestRecordID] {
			return nil, fmt.Errorf(
				"model call references request record %q outside the requested page",
				row.requestRecordID,
			)
		}
		if calls[row.requestRecordID] != nil {
			return nil, fmt.Errorf("duplicate model call for request record %q", row.requestRecordID)
		}
		call := row.call
		calls[row.requestRecordID] = &call
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read model call rows: %w", err)
	}
	return calls, nil
}

// getModelCall 返回请求详情对应的模型调用。
func (s *Store) getModelCall(
	ctx context.Context,
	requestRecordID string,
	startedAt time.Time,
) (call *request.ModelCall, err error) {
	statement := fmt.Sprintf(`
SELECT %s
FROM %s FINAL
WHERE started_at = @started_at AND request_record_id = @request_record_id
LIMIT 1`, modelCallSelectColumns, s.modelCallTable)
	rows, err := s.connection.Query(
		ctx,
		statement,
		clickhousego.DateNamed("started_at", startedAt, clickhousego.NanoSeconds),
		clickhousego.Named("request_record_id", requestRecordID),
	)
	if err != nil {
		return nil, fmt.Errorf("query model call for request record %q: %w", requestRecordID, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close model call rows: %w", closeErr))
		}
	}()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("read model call for request record %q: %w", requestRecordID, err)
		}
		return nil, nil
	}
	row, err := scanModelCallRow(rows)
	if err != nil {
		return nil, err
	}
	if row.requestRecordID != requestRecordID {
		return nil, fmt.Errorf(
			"model call references request record %q instead of %q",
			row.requestRecordID,
			requestRecordID,
		)
	}
	return &row.call, nil
}

func scanModelCallRow(rows driver.Rows) (modelCallRow, error) {
	var row modelCallRow
	if err := rows.Scan(
		&row.requestRecordID,
		&row.call.ClientModel,
		&row.call.UpstreamModel,
		&row.call.UpstreamProtocol,
		&row.call.ResponseModel,
		&row.call.FinishReason,
		&row.call.InputTokens,
		&row.call.OutputTokens,
		&row.call.TotalTokens,
	); err != nil {
		return modelCallRow{}, fmt.Errorf("scan model call: %w", err)
	}
	if !requestrecord.IsValidID(row.requestRecordID) ||
		!routeconfig.IsValidModelName(row.call.ClientModel) ||
		!routeconfig.IsValidModelName(row.call.UpstreamModel) ||
		row.call.ResponseModel != "" && !routeconfig.IsValidModelName(row.call.ResponseModel) {
		return modelCallRow{}, errors.New("stored model call has an invalid identity or model mapping")
	}
	switch row.call.UpstreamProtocol {
	case string(aiprotocol.UpstreamProtocolOpenAI), string(aiprotocol.UpstreamProtocolAnthropic):
	default:
		return modelCallRow{}, fmt.Errorf(
			"stored model call for request record %q has an invalid protocol",
			row.requestRecordID,
		)
	}
	if row.call.InputTokens != nil && row.call.OutputTokens != nil &&
		*row.call.InputTokens > math.MaxUint64-*row.call.OutputTokens {
		return modelCallRow{}, fmt.Errorf(
			"stored model call for request record %q has token counts outside the supported range",
			row.requestRecordID,
		)
	}
	var minimumTotal uint64
	if row.call.InputTokens != nil {
		minimumTotal = *row.call.InputTokens
	}
	if row.call.OutputTokens != nil && *row.call.OutputTokens > minimumTotal {
		minimumTotal = *row.call.OutputTokens
	}
	if row.call.InputTokens != nil && row.call.OutputTokens != nil {
		minimumTotal = *row.call.InputTokens + *row.call.OutputTokens
	}
	if row.call.TotalTokens != nil && *row.call.TotalTokens < minimumTotal {
		return modelCallRow{}, fmt.Errorf(
			"stored model call for request record %q has inconsistent token counts",
			row.requestRecordID,
		)
	}
	return row, nil
}
