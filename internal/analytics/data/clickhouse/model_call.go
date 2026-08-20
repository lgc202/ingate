package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"time"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	alsv1 "github.com/lgc202/ingate/api/als/v1"
	"github.com/lgc202/ingate/internal/analytics/biz/request"
)

const modelCallColumns = `
    request_record_id,
    started_at,
    gateway_id,
    route_id,
    upstream_id,
    caller_id,
    access_key_id,
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

// saveModelCalls 只保存确实经过模型 Service 的请求，不为普通 HTTP 请求写空行
func (s *Store) saveModelCalls(ctx context.Context, facts []request.Fact) (err error) {
	hasModelCall := false
	for i := range facts {
		if facts[i].Record.GetAiModelCall() != nil {
			hasModelCall = true
			break
		}
	}
	if !hasModelCall {
		return nil
	}

	statement := fmt.Sprintf("INSERT INTO %s (%s)", s.modelCallTable, modelCallColumns)
	batch, err := s.connection.PrepareBatch(ctx, statement)
	if err != nil {
		return fmt.Errorf("prepare model call batch: %w", err)
	}
	defer func() {
		if batch.IsSent() {
			return
		}
		if abortErr := batch.Abort(); abortErr != nil {
			err = errors.Join(err, fmt.Errorf("abort model call batch: %w", abortErr))
		}
	}()

	for _, fact := range facts {
		record := fact.Record
		call := record.GetAiModelCall()
		if call == nil {
			continue
		}
		if err := batch.Append(
			record.GetId(),
			record.GetStartedAt().AsTime(),
			record.GetGatewayId(),
			record.GetRouteId(),
			record.GetUpstreamId(),
			record.GetCallerId(),
			record.GetAccessKeyId(),
			call.GetClientModel(),
			call.GetUpstreamModel(),
			call.GetUpstreamProtocol(),
			call.GetResponseModel(),
			call.GetFinishReason(),
			call.InputTokens,
			call.OutputTokens,
			call.TotalTokens,
		); err != nil {
			return fmt.Errorf("append model call for request record %q: %w", record.GetId(), err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send model call batch: %w", err)
	}
	return nil
}

// listModelCalls 批量补充当前请求页的模型调用，避免请求列表执行大表 JOIN
func (s *Store) listModelCalls(
	ctx context.Context,
	records []request.Summary,
) (calls map[string]*request.ModelCall, err error) {
	calls = make(map[string]*request.ModelCall)
	if len(records) == 0 {
		return calls, nil
	}

	ids := make([]string, 0, len(records))
	minStartedAt := records[0].StartedAt
	maxStartedAt := records[0].StartedAt
	for i := range records {
		ids = append(ids, records[i].ID)
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
		call := row.call
		calls[row.requestRecordID] = &call
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read model call rows: %w", err)
	}
	return calls, nil
}

// getModelCall 返回请求详情对应的模型调用
func (s *Store) getModelCall(
	ctx context.Context,
	requestRecordID string,
	startedAt time.Time,
) (call *alsv1.AIModelCall, err error) {
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
	return modelCallProto(&row.call), nil
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
	return row, nil
}

func modelCallProto(call *request.ModelCall) *alsv1.AIModelCall {
	return &alsv1.AIModelCall{
		ClientModel:      call.ClientModel,
		UpstreamModel:    call.UpstreamModel,
		UpstreamProtocol: call.UpstreamProtocol,
		ResponseModel:    call.ResponseModel,
		FinishReason:     call.FinishReason,
		InputTokens:      call.InputTokens,
		OutputTokens:     call.OutputTokens,
		TotalTokens:      call.TotalTokens,
	}
}
