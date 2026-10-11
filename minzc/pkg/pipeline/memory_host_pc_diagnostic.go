package pipeline

import "github.com/minz/minzc/pkg/z80asm"

// Isolated diagnostic wiring. It is deliberately not a public API or evidence
// field. This file and its two runner calls are not proposed for parent merge.
var memoryHostPCLayoutDiagnostic func(*z80asm.Result, []string, map[string]string) error
var memoryHostPCStepDiagnostic func(invocation int, pc uint16, cycles int, instruction []byte)
