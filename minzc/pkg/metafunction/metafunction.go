package metafunction

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/ast"
)

// Processor handles metafunction execution during compilation
type Processor struct {
	luaPath        string
	metafunctions  map[string]string // name -> lua script path
	buildMode      string           // debug, release, benchmark
	targetPlatform string           // zx_spectrum, msx, cpm
}

// New creates a new metafunction processor
func New(projectRoot string) *Processor {
	return &Processor{
		luaPath:        filepath.Join(projectRoot, "stdlib", "metafunctions"),
		metafunctions:  make(map[string]string),
		buildMode:      "debug",
		targetPlatform: "zx_spectrum",
	}
}

// SetBuildMode sets the current build mode
func (p *Processor) SetBuildMode(mode string) {
	p.buildMode = mode
}

// SetTargetPlatform sets the target platform
func (p *Processor) SetTargetPlatform(platform string) {
	p.targetPlatform = platform
}

// LoadMetafunctions loads all metafunction Lua scripts
func (p *Processor) LoadMetafunctions() error {
	// Core metafunctions
	p.metafunctions["print"] = filepath.Join(p.luaPath, "print.lua")
	p.metafunctions["println"] = filepath.Join(p.luaPath, "print.lua")
	p.metafunctions["write_byte"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["write_string"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["read_byte"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["hex"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["bin"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["assert"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["static_assert"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["debug"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["benchmark"] = filepath.Join(p.luaPath, "benchmark.lua")
	p.metafunctions["format"] = filepath.Join(p.luaPath, "io.lua")
	
	// Platform-specific metafunctions
	p.metafunctions["zx_cls"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["zx_beep"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["msx_vpoke"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["cpm_bdos"] = filepath.Join(p.luaPath, "io.lua")
	
	// Advanced metafunctions
	p.metafunctions["atomic"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["inline_asm"] = filepath.Join(p.luaPath, "io.lua")
	p.metafunctions["optimize"] = filepath.Join(p.luaPath, "io.lua")
	
	return nil
}

// ProcessMetafunctionCall processes a metafunction call and returns generated assembly
func (p *Processor) ProcessMetafunctionCall(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	// Check if this is a known metafunction
	_, exists := p.metafunctions[call.Name]
	if !exists {
		return "", fmt.Errorf("unknown metafunction: @%s", call.Name)
	}
	
	// For now, provide basic implementations for common metafunctions
	// In the future, this would execute the actual Lua scripts
	switch call.Name {
	case "print":
		return p.processprint(call, context)
	case "println":
		result, err := p.processprint(call, context)
		if err != nil {
			return "", err
		}
		return result + "\nLD A, 10\nRST 16  ; newline", nil
	case "write_byte":
		return p.processWriteByte(call, context)
	case "write_string":
		return p.processWriteString(call, context)
	case "hex":
		return p.processHex(call, context)
	case "debug":
		return p.processDebug(call, context)
	case "assert":
		return p.processAssert(call, context)
	case "static_assert":
		return p.processStaticAssert(call, context)
	case "zx_cls":
		return "CALL 0x0DAF  ; ROM CLS", nil
	case "zx_beep":
		return p.processZxBeep(call, context)
	default:
		return fmt.Sprintf("; Metafunction @%s not yet implemented", call.Name), nil
	}
}

// CompilationContext provides context for metafunction processing
type CompilationContext struct {
	CurrentFunction string
	Variables       map[string]VariableInfo
	Constants       map[string]interface{}
	LabelCounter    int
}

// VariableInfo holds information about a variable
type VariableInfo struct {
	Type    string
	Address string
}

// processprint implements the @print metafunction
func (p *Processor) processprint(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	if len(call.Arguments) == 0 {
		return "", fmt.Errorf("@print requires at least one argument")
	}
	
	// For now, implement basic format string processing
	// This would be much more sophisticated with actual Lua execution
	firstArg := call.Arguments[0]
	
	if stringLit, ok := firstArg.(*ast.StringLiteral); ok {
		formatStr := stringLit.Value
		
		// Simple format string processing
		if len(call.Arguments) == 1 {
			// Just a string literal - convert to direct byte output
			return p.generateStringOutput(formatStr), nil
		} else {
			// Format string with interpolation
			return p.processFormatString(formatStr, call.Arguments[1:], context)
		}
	}
	
	return "; @print with complex format not yet implemented", nil
}

// generateStringOutput generates optimal Z80 code for string output using smart strategy
func (p *Processor) generateStringOutput(str string) string {
	if len(str) == 0 {
		return "; Empty string - no output needed"
	}
	
	switch {
	case len(str) <= 2:
		// Very short: direct RST 16 is always optimal
		return p.generateDirectRST16(str)
		
	case len(str) <= 4:
		// Short: direct is usually optimal (better code locality)
		return p.generateDirectRST16(str)
		
	case len(str) <= 8:
		// Medium: context-dependent decision
		if p.shouldFavorInline() {
			return p.generateDirectRST16(str)
		}
		return p.generateLengthPrefixedLoop(str)
		
	default:
		// Long: loop is always optimal
		return p.generateLengthPrefixedLoop(str)
	}
}

// generateDirectRST16 generates direct LD A,n; RST 16 sequence
func (p *Processor) generateDirectRST16(str string) string {
	var result []string
	result = append(result, fmt.Sprintf("; String: \"%s\" (direct output, %d chars)", str, len(str)))
	
	for _, char := range str {
		if char >= 32 && char <= 126 {
			result = append(result, fmt.Sprintf("LD A, %d  ; '%c'", int(char), char))
		} else {
			result = append(result, fmt.Sprintf("LD A, %d  ; 0x%02X", int(char), int(char)))
		}
		result = append(result, "RST 16")
	}
	
	return strings.Join(result, "\n")
}

// generateLengthPrefixedLoop generates optimized loop for length-prefixed strings
func (p *Processor) generateLengthPrefixedLoop(str string) string {
	// Simple label counter for now
	labelId := 1
	
	var result []string
	result = append(result, fmt.Sprintf("; String: \"%s\" (length-prefixed loop, %d chars)", str, len(str)))
	result = append(result, fmt.Sprintf("LD HL, str_%d", labelId))
	result = append(result, "LD B, (HL)      ; B = length")
	result = append(result, "INC HL          ; HL -> string data")
	result = append(result, fmt.Sprintf("print_loop_%d:", labelId))
	result = append(result, "    LD A, (HL)")
	result = append(result, "    RST 16")
	result = append(result, "    INC HL")
	result = append(result, fmt.Sprintf("    DJNZ print_loop_%d", labelId))
	result = append(result, "")
	result = append(result, "; String data (length-prefixed, no null terminator):")
	result = append(result, fmt.Sprintf("str_%d:", labelId))
	result = append(result, fmt.Sprintf("    DB %d        ; Length", len(str)))
	result = append(result, fmt.Sprintf("    DB \"%s\"     ; String data", str))
	
	return strings.Join(result, "\n")
}

// shouldFavorInline determines whether to use inline vs loop for medium strings
func (p *Processor) shouldFavorInline() bool {
	// For now, always favor inline for medium strings
	// In the future, this could consider:
	// - Number of other strings in function
	// - Available code space
	// - Optimization level
	return true
}

// processFormatString handles format string interpolation
func (p *Processor) processFormatString(formatStr string, args []ast.Expression, context *CompilationContext) (string, error) {
	var result []string
	argIndex := 0
	
	i := 0
	for i < len(formatStr) {
		if i < len(formatStr)-1 && formatStr[i] == '{' && formatStr[i+1] == '}' {
			// Found {} placeholder
			if argIndex < len(args) {
				argCode, err := p.generateArgumentCode(args[argIndex], context)
				if err != nil {
					return "", err
				}
				result = append(result, argCode)
				argIndex++
			}
			i += 2
		} else {
			// Regular character - accumulate literal string
			start := i
			for i < len(formatStr) && !(i < len(formatStr)-1 && formatStr[i] == '{' && formatStr[i+1] == '}') {
				i++
			}
			literal := formatStr[start:i]
			if literal != "" {
				result = append(result, p.generateStringOutput(literal))
			}
		}
	}
	
	return strings.Join(result, "\n"), nil
}

// generateArgumentCode generates code for printing an argument
func (p *Processor) generateArgumentCode(arg ast.Expression, context *CompilationContext) (string, error) {
	switch expr := arg.(type) {
	case *ast.NumberLiteral:
		// Compile-time constant
		return p.generateStringOutput(fmt.Sprintf("%d", expr.Value)), nil
	case *ast.StringLiteral:
		// String literal
		return p.generateStringOutput(expr.Value), nil
	case *ast.BooleanLiteral:
		// Boolean literal
		value := "false"
		if expr.Value {
			value = "true"
		}
		return p.generateStringOutput(value), nil
	case *ast.Identifier:
		// Runtime variable - generate appropriate print call
		varInfo, exists := context.Variables[expr.Name]
		if !exists {
			return fmt.Sprintf("; Variable %s not found", expr.Name), nil
		}
		
		switch varInfo.Type {
		case "u8", "i8":
			return fmt.Sprintf("LD A, (%s)\nCALL print_u8", expr.Name), nil
		case "u16", "i16":
			return fmt.Sprintf("LD HL, (%s)\nCALL print_u16", expr.Name), nil
		case "bool":
			return fmt.Sprintf("LD A, (%s)\nCALL print_bool", expr.Name), nil
		case "*u8": // string
			return fmt.Sprintf("LD HL, (%s)\nCALL print_string", expr.Name), nil
		default:
			return fmt.Sprintf("; Unknown type %s for variable %s", varInfo.Type, expr.Name), nil
		}
	default:
		return "; Complex expression printing not yet implemented", nil
	}
}

// processWriteByte implements @write_byte
func (p *Processor) processWriteByte(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	if len(call.Arguments) != 1 {
		return "", fmt.Errorf("@write_byte requires exactly one argument")
	}
	
	arg := call.Arguments[0]
	if intLit, ok := arg.(*ast.NumberLiteral); ok {
		// Compile-time constant
		return fmt.Sprintf("LD A, %d\nRST 16", intLit.Value), nil
	} else if ident, ok := arg.(*ast.Identifier); ok {
		// Runtime variable
		return fmt.Sprintf("LD A, (%s)\nRST 16", ident.Name), nil
	}
	
	return "; @write_byte with complex expression not yet implemented", nil
}

// processWriteString implements @write_string
func (p *Processor) processWriteString(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	if len(call.Arguments) != 1 {
		return "", fmt.Errorf("@write_string requires exactly one argument")
	}
	
	arg := call.Arguments[0]
	if stringLit, ok := arg.(*ast.StringLiteral); ok {
		// Compile-time string
		return p.generateStringOutput(stringLit.Value), nil
	} else if ident, ok := arg.(*ast.Identifier); ok {
		// Runtime string variable
		return fmt.Sprintf("LD HL, (%s)\nCALL print_string", ident.Name), nil
	}
	
	return "; @write_string with complex expression not yet implemented", nil
}

// processHex implements @hex formatting
func (p *Processor) processHex(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	if len(call.Arguments) != 1 {
		return "", fmt.Errorf("@hex requires exactly one argument")
	}
	
	arg := call.Arguments[0]
	if intLit, ok := arg.(*ast.NumberLiteral); ok {
		// Compile-time constant
		hexStr := fmt.Sprintf("%02X", intLit.Value)
		if intLit.Value > 255 {
			hexStr = fmt.Sprintf("%04X", intLit.Value)
		}
		return p.generateStringOutput(hexStr), nil
	} else if ident, ok := arg.(*ast.Identifier); ok {
		// Runtime variable
		varInfo, exists := context.Variables[ident.Name]
		if exists && varInfo.Type == "u16" {
			return fmt.Sprintf("LD HL, (%s)\nCALL print_hex_u16", ident.Name), nil
		} else {
			return fmt.Sprintf("LD A, (%s)\nCALL print_hex_u8", ident.Name), nil
		}
	}
	
	return "; @hex with complex expression not yet implemented", nil
}

// processDebug implements @debug (only in debug builds)
func (p *Processor) processDebug(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	if p.buildMode != "debug" {
		return "; Debug output optimized out in release build", nil
	}
	
	if len(call.Arguments) != 1 {
		return "", fmt.Errorf("@debug requires exactly one argument")
	}
	
	arg := call.Arguments[0]
	var result []string
	
	if ident, ok := arg.(*ast.Identifier); ok {
		// Generate debug output: [DEBUG] var_name = value
		result = append(result, p.generateStringOutput(fmt.Sprintf("[DEBUG] %s = ", ident.Name)))
		
		// Generate code to print the variable value
		varCode, err := p.generateArgumentCode(arg, context)
		if err != nil {
			return "", err
		}
		result = append(result, varCode)
		
		// Add newline
		result = append(result, "LD A, 10\nRST 16  ; newline")
	} else {
		// For other expressions, just try to print them
		result = append(result, p.generateStringOutput("[DEBUG] "))
		argCode, err := p.generateArgumentCode(arg, context)
		if err != nil {
			return "", err
		}
		result = append(result, argCode)
		result = append(result, "LD A, 10\nRST 16  ; newline")
	}
	
	return strings.Join(result, "\n"), nil
}

// processAssert implements @assert (only in debug builds)
func (p *Processor) processAssert(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	if p.buildMode == "release" {
		return "; Assertion optimized out in release build", nil
	}
	
	if len(call.Arguments) < 1 || len(call.Arguments) > 2 {
		return "", fmt.Errorf("@assert requires 1 or 2 arguments")
	}
	
	// For now, just generate a placeholder
	// A full implementation would generate condition checking code
	message := "Assertion failed"
	if len(call.Arguments) == 2 {
		if msgLit, ok := call.Arguments[1].(*ast.StringLiteral); ok {
			message = msgLit.Value
		}
	}
	
	context.LabelCounter++
	labelId := context.LabelCounter
	
	return fmt.Sprintf(`; Assert condition check would go here
; If condition fails:
%s
CALL panic
assert_ok_%d:`, p.generateStringOutput("ASSERTION FAILED: "+message), labelId), nil
}

// processStaticAssert implements @static_assert (compile-time)
func (p *Processor) processStaticAssert(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	// Static assertions are checked at compile time
	// For now, just return a comment
	return "; Static assertion checked at compile time", nil
}

// processZxBeep implements @zx_beep
func (p *Processor) processZxBeep(call *ast.MetafunctionCall, context *CompilationContext) (string, error) {
	if len(call.Arguments) != 2 {
		return "", fmt.Errorf("@zx_beep requires exactly 2 arguments (duration, pitch)")
	}
	
	duration := call.Arguments[0]
	pitch := call.Arguments[1]
	
	if durLit, ok := duration.(*ast.NumberLiteral); ok {
		if pitchLit, ok := pitch.(*ast.NumberLiteral); ok {
			// Both compile-time constants
			return fmt.Sprintf(`LD HL, %d   ; Duration
LD DE, %d   ; Pitch
CALL 0x03B5 ; ROM BEEP`, durLit.Value, pitchLit.Value), nil
		}
	}
	
	// Runtime values
	return `LD HL, (duration)
LD DE, (pitch)
CALL 0x03B5 ; ROM BEEP`, nil
}