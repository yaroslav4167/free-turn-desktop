//go:build !darwin && !windows

package integration

func nativeSystemLanguage() string { return "en" }
