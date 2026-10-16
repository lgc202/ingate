package version

import (
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetWithoutLinkerInjection(t *testing.T) {
	want := Info{
		GitVersion: "devel", GitCommit: "unknown", GitTreeState: "unknown", BuildDate: "unknown",
		GoVersion: runtime.Version(), Compiler: runtime.Compiler, Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	if diff := cmp.Diff(want, Get()); diff != "" {
		t.Errorf("Get() mismatch (-want +got):\n%s", diff)
	}
}

func TestInfoStringAndText(t *testing.T) {
	info := Info{
		GitVersion: "v1.2.3", GitCommit: "abc123", GitTreeState: "dirty", BuildDate: "2026-10-07T00:00:00Z",
		GoVersion: "go1.27.0", Compiler: "gc", Platform: "linux/amd64",
	}
	if got := info.String(); got != "v1.2.3" {
		t.Errorf("Info.String() = %q, want v1.2.3 only", got)
	}

	want := "Version:        v1.2.3\n" +
		"Git commit:     abc123\n" +
		"Git tree state: dirty\n" +
		"Build date:     2026-10-07T00:00:00Z\n" +
		"Go version:     go1.27.0\n" +
		"Compiler:       gc\n" +
		"Platform:       linux/amd64"
	if got := info.Text(); got != want {
		t.Errorf("Info.Text() = %q, want %q without trailing newline", got, want)
	}
}
