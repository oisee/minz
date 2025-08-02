package semantic

// TODO: Interface monomorphization support - currently disabled due to missing types
// This file will be re-enabled when ir.Value, ir.BasicBlock, ir.CallInst are implemented

/*
import (
	"fmt"
	"strings"

	"github.com/minz/minzc/pkg/ast"
	"github.com/minz/minzc/pkg/ir"
)
*/

/*
// InterfaceMonomorphizer handles monomorphization of generic functions with interface bounds
type InterfaceMonomorphizer struct {
	analyzer          *Analyzer
	specializations   map[string]*ir.Function // key: mangled name
	pendingCalls      []pendingCall
	processedTypes    map[string]bool
}

type pendingCall struct {
	funcName     string
	typeArgs     []ir.Type
	interfaceReq []string // Required interfaces
}

// NewInterfaceMonomorphizer creates a new monomorphizer
func NewInterfaceMonomorphizer(analyzer *Analyzer) *InterfaceMonomorphizer {
	return &InterfaceMonomorphizer{
		analyzer:        analyzer,
		specializations: make(map[string]*ir.Function),
		processedTypes:  make(map[string]bool),
	}
}

// MonomorphizeInterfaceCalls processes all interface-based generic calls
func (m *InterfaceMonomorphizer) MonomorphizeInterfaceCalls(module *ir.Module) error {
	// Find all generic function calls
	for _, fn := range module.Functions {
		if err := m.findGenericCalls(fn); err != nil {
			return err
		}
	}

	// Process pending calls until none remain
	for len(m.pendingCalls) > 0 {
		call := m.pendingCalls[0]
		m.pendingCalls = m.pendingCalls[1:]

		if err := m.processCall(call, module); err != nil {
			return err
		}
	}

	return nil
}

// findGenericCalls finds all calls to generic functions in a function
func (m *InterfaceMonomorphizer) findGenericCalls(fn *ir.Function) error {
	for _, bb := range fn.Body {
		for _, inst := range bb.Instructions {
			switch inst := inst.(type) {
			case *ir.CallInst:
				// Check if this is a generic function call
				if isGenericCall(inst.Function) {
					// Determine concrete types from arguments
					typeArgs := m.inferTypeArgs(inst)
					if typeArgs != nil {
						m.pendingCalls = append(m.pendingCalls, pendingCall{
							funcName: inst.Function,
							typeArgs: typeArgs,
						})
					}
				}
			}
		}
	}
	return nil
}

// processCall creates a specialized version of a generic function
func (m *InterfaceMonomorphizer) processCall(call pendingCall, module *ir.Module) error {
	// Generate mangled name for this specialization
	mangledName := m.mangleName(call.funcName, call.typeArgs)

	// Check if we already created this specialization
	if _, exists := m.specializations[mangledName]; exists {
		return nil
	}

	// Find the generic function template
	var genericFunc *ast.FuncDecl
	// This would need to be stored during semantic analysis
	// For now, we'll assume we can retrieve it

	// Create specialized function
	specializedFunc := &ir.Function{
		Name:       mangledName,
		Parameters: make([]*ir.Parameter, 0),
		Body:       make([]*ir.BasicBlock, 0),
		// Copy other fields from generic template
	}

	// Replace type parameters with concrete types
	// Transform interface method calls to direct calls
	// For example: shape.draw() -> Circle_draw(shape)

	m.specializations[mangledName] = specializedFunc
	module.Functions = append(module.Functions, specializedFunc)

	// Update call sites to use specialized version
	m.updateCallSites(call.funcName, call.typeArgs, mangledName, module)

	return nil
}

// mangleName creates a unique name for a specialization
func (m *InterfaceMonomorphizer) mangleName(funcName string, typeArgs []ir.Type) string {
	parts := []string{funcName}
	for _, t := range typeArgs {
		parts = append(parts, strings.ReplaceAll(t.String(), " ", "_"))
	}
	return strings.Join(parts, "_")
}

// inferTypeArgs infers concrete type arguments from a call
func (m *InterfaceMonomorphizer) inferTypeArgs(call *ir.CallInst) []ir.Type {
	// This would analyze the argument types to determine
	// what concrete types are being used
	// For now, return nil
	return nil
}

// updateCallSites updates all calls to use the specialized version
func (m *InterfaceMonomorphizer) updateCallSites(genericName string, typeArgs []ir.Type, specializedName string, module *ir.Module) {
	mangledName := m.mangleName(genericName, typeArgs)
	
	for _, fn := range module.Functions {
		for _, bb := range fn.Body {
			for i, inst := range bb.Instructions {
				if call, ok := inst.(*ir.CallInst); ok && call.Function == genericName {
					// Check if types match
					inferredTypes := m.inferTypeArgs(call)
					if typesMatch(inferredTypes, typeArgs) {
						// Update to use specialized version
						bb.Instructions[i] = &ir.CallInst{
							Function: specializedName,
							Args:     call.Args,
							Result:   call.Result,
						}
					}
				}
			}
		}
	}
}

// TransformInterfaceCall transforms an interface method call to a direct call
func (m *InterfaceMonomorphizer) TransformInterfaceCall(
	receiver ir.Type,
	methodName string,
	args []ir.Value,
) (*ir.CallInst, error) {
	// Determine concrete type of receiver
	concreteType := m.getConcreteType(receiver)
	if concreteType == nil {
		return nil, fmt.Errorf("cannot determine concrete type for interface call")
	}

	// Generate direct function name
	// e.g., Circle_draw for Circle.draw()
	directFuncName := fmt.Sprintf("%s_%s", concreteType.String(), methodName)

	// Create direct call
	return &ir.CallInst{
		Function: directFuncName,
		Args:     args,
		// Result will be set by caller
	}, nil
}

// getConcreteType determines the concrete type of a value
func (m *InterfaceMonomorphizer) getConcreteType(t ir.Type) ir.Type {
	// In a monomorphized context, we know the concrete type
	// This information would be tracked during specialization
	return t
}

// Helper functions

func isGenericCall(funcName string) bool {
	// Check if function has type parameters
	// This info would be stored during semantic analysis
	return false
}

func typesMatch(t1, t2 []ir.Type) bool {
	if len(t1) != len(t2) {
		return false
	}
	for i := range t1 {
		if t1[i].String() != t2[i].String() {
			return false
		}
	}
	return true
}
*/