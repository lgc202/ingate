package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	accesslogdata "github.com/envoyproxy/go-control-plane/envoy/data/accesslog/v3"
	accesslogservice "github.com/envoyproxy/go-control-plane/envoy/service/accesslog/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/lgc202/ingate/internal/pkg/requestrecord"
)

const requestTimeout = 15 * time.Second

func send(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("send", flag.ContinueOnError)
	endpoint := flags.String("endpoint", "als:18091", "ALS gRPC endpoint")
	nodeID := flags.String("node", "als-e2e", "Envoy node ID")
	requestID := flags.String("request", "request", "request ID")
	streamID := flags.String("stream", "stream", "stream ID")
	count := flags.Int("count", 1, "records in the batch")
	payloadBytes := flags.Int("payload-bytes", 0, "path payload size")
	invalid := flags.Bool("invalid", false, "send an incomplete record")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if *count <= 0 {
		return errors.New("count must be greater than zero")
	}

	entries := make([]*accesslogdata.HTTPAccessLogEntry, 0, *count)
	startedAt := time.Now().UTC()
	for index := range *count {
		entryRequestID := *requestID
		if *count > 1 {
			entryRequestID = fmt.Sprintf("%s-%d", *requestID, index)
		}
		entry := accessLogEntry(*streamID, entryRequestID, startedAt, *payloadBytes)
		if *invalid {
			entry.Request = nil
		}
		entries = append(entries, entry)
	}

	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	connection, err := grpc.NewClient(*endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("create ALS client: %w", err)
	}
	defer func() { _ = connection.Close() }()

	stream, err := accesslogservice.NewAccessLogServiceClient(connection).StreamAccessLogs(requestCtx)
	if err != nil {
		return fmt.Errorf("open ALS stream: %w", err)
	}
	if err := stream.Send(accessLogMessage(*nodeID, entries...)); err != nil {
		return fmt.Errorf("send ALS record: %w", err)
	}
	if _, err := stream.CloseAndRecv(); err != nil {
		return fmt.Errorf("close ALS stream: %w", err)
	}

	return nil
}

func accessLogMessage(
	nodeID string,
	entries ...*accesslogdata.HTTPAccessLogEntry,
) *accesslogservice.StreamAccessLogsMessage {
	return &accesslogservice.StreamAccessLogsMessage{
		Identifier: &accesslogservice.StreamAccessLogsMessage_Identifier{
			Node:    &corev3.Node{Id: nodeID},
			LogName: requestrecord.StreamName,
		},
		LogEntries: &accesslogservice.StreamAccessLogsMessage_HttpLogs{
			HttpLogs: &accesslogservice.StreamAccessLogsMessage_HTTPAccessLogEntries{LogEntry: entries},
		},
	}
}

func accessLogEntry(
	streamID string,
	requestID string,
	startedAt time.Time,
	payloadBytes int,
) *accesslogdata.HTTPAccessLogEntry {
	path := "/e2e"
	if payloadBytes > 0 {
		path += "/" + strings.Repeat("x", payloadBytes)
	}

	return &accesslogdata.HTTPAccessLogEntry{
		CommonProperties: &accesslogdata.AccessLogCommon{
			StartTime: timestamppb.New(startedAt),
			StreamId:  streamID,
		},
		ProtocolVersion: accesslogdata.HTTPAccessLogEntry_HTTP11,
		Request: &accesslogdata.HTTPRequestProperties{
			RequestMethod: corev3.RequestMethod_GET,
			Authority:     "als-e2e.local",
			Path:          path,
			RequestId:     requestID,
		},
		Response: &accesslogdata.HTTPResponseProperties{ResponseCode: wrapperspb.UInt32(200)},
	}
}
