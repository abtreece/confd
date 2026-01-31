package main

// Version and GitSHA are set at build time via ldflags.
// Example: -ldflags "-X main.Version=1.0.0 -X main.GitSHA=abc123"
// Goreleaser automatically injects these from the git tag.
var (
	Version = "dev"
	GitSHA  = ""
)
