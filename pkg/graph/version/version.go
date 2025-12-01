package version

import "runtime/debug"

var (
	info, _ = debug.ReadBuildInfo()
	Name    = info.Main.Path
	Version = "dev"
	Commit  = "dev"
)
