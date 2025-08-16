# TODO: MZA Z80 Assembler Improvements

## Current Status: 🚧 Encoder Fixes In Progress

**Last Updated**: 2025-08-16  
**Success Rate**: 1-2% → Target: 15-25%  
**Phase**: Implementation & Integration

---

## ✅ Completed Achievements

### Investigation & Analysis
- [x] **End-to-end corpus testing** - Tested 2,020+ .a80 files against MZA and SjASMPlus
- [x] **Label sanitization analysis** - Proved syntax isn't the primary issue (minimal improvement)
- [x] **Z80 instruction research** - Complete instruction set including undocumented opcodes
- [x] **Synthetic test suite** - Created `supertest_z80.a80` with 700+ lines covering all categories
- [x] **Root cause identification** - Memory LD instructions fail in encoder, not parser

### AI Colleague Consultation Success ✨
- [x] **Architectural decision** - Consulted GPT-4 and o4-mini on ANTLR vs hand-written parser
- [x] **Consensus achieved** - Both recommended keeping hand-written parser, fixing encoder
- [x] **Guidelines documented** - Added AI consultation best practices to CLAUDE.md
- [x] **Development time saved** - Avoided costly ANTLR migration that wouldn't solve core issue

### Infrastructure & Testing
- [x] **Test infrastructure** - Comprehensive testing scripts and validation tools
- [x] **Corpus sanitization** - Label compatibility processing for cross-assembler testing  
- [x] **SjASMPlus validation** - Confirmed syntax correctness against reference assembler
- [x] **Regression detection** - Systematic test progression to isolate issues

### Critical Bug Fixes
- [x] **Local label handling** - Fixed MinZ hierarchical label detection (`.` vs `...`)
- [x] **OpAddr16 operand matching** - Fixed `($F000)` syntax recognition  
- [x] **Operand classification** - Implemented semantic indirect addressing distinction

---

## 🚧 In Progress

### Encoder Integration (95% Complete)
- [x] Operand classification system implementation
- [x] OpAddr16 matching fixes
- [ ] **CURRENT**: Integration without breaking `(HL)` register indirect
- [ ] Final encoder routing validation
- [ ] Regression test execution

### Immediate Next Steps
1. **Complete encoder routing** - Ensure `(HL)` works while enabling `($F000)`
2. **Integration testing** - Validate all LD instruction types work correctly
3. **Corpus retest** - Measure actual success rate improvement
4. **Documentation update** - Record final success metrics

---

## 📋 Planned Improvements

### Phase 1: Core Encoder Fixes (Current)
- [ ] Complete memory LD instruction support
- [ ] Validate against comprehensive test suite
- [ ] Document success rate improvement (target: 15-25%)
- [ ] Create encoder fix regression tests

### Phase 2: Table-Driven Encoder (Next)
> *As recommended by o4-mini AI colleague*

- [ ] **Instruction pattern definitions** - Systematic instruction → encoding mapping
- [ ] **Table-driven encoding** - Replace ad-hoc encoding with pattern matching
- [ ] **Operand AST system** - Emit structured operands instead of raw strings
- [ ] **Validation framework** - Automated encoder correctness verification

### Phase 3: Instruction Completeness
- [ ] **Supertest gap analysis** - Identify all failing instruction patterns
- [ ] **Undocumented instruction support** - IX/IY halves, SLL, NEG variants
- [ ] **Advanced addressing modes** - Complete (IX+d), (IY+d) support
- [ ] **Edge case handling** - Boundary conditions and error cases

### Phase 4: Performance & Polish
- [ ] **Large corpus optimization** - Performance improvements for 2,000+ file processing
- [ ] **Error message improvements** - Better diagnostics for encoding failures  
- [ ] **SjASMPlus compatibility mode** - Full syntax compatibility option
- [ ] **Instruction support matrix** - Complete documentation of supported features

---

## 🎯 Success Metrics

### Current Baseline
- **Original corpus**: MZA 2% vs SjASMPlus 0%
- **Sanitized corpus**: MZA 1% vs SjASMPlus 1%
- **Test coverage**: 700+ Z80 instructions in synthetic test

### Target Goals
- **Phase 1**: 15-25% success rate (memory LD instructions fixed)
- **Phase 2**: 40-60% success rate (table-driven encoder)
- **Phase 3**: 80-90% success rate (complete instruction support)
- **Phase 4**: 95%+ success rate (SjASMPlus compatibility)

### Validation Strategy
- Comprehensive corpus retesting after each phase
- Synthetic test suite validation (supertest_z80.a80)
- Cross-validation against SjASMPlus reference behavior
- Regression testing for core functionality preservation

---

## 🔧 Technical Debt & Known Issues

### Immediate Issues
- **Register indirect regression** - `(HL)` failing during encoder fix integration
- **Symbol resolution errors** - `($F000)` treated as symbol rather than address
- **Operand routing conflicts** - Need clean separation of register vs memory indirect

### Architectural Improvements Needed
- **Encoder architecture** - Move from ad-hoc to systematic pattern-based encoding
- **Operand processing** - Structured operand AST instead of string manipulation
- **Error handling** - Better error messages and failure diagnostics
- **Test coverage** - Automated regression testing for encoder changes

### Performance Considerations
- **Corpus processing time** - Large-scale testing optimization needed
- **Memory usage** - Efficient handling of 2,000+ file processing
- **Incremental testing** - Faster feedback loops for development

---

## 📚 Documentation & Resources

### Created Documentation
- **Complete investigation report** - `inbox/MZA_Encoder_Investigation_Complete_Report.md`
- **AI consultation guidelines** - Added to `CLAUDE.md`
- **Test suite documentation** - Comprehensive Z80 instruction coverage
- **Technical analysis** - Encoder vs parser issue identification

### Reference Materials
- **Z80 instruction set** - Complete including undocumented opcodes
- **SjASMPlus compatibility** - Syntax and behavior reference
- **MinZ hierarchical labels** - Understanding and handling methodology
- **Encoder pattern analysis** - Current implementation gaps and fixes

### Test Artifacts
- **Synthetic test suite** - `supertest_z80.a80` (700+ lines)
- **Corpus sanitization** - Label compatibility processing tools
- **Validation scripts** - Comprehensive testing infrastructure
- **Progression tests** - Systematic instruction type isolation

---

## 🎉 Key Achievements

### AI-Driven Decision Making Success
The AI colleague consultation for the ANTLR vs hand-written parser decision was **highly successful**:
- **Prevented costly mistake** - ANTLR migration wouldn't solve the core problem
- **Guided to correct solution** - Focus on encoder fixes rather than parser replacement  
- **Saved development time** - Weeks of work avoided through expert consultation
- **Validated methodology** - Multi-model consensus for critical architectural decisions

### Technical Problem Solving
- **Systematic approach** - From 2,020 files to specific instruction identification
- **Hypothesis testing** - Label sanitization disproved syntax hypothesis
- **Root cause analysis** - Parser vs encoder issue distinction
- **Targeted implementation** - Surgical fixes rather than architectural overhaul

### Infrastructure Development
- **Comprehensive testing** - End-to-end corpus validation methodology
- **Cross-assembler validation** - SjASMPlus compatibility verification
- **Synthetic test coverage** - Complete Z80 instruction set validation
- **Regression detection** - Systematic testing for change impact assessment

---

## 📞 Next Actions

### Current Session Priority
1. **Complete encoder integration** - Fix `(HL)` regression while preserving `($F000)` support
2. **Validate instruction types** - Ensure all LD variants work correctly  
3. **Measure improvement** - Retest corpus to quantify success rate gains
4. **Document results** - Update metrics and create final implementation report

### Future Session Planning
1. **Table-driven encoder design** - Implement o4-mini's systematic approach recommendation
2. **Instruction gap analysis** - Process supertest results for remaining missing patterns
3. **Performance optimization** - Large corpus processing improvements
4. **SjASMPlus compatibility** - Full feature parity achievement

---

*This TODO reflects the comprehensive MZA improvement effort, from investigation through implementation, guided by AI colleague consultation and systematic problem-solving methodology.*