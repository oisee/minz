// Copyright 2022 The CC Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build cc.dmesg
// +build cc.dmesg

package cparse

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Dmesgs = true

var (
	pid  = fmt.Sprintf("[%v %v] ", os.Getpid(), filepath.Base(os.Args[0]))
	logf *os.File
)

func init() {
	t := time.Now()
	// 01/02 03:04:05PM '06 -0700
	DmesgsFile = "ccv4-dmesg-" + t.Format("2006-01-02-03-150405")
	DmesgsFile = filepath.Join(os.TempDir(), fmt.Sprintf("%s.%d", DmesgsFile, os.Getpid()))
	if err := os.Mkdir(DmesgsFile, 0770); err != nil {
		panic(err.Error())
	}

	fn := filepath.Join(DmesgsFile, "dmesg.log")
	var err error
	if logf, err = os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0644); err != nil {
		panic(err.Error())
	}

	Dmesg("%v", time.Now())
	fmt.Fprintf(os.Stderr, "debug messages in %s\n", fn)
}

// Dmesg synchronously appends a debug message to DmesgsFile. To disable do not
// use -tags=cc.dmesg. Cannot be enabled on Windows.
func Dmesg(s string, args ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(args))
	}
	s = fmt.Sprintf(pid+s, args...)
	s = fmt.Sprintf("%s %v", s, []string{origin(2), origin(3), origin(4)})
	switch {
	case len(s) != 0 && s[len(s)-1] == '\n':
		fmt.Fprint(logf, s)
	default:
		fmt.Fprintln(logf, s)
	}
}
