package main

// Port-level I/O handler for MZV — maps VM OpIn8/OpOut8 to console + network.
//
// Same port numbers as MZE/MZX:
//   $23 — console (stdout write, stdin read)
//   $25 — stderr (write only)
//   $30 — network data (read/write)
//   $31 — network control (connect/disconnect/status)
//
// Also registers @extern host functions (net_read, net_write, ctl_read,
// ctl_write) that delegate to the same port handlers, so programs using
// @extern declarations work identically to programs using OpIn8/OpOut8.
//
// Implements mir2.PortIO interface.

import (
	"fmt"
	"os"
	"runtime"

	"github.com/minz/minzc/pkg/mir2"
)

// VMPorts bridges MIR2 VM port I/O to console and network.
type VMPorts struct {
	net     *netHost       // network handler (port $30/$31)
	stdinCh <-chan byte     // keyboard input channel
	verbose bool
}

func newVMPorts(nh *netHost, stdinCh <-chan byte, verbose bool) *VMPorts {
	return &VMPorts{net: nh, stdinCh: stdinCh, verbose: verbose}
}

func (p *VMPorts) ReadPort(address uint16) byte {
	port := byte(address & 0xFF)

	switch port {
	case 0x23: // console stdin
		if p.stdinCh != nil {
			select {
			case b := <-p.stdinCh:
				return 0x80 | b
			default:
				return 0x00
			}
		}
		return 0x00

	case 0x30: // network data
		if p.net != nil {
			return p.net.DataRead()
		}
		return 0x00

	case 0x31: // network control status
		if p.net != nil {
			return p.net.CtlRead()
		}
		return 0xFF // idle
	}

	if p.verbose {
		fmt.Fprintf(os.Stderr, "[port] IN 0x%04X → 0xFF (unmapped)\n", address)
	}
	return 0xFF
}

func (p *VMPorts) WritePort(address uint16, b byte) {
	port := byte(address & 0xFF)

	switch port {
	case 0x23: // console stdout
		os.Stdout.Write([]byte{b})
		return

	case 0x25: // console stderr
		os.Stderr.Write([]byte{b})
		return

	case 0x30: // network data
		if p.net != nil {
			p.net.DataWrite(b)
		}
		return

	case 0x31: // network control
		if p.net != nil {
			p.net.CtlWrite(b)
		}
		return
	}

	if p.verbose {
		fmt.Fprintf(os.Stderr, "[port] OUT 0x%04X, 0x%02X (unmapped)\n", address, b)
	}
}

// registerPortHosts registers @extern host functions that delegate to port I/O.
// This bridges the gap: Nanz @extern declarations → VM host table → port handlers.
func registerPortHosts(vm *mir2.VM, ports *VMPorts) {
	// net_read() -> u8 : IN A, ($30)
	vm.Hosts["net_read"] = func(args []mir2.Value) ([]mir2.Value, error) {
		runtime.Gosched() // yield to stdin reader goroutine
		b := ports.ReadPort(0x30)
		if b != 0 && ports.verbose {
			ch := b & 0x7F
			fmt.Fprintf(os.Stderr, "[net<] %c (0x%02X)\n", ch, ch)
		}
		return []mir2.Value{{I: int64(b)}}, nil
	}
	// net_write(b: u8) : OUT ($30), A
	vm.Hosts["net_write"] = func(args []mir2.Value) ([]mir2.Value, error) {
		b := byte(args[0].I & 0xFF)
		if ports.verbose {
			fmt.Fprintf(os.Stderr, "[net>] %c (0x%02X)\n", b, b)
		}
		ports.WritePort(0x30, b)
		return nil, nil
	}
	// ctl_read() -> u8 : IN A, ($31)
	vm.Hosts["ctl_read"] = func(args []mir2.Value) ([]mir2.Value, error) {
		return []mir2.Value{{I: int64(ports.ReadPort(0x31))}}, nil
	}
	// ctl_write(b: u8) : OUT ($31), A
	vm.Hosts["ctl_write"] = func(args []mir2.Value) ([]mir2.Value, error) {
		ports.WritePort(0x31, byte(args[0].I&0xFF))
		return nil, nil
	}
}

// DataRead/DataWrite/CtlRead/CtlWrite on netHost — convenience wrappers
// that match the host function signatures but operate at port level.

func (n *netHost) DataRead() byte {
	if n.conn == nil {
		return 0x00
	}
	select {
	case b := <-n.rxBuf:
		return 0x80 | b
	default:
		return 0x00
	}
}

func (n *netHost) DataWrite(b byte) {
	if n.conn == nil {
		return
	}
	n.conn.Write([]byte{b})
}

func (n *netHost) CtlRead() byte {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.status
}

func (n *netHost) CtlWrite(b byte) {
	if b != 0 {
		n.ctlBuf = append(n.ctlBuf, b)
		return
	}
	if len(n.ctlBuf) == 0 {
		return
	}
	cmd := n.ctlBuf[0]
	arg := string(n.ctlBuf[1:])
	n.ctlBuf = n.ctlBuf[:0]

	switch cmd {
	case 'C':
		go n.doConnect(arg, false, false)
	case 'T':
		go n.doConnect(arg, true, false)
	case 'D':
		n.doDisconnect()
	}
}
