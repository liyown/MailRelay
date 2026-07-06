package version

import "fmt"

var Version = "dev"
var Commit = "unknown"
var BuildTime = "unknown"

func String() string {
	return fmt.Sprintf("mailrelay %s (commit %s, built %s)", Version, Commit, BuildTime)
}
