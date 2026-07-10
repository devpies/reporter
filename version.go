package main

// Version is the reporter build version. It defaults to v0.0.1 for local
// builds (go build, go run) and is overridden at release time via
// -ldflags "-X main.Version=vX.Y.Z".
var Version = "v0.0.1"
