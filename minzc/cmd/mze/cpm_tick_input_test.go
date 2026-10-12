package main

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/emulator"
)

func tickPipe(t *testing.T, interval time.Duration) (*cpmTickInput, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	return &cpmTickInput{fd: int(r.Fd()), interval: interval}, w
}

func TestCPMTickIdleBurstAndEOF(t *testing.T) {
	in, w := tickPipe(t, 15*time.Millisecond)
	start := time.Now()
	for i := 0; i < 3; i++ {
		if got := in.read(); got != 0 {
			t.Fatalf("idle = %d", got)
		}
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("idle ticks ran too quickly: %s", elapsed)
	}
	if _, err := w.Write([]byte("zxc")); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	for _, want := range []byte("zxc") {
		if got := in.read(); got != want {
			t.Fatalf("burst key = %d, want %d", got, want)
		}
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("queued keys accelerated ticks: %s", elapsed)
	}
	w.Close()
	start = time.Now()
	for i := 0; i < 3; i++ {
		if got := in.read(); got != 0 {
			t.Fatalf("EOF = %d", got)
		}
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("EOF accelerated ticks: %s", elapsed)
	}
}

func TestCPMTickEscapeContinuations(t *testing.T) {
	in, w := tickPipe(t, 100*time.Millisecond)
	w.Write([]byte("\x1b[Aq"))
	if got := in.read(); got != 27 {
		t.Fatal(got)
	}
	start := time.Now()
	if got := in.read(); got != '[' {
		t.Fatal(got)
	}
	if got := in.read(); got != 'A' {
		t.Fatal(got)
	}
	if elapsed := time.Since(start); elapsed >= 90*time.Millisecond {
		t.Fatalf("arrow continuation consumed game ticks: %s", elapsed)
	}
	if got := in.read(); got != 'q' {
		t.Fatal(got)
	}
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
		t.Fatalf("arrow advanced the game deadline: %s", elapsed)
	}
	w.Write([]byte{27})
	if got := in.read(); got != 27 {
		t.Fatal(got)
	}
	start = time.Now()
	if got := in.read(); got != 0 {
		t.Fatal(got)
	}
	if elapsed := time.Since(start); elapsed < 35*time.Millisecond || elapsed > time.Second {
		t.Fatalf("standalone Escape timeout: %s", elapsed)
	}
	if in.escape != 0 {
		t.Fatal("decoder state did not reset")
	}
}

func TestCPMInputOptionsNative(t *testing.T) {
	// Execute real CALL 5 instructions through the production BDOS handler.
	for _, tc := range []struct {
		name        string
		tick        uint
		quiet       bool
		input, want string
	}{
		{"ordinary_echo", 0, false, "z", "z"},
		{"ordinary_noecho", 0, true, "z", ""},
		{"timed_echo", 5, false, "z", "z"},
		{"timed_noecho", 5, true, "z", ""},
		{"timed_idle", 5, false, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			outR, outW, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			oldIn, oldOut := os.Stdin, os.Stdout
			oldTick, oldQuiet := cpmInputTickMS, cpmNoEcho
			defer func() {
				os.Stdin, os.Stdout = oldIn, oldOut
				cpmInputTickMS, cpmNoEcho = oldTick, oldQuiet
				r.Close()
				w.Close()
				outR.Close()
				outW.Close()
			}()
			os.Stdin, os.Stdout = r, outW
			cpmInputTickMS, cpmNoEcho = tc.tick, tc.quiet
			w.Write([]byte(tc.input))
			w.Close()
			z := emulator.NewRemogattoZ80WithScreen()
			setupCPMBDOS(z)
			// LD C,1 / CALL 5 / LD (0200),A / DI / HALT
			z.LoadAt(0x100, []byte{0x0e, 1, 0xcd, 5, 0, 0x32, 0, 2, 0xf3, 0x76})
			z.SetPC(0x100)
			z.SetSP(0xfe00)
			setExecutionLimit(z.RemogattoZ80, 10000)
			if err := z.Execute(); err != nil {
				t.Fatal(err)
			}
			wantKey := byte(0)
			if tc.input != "" {
				wantKey = tc.input[0]
			}
			if got := z.ReadMemory(0x200); got != wantKey {
				t.Fatalf("native key %d, want %d", got, wantKey)
			}
			outW.Close()
			got, err := io.ReadAll(outR)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("echo %q, want %q", got, tc.want)
			}
		})
	}
}
