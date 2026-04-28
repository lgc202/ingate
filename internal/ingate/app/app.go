// Package app 实现 ingate 命令行入口
package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lgc202/ingate-next/internal/core/pipeline"
	"github.com/lgc202/ingate-next/internal/core/target/builtin"
	resource "github.com/lgc202/ingate-next/pkg/apis/gateway/v1"
)

// Run 执行 ingate 命令
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command")
	}

	switch args[0] {
	case "build":
		return runBuild(args[1:], stdin, stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runBuild(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(stderr)

	file := flags.String("file", "", "resource bundle json file, or - for stdin")
	gatewayName := flags.String("gateway", "", "gateway name")
	targetName := flags.String("target", "debug", "target name")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("missing --file")
	}
	if *gatewayName == "" {
		return fmt.Errorf("missing --gateway")
	}

	var input io.Reader
	if *file == "-" {
		input = stdin
	} else {
		f, err := os.Open(*file)
		if err != nil {
			return fmt.Errorf("open file %q: %w", *file, err)
		}
		defer f.Close()
		input = f
	}

	var bundle resource.Bundle
	if err := json.NewDecoder(input).Decode(&bundle); err != nil {
		return fmt.Errorf("decode bundle: %w", err)
	}

	registry, err := builtin.NewRegistry()
	if err != nil {
		return fmt.Errorf("create target registry: %w", err)
	}

	snapshot, err := (pipeline.Pipeline{Registry: registry}).BuildGatewaySnapshotForTarget(bundle, *gatewayName, *targetName)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}

	return nil
}
