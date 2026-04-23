package version

import (
	"fmt"
	"runtime"
)

var (
	GitVersion = "dev"
	GitCommit  = "unknown"
	BuildDate  = "unknown"
)

func Get() Info {
	return Info{
		GitVersion: GitVersion,
		GitCommit:  GitCommit,
		BuildDate:  BuildDate,
		GoVersion:  runtime.Version(),
	}
}

type Info struct {
	GitVersion string
	GitCommit  string
	BuildDate  string
	GoVersion  string
}

func (i Info) String() string {
	return fmt.Sprintf("version=%s commit=%s buildDate=%s go=%s", i.GitVersion, i.GitCommit, i.BuildDate, i.GoVersion)
}
