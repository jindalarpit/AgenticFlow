package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/agenticflow/agenticflow/daemon/internal/release"
	"github.com/spf13/cobra"
)

// VersionInfo holds all version-related metadata for the CLI binary.
// Fields are populated at build time via ldflags for version, commit, and date.
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// GetVersionInfo returns the current version information.
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: date,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

func init() {
	versionCmd.Flags().String("output", "text", "Output format: text or json")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE:  runVersion,
}

func runVersion(cmd *cobra.Command, _ []string) error {
	output, _ := cmd.Flags().GetString("output")
	info := GetVersionInfo()

	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	fmt.Println(release.FormatVersion(info.Version, info.Commit, info.BuildDate))
	fmt.Printf("go: %s, os/arch: %s/%s\n", info.GoVersion, info.OS, info.Arch)
	return nil
}
