package buildinfo

// Version, Commit, and Date are set at build time via ldflags:
//
//	go build -ldflags "-X forgectl/buildinfo.Version=v1.0.0"
var (
	Version = "v0.0.1"
	Commit  = "unknown"
	Date    = "unknown"
)
