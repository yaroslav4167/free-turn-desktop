//go:build !windows

package integration

// Other platforms retain their existing installer/helper permission checks.
func CheckUpdatePermissions() error { return nil }
