//go:build !windows

package main

import "syscall"

// daemonSysProcAttr returns the SysProcAttr used when spawning the background
// daemon process. On Unix, Setpgid detaches the child from the parent's
// process group so that signals sent to the parent's group (e.g. when the
// terminal closes) do not propagate to the daemon.
//
// The withBreakaway parameter exists only to share a signature with the
// Windows implementation (where it controls CREATE_BREAKAWAY_FROM_JOB);
// on Unix it is ignored.
func daemonSysProcAttr(_ bool) *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// isAccessDeniedSpawnErr is always false on Unix. The Windows version
// looks for ERROR_ACCESS_DENIED to detect "parent Job Object disallowed
// breakaway" and trigger the breakaway-disabled retry; that retry is a
// no-op on Unix.
func isAccessDeniedSpawnErr(_ error) bool { return false }
