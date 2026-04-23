package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	configsyncv1 "github.com/lgc202/ingate/pkg/generated/proto/ingate/configsync/v1"
	discoveryv1 "github.com/lgc202/ingate/pkg/generated/proto/ingate/discovery/v1"
)

type resolveOptions struct {
	serverAddress string
	backendName   string
	backendType   string
	address       string
	port          uint32
	output        string
	timeout       time.Duration
}

type configOptions struct {
	serverAddress string
	gatewayKey    string
	output        string
	timeout       time.Duration
}

type listOptions struct {
	serverAddress string
	output        string
	timeout       time.Duration
}

type summaryOptions struct {
	serverAddress string
	gatewayKey    string
	output        string
	timeout       time.Duration
}

type checkOptions struct {
	serverAddress string
	gatewayKey    string
	backendName   string
	backendType   string
	output        string
	timeout       time.Duration
}

type resolveOutput struct {
	ServerAddress string             `json:"serverAddress"`
	BackendName   string             `json:"backendName"`
	BackendType   string             `json:"backendType,omitempty"`
	Endpoints     []resolvedEndpoint `json:"endpoints"`
}

type configOutput struct {
	ServerAddress  string                        `json:"serverAddress"`
	GatewayKey     string                        `json:"gatewayKey"`
	SourceVersion  string                        `json:"sourceVersion,omitempty"`
	PublishVersion string                        `json:"publishVersion,omitempty"`
	UpdatedAt      string                        `json:"updatedAt,omitempty"`
	Config         *configsyncv1.EffectiveConfig `json:"config,omitempty"`
}

type listOutput struct {
	ServerAddress string                  `json:"serverAddress"`
	Items         []publishedConfigOutput `json:"items"`
}

type summaryOutput struct {
	ServerAddress  string   `json:"serverAddress"`
	GatewayKey     string   `json:"gatewayKey"`
	SourceVersion  string   `json:"sourceVersion,omitempty"`
	PublishVersion string   `json:"publishVersion,omitempty"`
	UpdatedAt      string   `json:"updatedAt,omitempty"`
	ListenerCount  int      `json:"listenerCount"`
	RouteCount     int      `json:"routeCount"`
	BackendCount   int      `json:"backendCount"`
	EndpointCount  int      `json:"endpointCount"`
	ListenerNames  []string `json:"listenerNames"`
	RouteNames     []string `json:"routeNames"`
	BackendNames   []string `json:"backendNames"`
	ListenerHosts  []string `json:"listenerHosts"`
	RouteHostnames []string `json:"routeHostnames"`
	RoutePrefixes  []string `json:"routePrefixes"`
}

type checkOutput struct {
	ServerAddress        string `json:"serverAddress"`
	GatewayKey           string `json:"gatewayKey"`
	BackendName          string `json:"backendName"`
	BackendType          string `json:"backendType,omitempty"`
	GatewayPublished     bool   `json:"gatewayPublished"`
	ConfigReadable       bool   `json:"configReadable"`
	SummaryReady         bool   `json:"summaryReady"`
	BackendResolved      bool   `json:"backendResolved"`
	PublishedGatewaySeen bool   `json:"publishedGatewaySeen"`
	ListenerCount        int    `json:"listenerCount"`
	RouteCount           int    `json:"routeCount"`
	BackendCount         int    `json:"backendCount"`
	EndpointCount        int    `json:"endpointCount"`
	PublishedCount       int    `json:"publishedCount"`
	PublishVersion       string `json:"publishVersion,omitempty"`
	UpdatedAt            string `json:"updatedAt,omitempty"`
}

type publishedConfigOutput struct {
	GatewayKey     string `json:"gatewayKey"`
	SourceVersion  string `json:"sourceVersion,omitempty"`
	PublishVersion string `json:"publishVersion,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

type resolvedEndpoint struct {
	Address string `json:"address"`
	Port    uint32 `json:"port"`
	Weight  uint32 `json:"weight"`
	Healthy bool   `json:"healthy"`
}

func newXDSCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "xds",
		Short: "Interact with xds-server",
	}

	cmd.AddCommand(newResolveCommand())
	cmd.AddCommand(newConfigCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newSummaryCommand())
	cmd.AddCommand(newCheckCommand())
	cmd.AddCommand(newADSCommand())
	return cmd
}

func newResolveCommand() *cobra.Command {
	opts := &resolveOptions{
		serverAddress: "127.0.0.1:19090",
		timeout:       5 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve backend endpoints from xds-server discovery",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runResolve(cmd.Context(), cmd, *opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.serverAddress, "server", opts.serverAddress, "xds-server gRPC address")
	flags.StringVar(&opts.backendName, "backend", "", "backend name to resolve")
	flags.StringVar(&opts.backendType, "backend-type", "", "optional backend type filter")
	flags.StringVar(&opts.address, "address", "", "optional client address for discovery context")
	flags.Uint32Var(&opts.port, "port", 0, "optional client port for discovery context")
	flags.StringVar(&opts.output, "output", "json", "resolve output format: json or text")
	flags.DurationVar(&opts.timeout, "timeout", opts.timeout, "RPC timeout")
	_ = cmd.MarkFlagRequired("backend")

	return cmd
}

func newConfigCommand() *cobra.Command {
	opts := &configOptions{
		serverAddress: "127.0.0.1:19090",
		timeout:       5 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read the current published effective config from xds-server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfig(cmd.Context(), cmd, *opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.serverAddress, "server", opts.serverAddress, "xds-server gRPC address")
	flags.StringVar(&opts.gatewayKey, "gateway", "", "resolvedgateway object key (<namespace>/<name> or <name>)")
	flags.StringVar(&opts.output, "output", "json", "config output format: json or text")
	flags.DurationVar(&opts.timeout, "timeout", opts.timeout, "RPC timeout")
	_ = cmd.MarkFlagRequired("gateway")

	return cmd
}

func newListCommand() *cobra.Command {
	opts := &listOptions{
		serverAddress: "127.0.0.1:19090",
		timeout:       5 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List currently published gateway configs from xds-server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd.Context(), cmd, *opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.serverAddress, "server", opts.serverAddress, "xds-server gRPC address")
	flags.StringVar(&opts.output, "output", "json", "list output format: json or text")
	flags.DurationVar(&opts.timeout, "timeout", opts.timeout, "RPC timeout")

	return cmd
}

func newSummaryCommand() *cobra.Command {
	opts := &summaryOptions{
		serverAddress: "127.0.0.1:19090",
		timeout:       5 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Show an operator-friendly summary of a published gateway config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSummary(cmd.Context(), cmd, *opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.serverAddress, "server", opts.serverAddress, "xds-server gRPC address")
	flags.StringVar(&opts.gatewayKey, "gateway", "", "resolvedgateway object key (<namespace>/<name> or <name>)")
	flags.StringVar(&opts.output, "output", "json", "summary output format: json or text")
	flags.DurationVar(&opts.timeout, "timeout", opts.timeout, "RPC timeout")
	_ = cmd.MarkFlagRequired("gateway")

	return cmd
}

func newCheckCommand() *cobra.Command {
	opts := &checkOptions{
		serverAddress: "127.0.0.1:19090",
		timeout:       5 * time.Second,
	}

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run an operator-focused readiness check against xds-server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCheck(cmd.Context(), cmd, *opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.serverAddress, "server", opts.serverAddress, "xds-server gRPC address")
	flags.StringVar(&opts.gatewayKey, "gateway", "", "resolvedgateway object key (<namespace>/<name> or <name>)")
	flags.StringVar(&opts.backendName, "backend", "", "backend name to resolve")
	flags.StringVar(&opts.backendType, "backend-type", "", "optional backend type filter")
	flags.StringVar(&opts.output, "output", "json", "check output format: json or text")
	flags.DurationVar(&opts.timeout, "timeout", opts.timeout, "RPC timeout")
	_ = cmd.MarkFlagRequired("gateway")
	_ = cmd.MarkFlagRequired("backend")

	return cmd
}

func runResolve(parent context.Context, cmd *cobra.Command, opts resolveOptions) error {
	if opts.serverAddress == "" {
		return fmt.Errorf("server address must not be empty")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if opts.output != "json" && opts.output != "text" {
		return fmt.Errorf("unsupported output format %q", opts.output)
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		opts.serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial discovery service: %w", err)
	}
	defer conn.Close()

	client := discoveryv1.NewDiscoveryServiceClient(conn)
	resp, err := client.Resolve(ctx, &discoveryv1.ResolveRequest{
		BackendName: opts.backendName,
		BackendType: opts.backendType,
		Address:     opts.address,
		Port:        opts.port,
	})
	if err != nil {
		return fmt.Errorf("resolve backend %q: %w", opts.backendName, err)
	}

	output := resolveOutput{
		ServerAddress: opts.serverAddress,
		BackendName:   opts.backendName,
		BackendType:   opts.backendType,
		Endpoints:     make([]resolvedEndpoint, 0, len(resp.GetEndpoints())),
	}
	for _, endpoint := range resp.GetEndpoints() {
		output.Endpoints = append(output.Endpoints, resolvedEndpoint{
			Address: endpoint.GetAddress(),
			Port:    endpoint.GetPort(),
			Weight:  endpoint.GetWeight(),
			Healthy: endpoint.GetHealthy(),
		})
	}

	if opts.output == "text" {
		return writeResolveText(cmd.OutOrStdout(), output)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func runConfig(parent context.Context, cmd *cobra.Command, opts configOptions) error {
	if opts.serverAddress == "" {
		return fmt.Errorf("server address must not be empty")
	}
	if opts.gatewayKey == "" {
		return fmt.Errorf("gateway key must not be empty")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if opts.output != "json" && opts.output != "text" {
		return fmt.Errorf("unsupported output format %q", opts.output)
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()

	conn, err := dialGRPC(ctx, opts.serverAddress)
	if err != nil {
		return fmt.Errorf("dial configsync service: %w", err)
	}
	defer conn.Close()

	client := configsyncv1.NewConfigSyncServiceClient(conn)
	resp, err := client.GetConfig(ctx, &configsyncv1.GetConfigRequest{
		GatewayKey: opts.gatewayKey,
	})
	if err != nil {
		return fmt.Errorf("get config for gateway %q: %w", opts.gatewayKey, err)
	}

	output := configOutput{
		ServerAddress:  opts.serverAddress,
		GatewayKey:     resp.GetGatewayKey(),
		SourceVersion:  resp.GetSourceVersion(),
		PublishVersion: resp.GetPublishVersion(),
		UpdatedAt:      resp.GetUpdatedAt(),
		Config:         resp.GetConfig(),
	}

	if opts.output == "text" {
		return writeConfigText(cmd.OutOrStdout(), output)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func runList(parent context.Context, cmd *cobra.Command, opts listOptions) error {
	if opts.serverAddress == "" {
		return fmt.Errorf("server address must not be empty")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if opts.output != "json" && opts.output != "text" {
		return fmt.Errorf("unsupported output format %q", opts.output)
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()

	conn, err := dialGRPC(ctx, opts.serverAddress)
	if err != nil {
		return fmt.Errorf("dial configsync service: %w", err)
	}
	defer conn.Close()

	client := configsyncv1.NewConfigSyncServiceClient(conn)
	resp, err := client.ListConfigs(ctx, &configsyncv1.ListConfigsRequest{})
	if err != nil {
		return fmt.Errorf("list published configs: %w", err)
	}

	output := listOutput{
		ServerAddress: opts.serverAddress,
		Items:         make([]publishedConfigOutput, 0, len(resp.GetItems())),
	}
	for _, item := range resp.GetItems() {
		output.Items = append(output.Items, publishedConfigOutput{
			GatewayKey:     item.GetGatewayKey(),
			SourceVersion:  item.GetSourceVersion(),
			PublishVersion: item.GetPublishVersion(),
			UpdatedAt:      item.GetUpdatedAt(),
		})
	}

	if opts.output == "text" {
		return writeListText(cmd.OutOrStdout(), output)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func runSummary(parent context.Context, cmd *cobra.Command, opts summaryOptions) error {
	if opts.serverAddress == "" {
		return fmt.Errorf("server address must not be empty")
	}
	if opts.gatewayKey == "" {
		return fmt.Errorf("gateway key must not be empty")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if opts.output != "json" && opts.output != "text" {
		return fmt.Errorf("unsupported output format %q", opts.output)
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()

	conn, err := dialGRPC(ctx, opts.serverAddress)
	if err != nil {
		return fmt.Errorf("dial configsync service: %w", err)
	}
	defer conn.Close()

	client := configsyncv1.NewConfigSyncServiceClient(conn)
	resp, err := client.GetConfig(ctx, &configsyncv1.GetConfigRequest{
		GatewayKey: opts.gatewayKey,
	})
	if err != nil {
		return fmt.Errorf("get config for gateway %q: %w", opts.gatewayKey, err)
	}

	output := summaryOutput{
		ServerAddress:  opts.serverAddress,
		GatewayKey:     resp.GetGatewayKey(),
		SourceVersion:  resp.GetSourceVersion(),
		PublishVersion: resp.GetPublishVersion(),
		UpdatedAt:      resp.GetUpdatedAt(),
	}
	if cfg := resp.GetConfig(); cfg != nil {
		output.ListenerCount = len(cfg.GetListeners())
		output.RouteCount = len(cfg.GetRoutes())
		output.BackendCount = len(cfg.GetBackends())
		output.ListenerNames, output.ListenerHosts = summarizeListeners(cfg.GetListeners())
		output.RouteNames, output.RouteHostnames, output.RoutePrefixes = summarizeRoutes(cfg.GetRoutes())
		output.BackendNames, output.EndpointCount = summarizeBackends(cfg.GetBackends())
	}

	if opts.output == "text" {
		return writeSummaryText(cmd.OutOrStdout(), output)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func runCheck(parent context.Context, cmd *cobra.Command, opts checkOptions) error {
	if opts.serverAddress == "" {
		return fmt.Errorf("server address must not be empty")
	}
	if opts.gatewayKey == "" {
		return fmt.Errorf("gateway key must not be empty")
	}
	if opts.backendName == "" {
		return fmt.Errorf("backend name must not be empty")
	}
	if opts.timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	if opts.output != "json" && opts.output != "text" {
		return fmt.Errorf("unsupported output format %q", opts.output)
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithTimeout(parent, opts.timeout)
	defer cancel()

	conn, err := dialGRPC(ctx, opts.serverAddress)
	if err != nil {
		return fmt.Errorf("dial services: %w", err)
	}
	defer conn.Close()

	configClient := configsyncv1.NewConfigSyncServiceClient(conn)
	discoveryClient := discoveryv1.NewDiscoveryServiceClient(conn)

	configResp, err := configClient.GetConfig(ctx, &configsyncv1.GetConfigRequest{GatewayKey: opts.gatewayKey})
	if err != nil {
		return fmt.Errorf("get config for gateway %q: %w", opts.gatewayKey, err)
	}
	listResp, err := configClient.ListConfigs(ctx, &configsyncv1.ListConfigsRequest{})
	if err != nil {
		return fmt.Errorf("list published configs: %w", err)
	}
	resolveResp, err := discoveryClient.Resolve(ctx, &discoveryv1.ResolveRequest{
		BackendName: opts.backendName,
		BackendType: opts.backendType,
	})
	if err != nil {
		return fmt.Errorf("resolve backend %q: %w", opts.backendName, err)
	}

	output := checkOutput{
		ServerAddress:    opts.serverAddress,
		GatewayKey:       configResp.GetGatewayKey(),
		BackendName:      opts.backendName,
		BackendType:      opts.backendType,
		GatewayPublished: configResp.GetGatewayKey() != "",
		ConfigReadable:   configResp.GetConfig() != nil,
		BackendResolved:  len(resolveResp.GetEndpoints()) > 0,
		PublishedCount:   len(listResp.GetItems()),
		PublishVersion:   configResp.GetPublishVersion(),
		UpdatedAt:        configResp.GetUpdatedAt(),
	}
	if cfg := configResp.GetConfig(); cfg != nil {
		output.ListenerCount = len(cfg.GetListeners())
		output.RouteCount = len(cfg.GetRoutes())
		output.BackendCount = len(cfg.GetBackends())
		output.SummaryReady = output.ListenerCount > 0 || output.RouteCount > 0 || output.BackendCount > 0
	}
	if len(resolveResp.GetEndpoints()) > 0 {
		output.EndpointCount = len(resolveResp.GetEndpoints())
	}
	for _, item := range listResp.GetItems() {
		if item.GetGatewayKey() == opts.gatewayKey {
			output.PublishedGatewaySeen = true
			break
		}
	}

	if opts.output == "text" {
		return writeCheckText(cmd.OutOrStdout(), output)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func dialGRPC(ctx context.Context, serverAddress string) (*grpc.ClientConn, error) {
	return grpc.DialContext(
		ctx,
		serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}

func summarizeListeners(listeners []*configsyncv1.Listener) ([]string, []string) {
	names := make([]string, 0, len(listeners))
	hosts := make([]string, 0)
	seenHosts := map[string]struct{}{}
	for _, listener := range listeners {
		names = append(names, listener.GetName())
		for _, host := range listener.GetHostnames() {
			if _, ok := seenHosts[host]; ok {
				continue
			}
			seenHosts[host] = struct{}{}
			hosts = append(hosts, host)
		}
	}
	return names, hosts
}

func summarizeRoutes(routes []*configsyncv1.Route) ([]string, []string, []string) {
	names := make([]string, 0, len(routes))
	hosts := make([]string, 0)
	prefixes := make([]string, 0)
	seenHosts := map[string]struct{}{}
	seenPrefixes := map[string]struct{}{}
	for _, route := range routes {
		names = append(names, route.GetName())
		for _, host := range route.GetHostnames() {
			if _, ok := seenHosts[host]; ok {
				continue
			}
			seenHosts[host] = struct{}{}
			hosts = append(hosts, host)
		}
		for _, prefix := range route.GetPathPrefixes() {
			if _, ok := seenPrefixes[prefix]; ok {
				continue
			}
			seenPrefixes[prefix] = struct{}{}
			prefixes = append(prefixes, prefix)
		}
	}
	return names, hosts, prefixes
}

func summarizeBackends(backends []*configsyncv1.Backend) ([]string, int) {
	names := make([]string, 0, len(backends))
	endpointCount := 0
	for _, backend := range backends {
		names = append(names, backend.GetName())
		endpointCount += len(backend.GetEndpoints())
	}
	return names, endpointCount
}

func writeSummaryText(out io.Writer, summary summaryOutput) error {
	if out == nil {
		return nil
	}

	lines := []string{
		fmt.Sprintf("gateway: %s", summary.GatewayKey),
		fmt.Sprintf("server: %s", summary.ServerAddress),
		fmt.Sprintf("sourceVersion: %s", summary.SourceVersion),
		fmt.Sprintf("publishVersion: %s", summary.PublishVersion),
		fmt.Sprintf("updatedAt: %s", summary.UpdatedAt),
		fmt.Sprintf("listeners: %d", summary.ListenerCount),
		fmt.Sprintf("routes: %d", summary.RouteCount),
		fmt.Sprintf("backends: %d", summary.BackendCount),
		fmt.Sprintf("endpoints: %d", summary.EndpointCount),
		fmt.Sprintf("listenerNames: %s", joinOrDash(summary.ListenerNames)),
		fmt.Sprintf("routeNames: %s", joinOrDash(summary.RouteNames)),
		fmt.Sprintf("backendNames: %s", joinOrDash(summary.BackendNames)),
		fmt.Sprintf("listenerHosts: %s", joinOrDash(summary.ListenerHosts)),
		fmt.Sprintf("routeHostnames: %s", joinOrDash(summary.RouteHostnames)),
		fmt.Sprintf("routePrefixes: %s", joinOrDash(summary.RoutePrefixes)),
	}
	_, err := io.WriteString(out, strings.Join(lines, "\n")+"\n")
	return err
}

func writeListText(out io.Writer, list listOutput) error {
	if out == nil {
		return nil
	}

	lines := []string{
		fmt.Sprintf("server: %s", list.ServerAddress),
		"gatewayKey | sourceVersion | publishVersion | updatedAt",
	}
	if len(list.Items) == 0 {
		lines = append(lines, "-")
	} else {
		for _, item := range list.Items {
			lines = append(lines, fmt.Sprintf(
				"%s | %s | %s | %s",
				emptyOrDash(item.GatewayKey),
				emptyOrDash(item.SourceVersion),
				emptyOrDash(item.PublishVersion),
				emptyOrDash(item.UpdatedAt),
			))
		}
	}

	_, err := io.WriteString(out, strings.Join(lines, "\n")+"\n")
	return err
}

func writeResolveText(out io.Writer, resolved resolveOutput) error {
	if out == nil {
		return nil
	}

	lines := []string{
		fmt.Sprintf("server: %s", resolved.ServerAddress),
		fmt.Sprintf("backend: %s", resolved.BackendName),
		fmt.Sprintf("backendType: %s", emptyOrDash(resolved.BackendType)),
		fmt.Sprintf("endpoints: %d", len(resolved.Endpoints)),
	}
	if len(resolved.Endpoints) == 0 {
		lines = append(lines, "-")
	} else {
		for _, endpoint := range resolved.Endpoints {
			lines = append(lines, fmt.Sprintf(
				"%s:%d weight=%d healthy=%t",
				endpoint.Address,
				endpoint.Port,
				endpoint.Weight,
				endpoint.Healthy,
			))
		}
	}

	_, err := io.WriteString(out, strings.Join(lines, "\n")+"\n")
	return err
}

func writeConfigText(out io.Writer, config configOutput) error {
	if out == nil {
		return nil
	}

	lines := []string{
		fmt.Sprintf("server: %s", config.ServerAddress),
		fmt.Sprintf("gateway: %s", config.GatewayKey),
		fmt.Sprintf("sourceVersion: %s", emptyOrDash(config.SourceVersion)),
		fmt.Sprintf("publishVersion: %s", emptyOrDash(config.PublishVersion)),
		fmt.Sprintf("updatedAt: %s", emptyOrDash(config.UpdatedAt)),
	}

	if config.Config == nil {
		lines = append(lines, "config: -")
	} else {
		lines = append(lines,
			fmt.Sprintf("configVersion: %s", emptyOrDash(config.Config.GetVersion())),
			fmt.Sprintf("listeners: %d", len(config.Config.GetListeners())),
			fmt.Sprintf("routes: %d", len(config.Config.GetRoutes())),
			fmt.Sprintf("backends: %d", len(config.Config.GetBackends())),
		)
	}

	_, err := io.WriteString(out, strings.Join(lines, "\n")+"\n")
	return err
}

func writeCheckText(out io.Writer, check checkOutput) error {
	if out == nil {
		return nil
	}

	lines := []string{
		fmt.Sprintf("server: %s", check.ServerAddress),
		fmt.Sprintf("gateway: %s", check.GatewayKey),
		fmt.Sprintf("backend: %s", check.BackendName),
		fmt.Sprintf("backendType: %s", emptyOrDash(check.BackendType)),
		fmt.Sprintf("gatewayPublished: %t", check.GatewayPublished),
		fmt.Sprintf("configReadable: %t", check.ConfigReadable),
		fmt.Sprintf("summaryReady: %t", check.SummaryReady),
		fmt.Sprintf("backendResolved: %t", check.BackendResolved),
		fmt.Sprintf("publishedGatewaySeen: %t", check.PublishedGatewaySeen),
		fmt.Sprintf("listeners: %d", check.ListenerCount),
		fmt.Sprintf("routes: %d", check.RouteCount),
		fmt.Sprintf("backends: %d", check.BackendCount),
		fmt.Sprintf("endpoints: %d", check.EndpointCount),
		fmt.Sprintf("publishedCount: %d", check.PublishedCount),
		fmt.Sprintf("publishVersion: %s", emptyOrDash(check.PublishVersion)),
		fmt.Sprintf("updatedAt: %s", emptyOrDash(check.UpdatedAt)),
	}

	_, err := io.WriteString(out, strings.Join(lines, "\n")+"\n")
	return err
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func emptyOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
