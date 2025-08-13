# Translating ANTLR Grammar to MinZ: The Path to Self-Hosting

*August 13, 2025*

## Executive Summary

Can we translate the ANTLR4 MinZ grammar (597 lines, 121 rules) into MinZ itself for true self-hosting? This analysis explores three approaches: direct translation, parser combinators, and a hybrid PEG-style solution. **Verdict: Partially feasible with significant constraints.**

## Part 1: Understanding the Challenge

### Current ANTLR Grammar Statistics
- **Total lines**: 597
- **Parser rules**: 121
- **Lexer rules**: ~40
- **Complexity**: Medium (recursive, with precedence)
- **Features**: Expressions, statements, declarations, patterns

### ANTLR4 Runtime Requirements
The ANTLR4 Go runtime is approximately:
- **Runtime size**: ~200KB compiled
- **Memory usage**: 5-10MB for parsing
- **Parse tree nodes**: 100-200 bytes each
- **DFA tables**: 10-50KB depending on grammar

### Z80 Memory Constraints
- **Available RAM**: 48KB (ZX Spectrum)
- **After system**: ~40KB usable
- **With banking**: 128KB (Spectrum +2/+3)

## Part 2: Direct ANTLR Grammar Translation

### Approach 1: Literal Translation to MinZ

```minz
// MinZ.minz - Direct translation of ANTLR grammar
struct Rule {
    name: str,
    alternatives: [Alternative; 10],
    alt_count: u8,
}

struct Alternative {
    elements: [Element; 20],
    elem_count: u8,
}

struct Element {
    type: ElementType,  // Terminal, NonTerminal, Optional, Repeat
    value: u16,         // Index into symbol table
}

fun parse_source_file(input: *Token) -> *ParseTree {
    let tree = create_node("sourceFile");
    
    // (importStatement | declaration | statement)* EOF
    while !at_eof(input) {
        if looks_like_import(input) {
            add_child(tree, parse_import_statement(input));
        } else if looks_like_declaration(input) {
            add_child(tree, parse_declaration(input));
        } else {
            add_child(tree, parse_statement(input));
        }
    }
    
    expect_token(input, TOKEN_EOF);
    return tree;
}
```

**Memory Analysis**:
- Rule storage: 121 rules × 250 bytes = **30KB**
- Parse tree for medium program: **20-40KB**
- Symbol tables and buffers: **10KB**
- **Total: 60-80KB** ❌ Exceeds 48KB limit

### Approach 2: Table-Driven Parser

```minz
// Compact table-driven approach
const PARSE_TABLE: [[u8; 256]; 121] = [
    // State 0: sourceFile
    [SHIFT_1, SHIFT_2, REDUCE_3, ...],
    // State 1: importStatement
    [SHIFT_4, ERROR, REDUCE_5, ...],
    // ... 119 more states
];

fun parse_with_tables(input: *Token) -> bool {
    let stack: [u8; 100];  // Parse stack
    let sp = 0;
    let state = 0;
    
    while true {
        let token = peek_token(input);
        let action = PARSE_TABLE[state][token.type];
        
        case action & 0xF0 {
            ACTION_SHIFT => {
                stack[sp++] = state;
                state = action & 0x0F;
                consume_token(input);
            },
            ACTION_REDUCE => {
                let rule = action & 0x0F;
                sp = sp - rule_length[rule];
                state = stack[sp - 1];
                state = goto_table[state][rule_lhs[rule]];
            },
            ACTION_ACCEPT => return true,
            ACTION_ERROR => return false,
        }
    }
}
```

**Memory Analysis**:
- Parse table: 121 × 256 = **31KB**
- Goto table: ~**5KB**
- Stack and buffers: **2KB**
- **Total: 38KB** ✅ Fits in 48KB!

## Part 3: Parser Combinator Library in MinZ

### Design: Minimal Combinators

```minz
// parser_combinators.minz - Lightweight parsing library
type Parser<T> = fn(*Input) -> Result<T>;

struct Result<T> {
    success: bool,
    value: T,
    remaining: *Input,
}

// Core combinators
fun char(c: u8) -> Parser<u8> {
    return |input| => Result<u8> {
        if input.current() == c {
            return Result { 
                success: true, 
                value: c, 
                remaining: input.advance(1) 
            };
        }
        return Result { success: false };
    };
}

fun sequence<A, B>(p1: Parser<A>, p2: Parser<B>) -> Parser<(A, B)> {
    return |input| => Result<(A, B)> {
        let r1 = p1(input);
        if !r1.success { return Result { success: false }; }
        
        let r2 = p2(r1.remaining);
        if !r2.success { return Result { success: false }; }
        
        return Result {
            success: true,
            value: (r1.value, r2.value),
            remaining: r2.remaining
        };
    };
}

fun choice<T>(p1: Parser<T>, p2: Parser<T>) -> Parser<T> {
    return |input| => Result<T> {
        let r1 = p1(input);
        if r1.success { return r1; }
        return p2(input);
    };
}

fun many<T>(p: Parser<T>) -> Parser<[T]> {
    return |input| => Result<[T]> {
        let results: [T; 100];
        let count = 0;
        let current = input;
        
        loop {
            let r = p(current);
            if !r.success { break; }
            results[count++] = r.value;
            current = r.remaining;
        }
        
        return Result {
            success: true,
            value: results[0..count],
            remaining: current
        };
    };
}
```

### MinZ Grammar Using Combinators

```minz
// minz_grammar.minz - Grammar defined with combinators
fun identifier() -> Parser<Token> {
    return satisfy(|t| t.type == TOKEN_IDENTIFIER);
}

fun keyword(word: str) -> Parser<Token> {
    return satisfy(|t| t.type == TOKEN_KEYWORD && t.text == word);
}

fun function_declaration() -> Parser<FunctionDecl> {
    return sequence(
        optional(keyword("pub")),
        choice(keyword("fun"), keyword("fn")),
        identifier(),
        optional(generic_params()),
        between(char('('), parameter_list(), char(')')),
        optional(return_type()),
        block()
    ).map(|parts| => FunctionDecl {
        FunctionDecl {
            visibility: parts.0,
            name: parts.2,
            generics: parts.3,
            params: parts.4,
            return_type: parts.5,
            body: parts.6
        }
    });
}

fun source_file() -> Parser<SourceFile> {
    return many(
        choice(
            import_statement(),
            declaration(),
            statement()
        )
    ).followed_by(eof());
}
```

**Memory Analysis**:
- Combinator functions: ~**10KB** code
- Parser closures: ~**5KB** 
- Runtime stack: **2-4KB** (deep recursion)
- **Total: 17-19KB** ✅ Excellent!

## Part 4: Hybrid PEG-Style Approach

### Packrat Parser with Memoization

```minz
// Parsing Expression Grammar (PEG) style
struct PEGParser {
    input: *Token,
    position: u16,
    memo: [MemoEntry; 1000],  // Memoization table
}

struct MemoEntry {
    rule: u8,
    position: u16,
    result: ParseResult,
}

fun parse_with_memo(parser: *mut PEGParser, rule: u8) -> ParseResult {
    // Check memo table
    let key = (rule, parser.position);
    if let Some(entry) = find_memo(parser, key) {
        return entry.result;
    }
    
    // Parse and memoize
    let result = parse_rule(parser, rule);
    add_memo(parser, key, result);
    return result;
}

// Compact rule representation
const PEG_RULES: [str; 121] = [
    "sourceFile <- (import / declaration / statement)* EOF",
    "import <- 'import' path ('as' ID)? ';'",
    "declaration <- function / struct / enum / type / interface",
    // ... 118 more rules
];

fun compile_peg_rule(rule: str) -> CompiledRule {
    // Parse PEG syntax and compile to bytecode
    let bytecode: [u8; 100];
    let pc = 0;
    
    for element in parse_peg_elements(rule) {
        case element.type {
            PEG_LITERAL => {
                bytecode[pc++] = OP_MATCH_LITERAL;
                bytecode[pc++] = element.value;
            },
            PEG_RULE_REF => {
                bytecode[pc++] = OP_CALL_RULE;
                bytecode[pc++] = element.rule_id;
            },
            PEG_STAR => {
                bytecode[pc++] = OP_REPEAT_ZERO_OR_MORE;
            },
            PEG_CHOICE => {
                bytecode[pc++] = OP_CHOICE;
                bytecode[pc++] = element.choice_count;
            },
        }
    }
    
    return CompiledRule { bytecode: bytecode[0..pc] };
}
```

**Memory Analysis**:
- PEG bytecode: ~**8KB**
- Memoization table: **4KB**
- Parse tree: **10-20KB**
- **Total: 22-32KB** ✅ Fits!

## Part 5: Practical Implementation Strategy

### Phase 1: Minimal Bootstrap Parser (1-2 months)

```minz
// bootstrap_parser.minz - Just enough to parse MinZ
enum TokenType {
    // Only essential tokens
    IDENTIFIER, NUMBER, STRING,
    KEYWORD_FUN, KEYWORD_STRUCT, KEYWORD_IF,
    LPAREN, RPAREN, LBRACE, RBRACE,
    SEMICOLON, COMMA, DOT, ARROW,
}

struct SimpleParser {
    tokens: [Token; 1000],
    position: u16,
}

fun parse_minimal_minz(input: str) -> bool {
    let tokens = tokenize_basic(input);
    let parser = SimpleParser { tokens: tokens, position: 0 };
    
    while !at_end(&parser) {
        if !parse_top_level(&mut parser) {
            return false;
        }
    }
    
    return true;
}
```

### Phase 2: Parser Combinator Implementation (2-3 months)

1. Implement core combinators (2KB)
2. Build MinZ-specific parsers (5KB)
3. Add error recovery (2KB)
4. Optimize for Z80 (3KB)

### Phase 3: Grammar Translation (3-4 months)

1. Hand-translate 20% of rules (critical path)
2. Auto-generate simple rules
3. Optimize memory usage
4. Test on real hardware

## Part 6: Feasibility Assessment

### What's Feasible ✅

1. **Parser Combinators** (17-19KB)
   - Clean, functional approach
   - Reasonable memory usage
   - Good error messages
   - **Recommendation: Best approach**

2. **Table-Driven Parser** (38KB)
   - Fits in 48KB
   - Fast execution
   - Complex to maintain
   - **Viable but challenging**

3. **PEG/Packrat Parser** (22-32KB)
   - Good performance
   - Automatic memoization
   - Linear time guarantee
   - **Good alternative**

### What's NOT Feasible ❌

1. **Full ANTLR Runtime Port**
   - 200KB+ memory requirement
   - Complex DFA/ATN machinery
   - Multiple parse phases
   - **Impossible on Z80**

2. **Direct Grammar Translation**
   - 60-80KB memory needed
   - Deep recursion stack
   - Parse tree too large
   - **Exceeds 48KB limit**

## Part 7: Recommended Implementation

### The MinZ Parser Combinator Library

```minz
// Recommended approach for self-hosting
module minz.parsing;

// Core types
type Parser<T> = fn(*Input) -> Maybe<(T, *Input)>;
type Input = struct { data: *u8, pos: u16, len: u16 };

// Basic combinators (2KB)
pub fun pure<T>(value: T) -> Parser<T>;
pub fun fail<T>() -> Parser<T>;
pub fun satisfy(pred: fn(u8) -> bool) -> Parser<u8>;
pub fun char(c: u8) -> Parser<u8>;
pub fun string(s: str) -> Parser<str>;

// Combinators (3KB)
pub fun map<A,B>(p: Parser<A>, f: fn(A) -> B) -> Parser<B>;
pub fun bind<A,B>(p: Parser<A>, f: fn(A) -> Parser<B>) -> Parser<B>;
pub fun sequence<A,B>(p1: Parser<A>, p2: Parser<B>) -> Parser<(A,B)>;
pub fun choice<T>(p1: Parser<T>, p2: Parser<T>) -> Parser<T>;
pub fun many<T>(p: Parser<T>) -> Parser<[T]>;
pub fun many1<T>(p: Parser<T>) -> Parser<[T]>;
pub fun optional<T>(p: Parser<T>) -> Parser<Maybe<T>>;
pub fun between<A,B,C>(open: Parser<A>, p: Parser<B>, close: Parser<C>) -> Parser<B>;
pub fun sep_by<T,S>(p: Parser<T>, sep: Parser<S>) -> Parser<[T]>;

// MinZ-specific parsers (5KB)
pub fun identifier() -> Parser<str>;
pub fun number() -> Parser<u32>;
pub fun type_expr() -> Parser<Type>;
pub fun expression() -> Parser<Expr>;
pub fun statement() -> Parser<Stmt>;
pub fun declaration() -> Parser<Decl>;
pub fun source_file() -> Parser<[Decl]>;

// Error handling (2KB)
pub fun with_error<T>(p: Parser<T>, msg: str) -> Parser<T>;
pub fun recover<T>(p: Parser<T>, recovery: Parser<()>) -> Parser<Maybe<T>>;
```

### Memory Budget

| Component | Size | Running Total |
|-----------|------|---------------|
| Tokenizer | 3KB | 3KB |
| Combinator library | 5KB | 8KB |
| MinZ parsers | 7KB | 15KB |
| Symbol table | 4KB | 19KB |
| Parse result | 8KB | 27KB |
| Stack/heap | 5KB | 32KB |
| **Total** | **32KB** | ✅ **Fits in 48KB!** |

## Part 8: Implementation Roadmap

### Milestone 1: Proof of Concept (Month 1)
- [ ] Basic tokenizer in MinZ
- [ ] Core combinators (char, sequence, choice)
- [ ] Parse simple expressions
- [ ] Test on emulator

### Milestone 2: Core Parser (Month 2-3)
- [ ] Full combinator library
- [ ] Parse functions and structs
- [ ] Basic type checking
- [ ] Run on real hardware

### Milestone 3: Complete Grammar (Month 4-6)
- [ ] All 121 grammar rules
- [ ] Error recovery
- [ ] Optimize for size
- [ ] Bootstrap test

### Milestone 4: Self-Hosting (Month 7-8)
- [ ] Parser parses itself
- [ ] Generate MIR bytecode
- [ ] Complete bootstrap chain
- [ ] Documentation

## Part 9: Code Generation Strategy

Instead of building a full parse tree, generate code directly:

```minz
// Direct code generation during parsing
fun parse_function() -> Parser<()> {
    return sequence(
        keyword("fun"),
        identifier(),
        parameters(),
        return_type(),
        block()
    ).map(|(_, name, params, ret, body)| => () {
        // Generate code immediately
        emit_function_header(name);
        emit_parameters(params);
        emit_body(body);
        emit_function_footer();
        // Return unit, no AST needed!
    });
}
```

This saves 20-30KB of parse tree memory!

## Part 10: Comparison Table

| Approach | Memory | Complexity | Performance | Feasibility |
|----------|--------|------------|-------------|-------------|
| **Parser Combinators** | 17-19KB | Medium | Good | ✅ **Recommended** |
| **Table-Driven** | 38KB | High | Excellent | ✅ Viable |
| **PEG/Packrat** | 22-32KB | Medium | Very Good | ✅ Good |
| **Direct Translation** | 60-80KB | Low | Poor | ❌ Too Large |
| **Full ANTLR Port** | 200KB+ | Very High | Excellent | ❌ Impossible |

## Conclusion

**Yes, translating ANTLR grammar to MinZ is feasible!** But not through direct translation. The recommended approach is:

1. **Parser Combinator Library** - Most elegant, 17-19KB
2. **Direct Code Generation** - Skip AST, save 20KB
3. **Incremental Development** - Start with 20% of grammar
4. **Memory-First Design** - Every byte counts on Z80

The key insight: **Don't port ANTLR, replace it** with a MinZ-native solution that respects Z80 constraints.

### The Dream Lives On

```minz
// One day, on a ZX Spectrum...
fun main() -> void {
    let source = read_file("minzc.minz");
    let parser = create_minz_parser();
    let result = parse(parser, source);
    
    if result.success {
        print("MinZ compiler compiled itself!");
        print("Self-hosting achieved on 48KB!");
    }
}
```

With parser combinators and careful memory management, this dream is within reach. The journey from ANTLR to self-hosted MinZ parser is not just feasible - it's the natural evolution of the language.

---

*Next: Implementing the MinZ Parser Combinator Library - A practical guide to building parsers in 48KB of RAM*