//go:build !darwin

package main

func suppressStderr() {}
func restoreStderr()  {}
