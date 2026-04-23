package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"google.golang.org/protobuf/encoding/protojson"

	xdsads "github.com/lgc202/ingate/internal/controlplane/xds/ads"
	"github.com/spf13/cobra"
)

type adsOptions struct {
	serverAddress string
	gatewayKey    string
	resourceType  string
	resourceNames []string
	timeout       time.Duration
}

type adsOutput struct {
	ServerAddress string            `json:"serverAddress"`
	GatewayKey    string            `json:"gatewayKey"`
	ResourceType  string            `json:"resourceType"`
	TypeURL       string            `json:"typeUrl"`
	VersionInfo   string            `json:"versionInfo,omitempty"`
	Nonce         string            `json:"nonce,omitempty"`
	ResourceCount int               `json:"resourceCount"`
	ResourceNames []string          `json:"resourceNames"`
	Resources     []json.RawMessage `json:"resources"`
}

func newADSCommand() *cobra.Command {
	opts := &adsOptions{
		serverAddress: "127.0.0.1:19090",
		timeout:       5 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "ads",
		Short: "Fetch standard ADS/xDS resources from xds-server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runADS(cmd.Context(), cmd, *opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.serverAddress, "server", opts.serverAddress, "xds-server gRPC address")
	flags.StringVar(&opts.gatewayKey, "gateway", "", "published gateway key used as ADS node.id")
	flags.StringVar(&opts.resourceType, "type", "", "ADS resource type: lds, rds, cds, or eds")
	flags.StringArrayVar(&opts.resourceNames, "resource", nil, "optional resource name filter; repeat to select multiple resources")
	flags.DurationVar(&opts.timeout, "timeout", opts.timeout, "RPC timeout")
	_ = cmd.MarkFlagRequired("gateway")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func runADS(parent context.Context, cmd *cobra.Command, opts adsOptions) error {
	if opts.serverAddress == "" {
		return fmt.Errorf("server address must not be empty")
	}
	if opts.gatewayKey == "" {
		return fmt.Errorf("gateway key must not be empty")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if parent == nil {
		parent = context.Background()
	}

	typeURL, err := xdsads.TypeURLForAlias(opts.resourceType)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()

	conn, err := dialGRPC(ctx, opts.serverAddress)
	if err != nil {
		return fmt.Errorf("dial ads service: %w", err)
	}
	defer conn.Close()

	client := discoveryv3.NewAggregatedDiscoveryServiceClient(conn)
	stream, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		return fmt.Errorf("open ads stream: %w", err)
	}
	if err := stream.Send(&discoveryv3.DiscoveryRequest{
		Node:          &corev3.Node{Id: opts.gatewayKey},
		TypeUrl:       typeURL,
		ResourceNames: opts.resourceNames,
	}); err != nil {
		return fmt.Errorf("send %s discovery request: %w", opts.resourceType, err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("receive %s discovery response for gateway %q: %w", opts.resourceType, opts.gatewayKey, err)
	}
	_ = stream.CloseSend()

	output := adsOutput{
		ServerAddress: opts.serverAddress,
		GatewayKey:    opts.gatewayKey,
		ResourceType:  opts.resourceType,
		TypeURL:       resp.GetTypeUrl(),
		VersionInfo:   resp.GetVersionInfo(),
		Nonce:         resp.GetNonce(),
		ResourceCount: len(resp.GetResources()),
		ResourceNames: make([]string, 0, len(resp.GetResources())),
		Resources:     make([]json.RawMessage, 0, len(resp.GetResources())),
	}
	marshaler := protojson.MarshalOptions{Multiline: false, UseProtoNames: false}
	for _, resource := range resp.GetResources() {
		name, err := xdsads.ResourceNameFromAny(resource)
		if err != nil {
			return fmt.Errorf("decode %s resource name: %w", opts.resourceType, err)
		}
		raw, err := marshaler.Marshal(resource)
		if err != nil {
			return fmt.Errorf("marshal %s resource %q: %w", opts.resourceType, name, err)
		}
		output.ResourceNames = append(output.ResourceNames, name)
		output.Resources = append(output.Resources, json.RawMessage(raw))
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
