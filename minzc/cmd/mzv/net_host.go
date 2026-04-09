package main

// Net host functions for MZV — TCP/TLS network I/O.
//
// Implements the net_* and ctl_* @extern functions used by the IRC client.
// Bridges Z80 @extern calls to real TCP sockets via Go net package.
//
// Functions:
//   net_read()     → read byte from server ($00 = no data, $80|byte = ready)
//   net_write(b)   → write byte to server
//   ctl_read()     → read connection status
//   ctl_write(b)   → write control command byte (connect/disconnect)
//
// Control protocol:
//   OUT: 'C' + host:port + \0 → TCP connect
//   OUT: 'T' + host:port + \0 → TLS connect
//   OUT: 'D' + \0             → disconnect
//   IN:  $00=busy, $01=connected, $02=dns error, $03=refused, $04=timeout, $FF=idle

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/minz/minzc/pkg/mir2"
)

type netHost struct {
	conn   net.Conn
	rxBuf  chan byte
	ctlBuf []byte
	status byte // $00=busy, $01=connected, $02-$04=error, $FF=idle
	mu     sync.Mutex
}

func newNetHost() *netHost {
	return &netHost{status: 0xFF}
}

// preConnect establishes a connection before the VM starts (--net flag).
func (n *netHost) preConnect(addr string, useTLS bool) error {
	var conn net.Conn
	var err error

	if useTLS {
		host, _, _ := net.SplitHostPort(addr)
		conn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp", addr,
			&tls.Config{ServerName: host},
		)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
	}
	if err != nil {
		return fmt.Errorf("net connect %s: %w", addr, err)
	}

	n.conn = conn
	n.rxBuf = make(chan byte, 8192)
	n.status = 0x01 // connected
	go n.readPump()
	return nil
}

func (n *netHost) readPump() {
	b := make([]byte, 1)
	for {
		_, err := n.conn.Read(b)
		if err != nil {
			return
		}
		select {
		case n.rxBuf <- b[0]:
		default:
			// buffer full — drop byte (shouldn't happen with 8K buffer)
		}
	}
}

// registerNetHosts installs net_* and ctl_* host functions on the VM.
func registerNetHosts(vm *mir2.VM, nh *netHost, verbose bool) {
	// net_read() -> u8
	vm.Hosts["net_read"] = func(args []mir2.Value) ([]mir2.Value, error) {
		if nh.conn == nil {
			return []mir2.Value{{I: 0}}, nil
		}
		select {
		case b := <-nh.rxBuf:
			return []mir2.Value{{I: int64(0x80 | b)}}, nil
		default:
			return []mir2.Value{{I: 0}}, nil
		}
	}

	// net_write(b: u8)
	vm.Hosts["net_write"] = func(args []mir2.Value) ([]mir2.Value, error) {
		if nh.conn == nil {
			return nil, nil
		}
		b := byte(args[0].I & 0xFF)
		nh.conn.Write([]byte{b})
		return nil, nil
	}

	// ctl_read() -> u8
	vm.Hosts["ctl_read"] = func(args []mir2.Value) ([]mir2.Value, error) {
		nh.mu.Lock()
		s := nh.status
		nh.mu.Unlock()
		return []mir2.Value{{I: int64(s)}}, nil
	}

	// ctl_write(b: u8)
	vm.Hosts["ctl_write"] = func(args []mir2.Value) ([]mir2.Value, error) {
		b := byte(args[0].I & 0xFF)
		if b != 0 {
			nh.ctlBuf = append(nh.ctlBuf, b)
			return nil, nil
		}
		// \0 = execute command
		if len(nh.ctlBuf) == 0 {
			return nil, nil
		}
		cmd := nh.ctlBuf[0]
		arg := string(nh.ctlBuf[1:])
		nh.ctlBuf = nh.ctlBuf[:0]

		switch cmd {
		case 'C': // TCP connect
			go nh.doConnect(arg, false, verbose)
		case 'T': // TLS connect
			go nh.doConnect(arg, true, verbose)
		case 'D': // Disconnect
			nh.doDisconnect()
		}
		return nil, nil
	}

	if verbose && nh.conn != nil {
		fmt.Fprintf(os.Stderr, "[mzv] net: pre-connected to %s\n", nh.conn.RemoteAddr())
	}
}

func (n *netHost) doConnect(addr string, useTLS bool, verbose bool) {
	n.mu.Lock()
	if n.conn != nil {
		n.mu.Unlock()
		n.mu.Lock()
		n.status = 0x05 // already connected
		n.mu.Unlock()
		return
	}
	n.status = 0x00 // busy
	n.mu.Unlock()

	if verbose {
		proto := "tcp"
		if useTLS {
			proto = "tls"
		}
		fmt.Fprintf(os.Stderr, "[mzv] net: connecting %s to %s...\n", proto, addr)
	}

	var conn net.Conn
	var err error

	if useTLS {
		host := addr
		if idx := strings.Index(addr, ":"); idx >= 0 {
			host = addr[:idx]
		}
		conn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp", addr,
			&tls.Config{ServerName: host},
		)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "no such host") {
			n.status = 0x02 // DNS error
		} else if strings.Contains(errStr, "refused") {
			n.status = 0x03 // connection refused
		} else {
			n.status = 0x04 // timeout / other
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "[mzv] net: connect failed: %v\n", err)
		}
		return
	}

	n.conn = conn
	n.rxBuf = make(chan byte, 8192)
	n.status = 0x01 // connected
	go n.readPump()

	if verbose {
		fmt.Fprintf(os.Stderr, "[mzv] net: connected to %s\n", addr)
	}
}

func (n *netHost) doDisconnect() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}
	n.status = 0xFF
}

func (n *netHost) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}
}
