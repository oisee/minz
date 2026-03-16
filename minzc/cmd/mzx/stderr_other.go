//go:build !darwin && !mzx_headless

package main

func suppressStderr() {}
func restoreStderr()  {}
