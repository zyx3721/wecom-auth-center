// Package buildinfo 承载构建信息，由 CI 通过 -ldflags -X 注入，源码直接编译时使用默认值。
package buildinfo

// Version、Commit、BuildDate 为构建信息变量，CI 发布时注入
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
