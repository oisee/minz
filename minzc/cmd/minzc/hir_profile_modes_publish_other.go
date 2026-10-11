//go:build !linux

package main

import "fmt"

func publishCompilerModeDirectory(stage, destination string) error {
	return fmt.Errorf("compiler modes: atomic no-replace directory publication requires Linux")
}
