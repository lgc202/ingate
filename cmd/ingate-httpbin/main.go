package main

import (
	"os"

	"github.com/mccutchen/go-httpbin/v2/httpbin/cmd"
)

func main() {
	os.Exit(cmd.Main(cmd.BuildInfo{
		Version: "ingate-dev",
		Commit:  "unknown",
		Date:    "unknown",
	}))
}
