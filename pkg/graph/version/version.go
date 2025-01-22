package version

import "runtime/debug"

var info, _ = debug.ReadBuildInfo()
var Name = info.Main.Path
var Version = "dev"
var Commit = "dev"
