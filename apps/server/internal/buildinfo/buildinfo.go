package buildinfo

import (
	"runtime"
	"strings"
)

var (
	// Version 由构建系统通过 -ldflags 注入，默认值用于本地开发态。
	Version = "dev"
	// CommitSHA 由构建系统通过 -ldflags 注入，默认值用于本地开发态。
	CommitSHA = "unknown"
	// BuildTimeUTC 由构建系统通过 -ldflags 注入，格式建议为 RFC3339 UTC。
	BuildTimeUTC = ""
	// GoVersion 由构建系统通过 -ldflags 注入；缺失时回退 runtime.Version()。
	GoVersion = ""
)

// Info 统一描述后端构建元信息。
type Info struct {
	Version      string `json:"version"`
	CommitSHA    string `json:"commitSha"`
	BuildTimeUTC string `json:"buildTimeUtc"`
	GoVersion    string `json:"goVersion"`
}

// Current 返回当前进程可用的构建元信息。
func Current() Info {
	version := strings.TrimSpace(Version)
	if version == "" {
		version = "dev"
	}

	commitSHA := strings.TrimSpace(CommitSHA)
	if commitSHA == "" {
		commitSHA = "unknown"
	}

	buildTimeUTC := strings.TrimSpace(BuildTimeUTC)
	if buildTimeUTC == "" {
		buildTimeUTC = "unknown"
	}

	goVersion := strings.TrimSpace(GoVersion)
	if goVersion == "" {
		goVersion = runtime.Version()
	}

	return Info{
		Version:      version,
		CommitSHA:    commitSHA,
		BuildTimeUTC: buildTimeUTC,
		GoVersion:    goVersion,
	}
}
