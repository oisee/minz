package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/minz/minzc/pkg/mir2"
)

// registerFileHosts adds host filesystem access for self-hosting toolchain.
// Programs can read source files directly from the host filesystem via MZV.
//
// Host functions:
//   @file_size(path: ^u8) -> u16          — returns file size (0 if not found)
//   @file_read(path: ^u8, buf: ^u8) -> u16 — reads entire file into buf, returns bytes read
//   @file_exists(path: ^u8) -> u8         — returns 1 if file exists, 0 otherwise
func registerFileHosts(vm *mir2.VM, baseDir string, trace bool) {
	// Helper: read null-terminated string from VM heap.
	readStr := func(ptr int64) string {
		var buf []byte
		for i := int64(0); i < 4096; i++ {
			b := vm.ReadHeap(ptr+i, 1)
			if b == nil || len(b) == 0 || b[0] == 0 {
				break
			}
			buf = append(buf, b[0])
		}
		return string(buf)
	}

	// Resolve path relative to baseDir.
	resolve := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(baseDir, path)
	}

	// @file_size(path: ^u8) -> u16
	vm.Hosts["@file_size"] = func(args []mir2.Value) ([]mir2.Value, error) {
		path := resolve(readStr(args[0].I))
		info, err := os.Stat(path)
		if err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  file_size(%q): %v\n", path, err)
			}
			return []mir2.Value{{I: 0}}, nil
		}
		size := info.Size()
		if size > 65535 {
			size = 65535
		}
		if trace {
			fmt.Fprintf(os.Stderr, "  file_size(%q) → %d\n", path, size)
		}
		return []mir2.Value{{I: size}}, nil
	}

	// @file_read(path: ^u8, buf: ^u8) -> u16
	vm.Hosts["@file_read"] = func(args []mir2.Value) ([]mir2.Value, error) {
		path := resolve(readStr(args[0].I))
		data, err := os.ReadFile(path)
		if err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  file_read(%q): %v\n", path, err)
			}
			return []mir2.Value{{I: 0}}, nil
		}
		if len(data) > 65535 {
			data = data[:65535]
		}
		vm.WriteHeapBytes(args[1].I, data)
		// Null-terminate
		vm.WriteHeapBytes(args[1].I+int64(len(data)), []byte{0})
		if trace {
			fmt.Fprintf(os.Stderr, "  file_read(%q) → %d bytes at heap@%d\n", path, len(data), args[1].I)
		}
		return []mir2.Value{{I: int64(len(data))}}, nil
	}

	// @file_exists(path: ^u8) -> u8
	vm.Hosts["@file_exists"] = func(args []mir2.Value) ([]mir2.Value, error) {
		path := resolve(readStr(args[0].I))
		_, err := os.Stat(path)
		if err != nil {
			return []mir2.Value{{I: 0}}, nil
		}
		return []mir2.Value{{I: 1}}, nil
	}
}
