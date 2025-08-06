package meta

import (
	"fmt"
	"os"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"github.com/minz/minzc/pkg/ast"
)

// LuaEvaluator performs compile-time evaluation using embedded Lua
type LuaEvaluator struct {
	L          *lua.LState
	constants  map[string]interface{}
	generators map[string]string // Generated code snippets
}

// NewLuaEvaluator creates a new Lua-based evaluator
func NewLuaEvaluator() *LuaEvaluator {
	L := lua.NewState()
	
	evaluator := &LuaEvaluator{
		L:          L,
		constants:  make(map[string]interface{}),
		generators: make(map[string]string),
	}
	
	// Set up MinZ-specific Lua functions
	evaluator.setupMinzAPI()
	
	return evaluator
}

// Close cleans up the Lua state
func (e *LuaEvaluator) Close() {
	e.L.Close()
}

// EvaluateExpression evaluates a simple expression and returns its string representation
func (e *LuaEvaluator) EvaluateExpression(expr string) (string, error) {
	// Wrap expression in return statement
	code := fmt.Sprintf("return (%s)", expr)
	
	// Execute the Lua code
	if err := e.L.DoString(code); err != nil {
		return "", fmt.Errorf("failed to evaluate expression: %w", err)
	}
	
	// Get the result from the stack
	result := e.L.Get(-1)
	e.L.Pop(1)
	
	// Convert to string
	switch v := result.(type) {
	case lua.LNumber:
		// Format as integer if it's a whole number
		if float64(int64(v)) == float64(v) {
			return fmt.Sprintf("%d", int64(v)), nil
		}
		return fmt.Sprintf("%g", float64(v)), nil
	case lua.LString:
		return string(v), nil
	case lua.LBool:
		if bool(v) {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("unsupported result type: %T", result)
	}
}

// setupMinzAPI adds MinZ-specific functions to Lua
func (e *LuaEvaluator) setupMinzAPI() {
	// Add MinZ code generation helpers
	module := e.createMinzModule()
	e.L.SetGlobal("minz", module)
	
	// Add platform constants
	e.L.SetGlobal("PLATFORM", lua.LString("ZX_SPECTRUM"))
	e.L.SetGlobal("ARCH", lua.LString("Z80"))
}

// createMinzModule creates the 'minz' Lua module
func (e *LuaEvaluator) createMinzModule() *lua.LTable {
	module := e.L.NewTable()
	
	// Code generation functions
	e.L.SetField(module, "enum", e.L.NewFunction(e.luaGenerateEnum))
	e.L.SetField(module, "struct", e.L.NewFunction(e.luaGenerateStruct))
	e.L.SetField(module, "const_array", e.L.NewFunction(e.luaGenerateConstArray))
	
	// File I/O functions
	e.L.SetField(module, "save_bin", e.L.NewFunction(luaSaveBin))
	e.L.SetField(module, "load_bin", e.L.NewFunction(luaLoadBin))
	
	// Type helpers
	e.L.SetField(module, "u8", e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString("u8"))
		return 1
	}))
	e.L.SetField(module, "u16", e.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString("u16"))
		return 1
	}))
	
	return module
}

// EvaluateLuaBlock evaluates a Lua code block
func (e *LuaEvaluator) EvaluateLuaBlock(code string) error {
	return e.L.DoString(code)
}

// CallLuaFunction calls a Lua function and returns the result
func (e *LuaEvaluator) CallLuaFunction(name string, args ...interface{}) (interface{}, error) {
	// Get the function
	fn := e.L.GetGlobal(name)
	if fn == lua.LNil {
		return nil, fmt.Errorf("undefined Lua function: %s", name)
	}
	
	// Push arguments
	for _, arg := range args {
		e.L.Push(toLuaValue(e.L, arg))
	}
	
	// Call function
	err := e.L.PCall(len(args), 1, nil)
	if err != nil {
		return nil, err
	}
	
	// Get result
	result := e.L.Get(-1)
	e.L.Pop(1)
	
	return fromLuaValue(result), nil
}

// luaGenerateEnum generates an enum declaration
func (e *LuaEvaluator) luaGenerateEnum(L *lua.LState) int {
	name := L.CheckString(1)
	variants := L.CheckTable(2)
	
	code := fmt.Sprintf("enum %s {\n", name)
	
	variants.ForEach(func(k, v lua.LValue) {
		code += fmt.Sprintf("    %s,\n", v.String())
	})
	
	code += "}\n"
	
	L.Push(lua.LString(code))
	return 1
}

// luaGenerateStruct generates a struct declaration
func (e *LuaEvaluator) luaGenerateStruct(L *lua.LState) int {
	name := L.CheckString(1)
	fields := L.CheckTable(2)
	
	code := fmt.Sprintf("struct %s {\n", name)
	
	fields.ForEach(func(k, v lua.LValue) {
		if tbl, ok := v.(*lua.LTable); ok {
			fieldName := tbl.RawGetInt(1).String()
			fieldType := tbl.RawGetInt(2).String()
			code += fmt.Sprintf("    %s: %s,\n", fieldName, fieldType)
		}
	})
	
	code += "}\n"
	
	L.Push(lua.LString(code))
	return 1
}

// luaGenerateConstArray generates a const array declaration
func (e *LuaEvaluator) luaGenerateConstArray(L *lua.LState) int {
	name := L.CheckString(1)
	typ := L.CheckString(2)
	values := L.CheckTable(3)
	
	var elements []string
	values.ForEach(func(k, v lua.LValue) {
		elements = append(elements, v.String())
	})
	
	code := fmt.Sprintf("const %s: [%s; %d] = [%s];",
		name, typ, len(elements), strings.Join(elements, ", "))
	
	L.Push(lua.LString(code))
	return 1
}

// toLuaValue converts a Go value to a Lua value
func toLuaValue(L *lua.LState, val interface{}) lua.LValue {
	switch v := val.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case int:
		return lua.LNumber(v)
	case int64:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		tbl := L.NewTable()
		for i, elem := range v {
			tbl.RawSetInt(i+1, toLuaValue(L, elem))
		}
		return tbl
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, v := range v {
			tbl.RawSetString(k, toLuaValue(L, v))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

// fromLuaValue converts a Lua value to a Go value
func fromLuaValue(lval lua.LValue) interface{} {
	switch v := lval.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		// Check if it's an array or map
		isArray := true
		length := 0
		
		v.ForEach(func(k, val lua.LValue) {
			if _, ok := k.(lua.LNumber); !ok {
				isArray = false
			}
			length++
		})
		
		if isArray && length > 0 {
			// Convert to slice
			result := make([]interface{}, length)
			v.ForEach(func(k, val lua.LValue) {
				if idx, ok := k.(lua.LNumber); ok {
					result[int(idx)-1] = fromLuaValue(val)
				}
			})
			return result
		} else {
			// Convert to map
			result := make(map[string]interface{})
			v.ForEach(func(k, val lua.LValue) {
				result[k.String()] = fromLuaValue(val)
			})
			return result
		}
	default:
		return nil
	}
}

// Example usage in AST processing:

// ProcessLuaBlock processes @lua[[...]] blocks
func (e *LuaEvaluator) ProcessLuaBlock(node *ast.LuaBlock) error {
	return e.EvaluateLuaBlock(node.Code)
}

// ProcessLuaExpr processes @lua(...) expressions
func (e *LuaEvaluator) ProcessLuaExpr(node *ast.LuaExpr) (interface{}, error) {
	// Evaluate the Lua expression
	err := e.L.DoString("return " + node.Code)
	if err != nil {
		return nil, err
	}
	
	// Get the result
	result := e.L.Get(-1)
	e.L.Pop(1)
	
	return fromLuaValue(result), nil
}

// GenerateCodeFromLua generates MinZ code from a Lua function
func (e *LuaEvaluator) GenerateCodeFromLua(funcName string, args ...interface{}) (string, error) {
	result, err := e.CallLuaFunction(funcName, args...)
	if err != nil {
		return "", err
	}
	
	if code, ok := result.(string); ok {
		return code, nil
	}
	
	return "", fmt.Errorf("Lua function %s did not return a string", funcName)
}

// luaSaveBin saves binary data to a file at compile time
// Usage: minz.save_bin(filename, data)
// data can be:
//   - string: raw bytes
//   - table: array of bytes (0-255)
func luaSaveBin(L *lua.LState) int {
	filename := L.CheckString(1)
	dataValue := L.Get(2)
	
	var data []byte
	
	switch v := dataValue.(type) {
	case lua.LString:
		// String is treated as raw bytes
		data = []byte(string(v))
		
	case *lua.LTable:
		// Table is treated as array of bytes
		length := v.Len()
		data = make([]byte, 0, length)
		
		for i := 1; i <= length; i++ {
			val := v.RawGetInt(i)
			if num, ok := val.(lua.LNumber); ok {
				byteVal := int(num)
				if byteVal < 0 || byteVal > 255 {
					L.RaiseError("byte value out of range [0-255]: %d", byteVal)
					return 0
				}
				data = append(data, byte(byteVal))
			} else {
				L.RaiseError("table must contain only numbers (bytes)")
				return 0
			}
		}
		
	default:
		L.RaiseError("data must be string or table, got %s", dataValue.Type().String())
		return 0
	}
	
	// Write the file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		L.RaiseError("failed to write file %s: %v", filename, err)
		return 0
	}
	
	// Return number of bytes written
	L.Push(lua.LNumber(len(data)))
	return 1
}

// luaLoadBin loads binary data from a file at compile time
// Usage: data = minz.load_bin(filename)
// Returns: string containing the raw bytes
func luaLoadBin(L *lua.LState) int {
	filename := L.CheckString(1)
	
	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		L.RaiseError("failed to read file %s: %v", filename, err)
		return 0
	}
	
	// Return as string (raw bytes)
	L.Push(lua.LString(string(data)))
	return 1
}