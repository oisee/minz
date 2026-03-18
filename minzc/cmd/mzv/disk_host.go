package main

import (
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/mir2"
)

// registerDiskHosts opens a FAT disk image and wires @disk_read / @disk_write
// host functions so that Nanz programs can use stdlib/fs/fat12.minz.
func registerDiskHosts(vm *mir2.VM, imgPath string, trace bool) {
	f, err := os.OpenFile(imgPath, os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mzv: --disk: %v\n", err)
		os.Exit(1)
	}
	// Note: file intentionally not closed — lives for the VM's lifetime.

	// @disk_read(pdrv: u8, buf: ^u8, sector: u16, count: u8) -> u8
	vm.Hosts["@disk_read"] = func(args []mir2.Value) ([]mir2.Value, error) {
		sector := args[2].I
		count := int(args[3].I)
		if count == 0 {
			count = 1
		}
		buf := make([]byte, 512*count)
		if _, err := f.ReadAt(buf, sector*512); err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  disk_read(sect=%d, n=%d): %v\n", sector, count, err)
			}
			return []mir2.Value{{I: 1}}, nil
		}
		vm.WriteHeapBytes(args[1].I, buf)
		if trace {
			fmt.Fprintf(os.Stderr, "  disk_read(sect=%d, n=%d) → heap@%d\n", sector, count, args[1].I)
		}
		return []mir2.Value{{I: 0}}, nil
	}

	// @disk_write(pdrv: u8, buf: ^u8, sector: u16, count: u8) -> u8
	vm.Hosts["@disk_write"] = func(args []mir2.Value) ([]mir2.Value, error) {
		sector := args[2].I
		count := int(args[3].I)
		if count == 0 {
			count = 1
		}
		data := vm.ReadHeap(args[1].I, 512*count)
		if _, err := f.WriteAt(data, sector*512); err != nil {
			if trace {
				fmt.Fprintf(os.Stderr, "  disk_write(sect=%d, n=%d): %v\n", sector, count, err)
			}
			return []mir2.Value{{I: 1}}, nil
		}
		if trace {
			fmt.Fprintf(os.Stderr, "  disk_write(sect=%d, n=%d)\n", sector, count)
		}
		return []mir2.Value{{I: 0}}, nil
	}

	fmt.Fprintf(os.Stderr, "mzv: disk image: %s\n", imgPath)
}
