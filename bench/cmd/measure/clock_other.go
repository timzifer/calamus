//go:build !windows

package main

func nanotime() int64 { return portable() }
