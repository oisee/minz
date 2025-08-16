# MZA Compatibility Baseline Report
*Generated: 2025-08-16*

## 📊 Executive Summary

**Current Status**: 🚨 **CRITICAL COMPATIBILITY ISSUES IDENTIFIED**

- **Total .a80 files**: 2,014 (massive scale)
- **Initial test sample**: 28 files  
- **Compatibility rate**: **0%** (0/28 files compatible)
- **MZA failures**: High (undefined symbols)
- **SjASMPlus failures**: Moderate (syntax/label issues)

## 🔍 Key Findings

### 1. **Fundamental Architectural Differences**

#### MZA (Strict)
- ✅ Requires ALL symbols to be defined
- ✅ Strict error checking  
- ❌ Fails on undefined `print_string`, etc.

#### SjASMPlus (Permissive)  
- ✅ Allows undefined symbols
- ❌ Rejects MinZ label syntax
- ❌ Complex expression parsing issues

### 2. **Critical Incompatibilities**

#### Label Naming Issues (SjASMPlus rejects)
```asm
; MinZ generates - SjASMPlus rejects:
expected.simple_add.add_numbers$u8$u8:          ; Invalid chars: . $
Users.alice.dev.zvdb-minz.zvdb_test.main:       ; Path-based labels
..hello_char.main:                               ; Double dots
```

#### Shadow Register Syntax (SjASMPlus rejects)
```asm
; MinZ generates - SjASMPlus rejects:
LD C', A         ; Shadow register syntax not supported
LD D', A         ; Alternative register syntax
```

#### String Escape Sequences (SjASMPlus rejects)
```asm
; MinZ generates - SjASMPlus rejects:
DB "Hello\'s"    ; Backslash escapes not recognized
```

#### Complex Labels and Expressions
```asm
; MinZ generates - SjASMPlus rejects:
str_14.eq_true_0:           ; Complex hierarchical labels
end_if_42.ge_done_24:       ; Nested conditional labels
```

### 3. **Test Infrastructure Success**

✅ **Created**: Automated differential testing harness  
✅ **Identified**: Root causes of incompatibility  
✅ **Established**: Baseline measurement process  

## 🎯 Priority Fix Areas

### Phase 1: Critical Symbol Issues
1. **Undefined Symbol Resolution**
   - MZA needs relaxed symbol resolution OR
   - MinZ needs to generate complete symbol tables

2. **Label Name Sanitization**  
   - Replace `.` with `_` in labels
   - Replace `$` with `_` in type signatures
   - Shorten complex hierarchical names

### Phase 2: Syntax Compatibility
3. **Shadow Register Translation**
   - Convert `LD C', A` to standard register usage
   - Map shadow registers to memory locations

4. **String Escape Handling**
   - Convert `\'` to `\x27` or remove escapes
   - Standardize string literal generation

### Phase 3: Advanced Features  
5. **Expression Evaluation**
   - Simplify complex label expressions
   - Use intermediate labels for nested conditions

## 🛠️ Implementation Strategy

### MZA Improvements Needed
1. **Symbol Resolution**: Add relaxed mode for incomplete symbols
2. **Error Reporting**: Clearer messages about missing symbols  
3. **Compatibility Mode**: SjASMPlus-compatible label generation

### MinZ Code Generation Improvements  
1. **Label Sanitization**: Clean generated label names
2. **Complete Symbol Tables**: Generate all required symbols
3. **Standard Register Usage**: Avoid shadow register syntax

## 📁 Test Infrastructure

### Created Files
- `scripts/test_mza_compatibility.sh` - Main test harness  
- `tests/asm/differential/` - Results directory
- `scripts/test_mza_simple.sh` - Simple compatibility test

### Test Categories Identified
1. **Simple programs** (basic opcodes) - Should work  
2. **Function calls** (undefined symbols) - MZA fails
3. **Complex labels** (hierarchical) - SjASMPlus fails  
4. **Shadow registers** (MinZ specific) - SjASMPlus fails

## 🎯 Success Metrics

### Immediate Goals (Week 1)
- [ ] Fix 5 most common label naming issues
- [ ] Implement undefined symbol relaxation in MZA
- [ ] Achieve 20% compatibility rate

### Short-term Goals (Month 1)  
- [ ] 80% compatibility on simple programs
- [ ] Complete symbol table generation
- [ ] Automated regression testing

### Long-term Goals (Release)
- [ ] 95%+ compatibility rate  
- [ ] Binary-identical output when compatible
- [ ] Zero regressions in main development

## 🤖 MCP AI Colleague Integration

**Available**: Multi-model analysis for assembly debugging
- `mcp__minz-ai__analyze_parser` - Assembly syntax issues
- `mcp__minz-ai__ask_gpt4` - Large file analysis  
- `mcp__minz-ai__ask_o4_mini` - Deep debugging

## 📋 Next Actions

1. **Label Sanitization** - Quick win for 50% of issues
2. **Symbol Resolution** - Core MZA improvement  
3. **Shadow Register Mapping** - MinZ codegen fix
4. **Automated Testing** - Prevent regressions

---

**Bottom Line**: MZA and SjASMPlus have fundamental architectural differences that require systematic fixes in both the assembler and code generator. The 0% compatibility rate is due to well-defined, solvable issues rather than insurmountable problems.

**Recommendation**: Focus on label sanitization first (quick wins), then tackle symbol resolution (architectural improvement).