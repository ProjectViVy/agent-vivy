//go:build !diva_c06_overlay

package app

func memoryLoopC06OverlayAvailable() bool { return false }

func configureMemoryLoopC06Hook(string) {}
