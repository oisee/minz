// MIR Backend - outputs MIR text format for MZV execution
package codegen

import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/ir"
)

// MIRBackend generates MIR text output for the MZV virtual machine
type MIRBackend struct {
	options *BackendOptions
}

// NewMIRBackend creates a new MIR backend
func NewMIRBackend(options *BackendOptions) *MIRBackend {
	if options == nil {
		options = &BackendOptions{}
	}
	return &MIRBackend{options: options}
}

func (b *MIRBackend) Name() string {
	return "mir"
}

func (b *MIRBackend) GetFileExtension() string {
	return ".mir"
}

func (b *MIRBackend) SupportsFeature(feature string) bool {
	switch feature {
	case FeatureFixedPoint:
		return true
	case FeatureIndirectCalls:
		return true
	default:
		return false
	}
}

// Generate outputs the MIR module as text
func (b *MIRBackend) Generate(module *ir.Module) (string, error) {
	// Disable SMC for MIR output - MIR VM doesn't support SMC
	for _, fn := range module.Functions {
		fn.IsSMCEnabled = false
		fn.IsSMCDefault = false
	}

	var sb strings.Builder

	// Write header
	sb.WriteString("; MinZ Intermediate Representation (MIR)\n")
	sb.WriteString(fmt.Sprintf("; Module: %s\n\n", module.Name))

	// Write globals if any
	if len(module.Globals) > 0 {
		sb.WriteString("; Globals:\n")
		for _, g := range module.Globals {
			sb.WriteString(fmt.Sprintf(";   %s: %s\n", g.Name, g.Type.String()))
		}
		sb.WriteString("\n")
	}

	// Write each function
	for _, fn := range module.Functions {
		sb.WriteString(fmt.Sprintf("Function %s(", fn.Name))
		for i, param := range fn.Params {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s: %s", param.Name, param.Type.String()))
		}
		sb.WriteString(fmt.Sprintf(") -> %s\n", fn.ReturnType.String()))

		// SMC flag
		if fn.IsSMCEnabled {
			sb.WriteString("  @smc\n")
		}

		// Locals
		if len(fn.Locals) > 0 {
			sb.WriteString("  Locals:\n")
			for _, local := range fn.Locals {
				sb.WriteString(fmt.Sprintf("    r%d = %s: %s\n", local.Reg, local.Name, local.Type.String()))
			}
		}

		// Instructions
		sb.WriteString("  Instructions:\n")
		for i, inst := range fn.Instructions {
			sb.WriteString(fmt.Sprintf("    %3d: %s\n", i, inst.String()))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func init() {
	RegisterBackend("mir", func(options *BackendOptions) Backend {
		return NewMIRBackend(options)
	})
}
