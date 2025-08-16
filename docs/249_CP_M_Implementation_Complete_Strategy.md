# CP/M Implementation Strategy for MinZ: Complete Feasibility Study

*Date: 2025-08-16*  
*Status: Research & Strategy Document*

## Executive Summary

CP/M support in MinZ is not just feasible - it's a **strategic game-changer** that would make MinZ the premier modern language for vintage business computing. With our current 64% compilation success and improving MZA assembler, we're ready to target CP/M's massive software ecosystem.

## 🎯 Why CP/M Matters

### Historical Significance
- **First portable OS** - Ran on hundreds of different machines
- **Business standard** - WordStar, dBASE, Turbo Pascal all started here
- **Still active** - RunCPM, z88dk community, RC2014 hardware renaissance

### Strategic Value for MinZ
1. **Massive user base** - Thousands of active retrocomputing enthusiasts
2. **Real applications** - Not just games, but databases, word processors, compilers
3. **Professional credibility** - "MinZ can build CP/M software" = serious language
4. **Cross-platform** - One binary runs on 100+ different Z80 machines

## 🏗️ Technical Architecture

### CP/M Memory Map
```
$0000-$00FF: Zero page (RST vectors, BIOS workspace)
$0100-$xxFF: TPA (Transient Program Area) - Our code lives here!
$xx00-$FEFF: TPA continues (size varies by system)
$FF00-$FFFF: BIOS jump table
```

### MinZ CP/M Runtime Model
```minz
// Platform-specific module
module cpm {
    const TPA_START: u16 = 0x0100;
    const BDOS: u16 = 0x0005;
    const FCB1: u16 = 0x005C;
    const FCB2: u16 = 0x006C;
    const DMA: u16 = 0x0080;
    
    // BDOS function codes
    const C_READ: u8 = 1;      // Console input
    const C_WRITE: u8 = 2;     // Console output
    const F_OPEN: u8 = 15;     // Open file
    const F_READ: u8 = 20;     // Read sequential
    const F_WRITE: u8 = 21;    // Write sequential
}
```

## 🚀 Implementation Phases

### Phase 1: Basic COM Generation (1 week)
```minz
// hello_cpm.minz
import cpm;

fun putchar(c: u8) -> void {
    @asm {
        LD E, A      ; Character to E
        LD C, 2      ; BDOS function 2
        CALL 0005h   ; Call BDOS
    }
}

fun main() -> void {
    let msg = "Hello, CP/M!$";
    cpm.print_string(msg);
    cpm.exit(0);
}
```

**Deliverables:**
- ✅ COM file header generation
- ✅ ORG $0100 directive
- ✅ BDOS call wrappers
- ✅ String termination with '$'

### Phase 2: File I/O Support (2 weeks)
```minz
struct FCB {
    drive: u8,           // 0=default, 1=A:, 2=B:
    filename: [u8; 8],   // 8.3 format
    extension: [u8; 3],
    extent: u8,
    reserved: [u8; 2],
    rc: u8,              // Record count
    allocation: [u8; 16],
    cr: u8,              // Current record
    random: [u8; 3]
}

fun open_file(name: str) -> Result<File, Error> {
    let fcb = parse_filename(name)?;
    cpm.bdos_call(F_OPEN, &fcb)?;
    return Ok(File { fcb: fcb });
}
```

### Phase 3: Advanced Features (1 month)
```minz
// Database application in MinZ!
struct Record {
    id: u16,
    name: [u8; 30],
    balance: i32
}

class Database {
    file: File,
    index: BTree<u16, u32>,  // ID -> file offset
    
    fun find(id: u16) -> Result<Record, Error> {
        let offset = self.index.get(id)?;
        self.file.seek(offset)?;
        return self.file.read_record();
    }
}
```

## 🎮 Killer Apps We Could Build

### 1. MinZ Self-Hosted Compiler
```minz
// mzc.com - MinZ compiler running on CP/M!
fun compile(source: str) -> Result<Binary, Error> {
    let ast = parse(source)?;
    let mir = analyze(ast)?;
    let asm = codegen(mir)?;
    return assemble(asm);
}
```

### 2. Modern Database Engine
```minz
// B-tree indexed database with SQL-like queries
let db = Database::open("SALES.DB")?;
let results = db.query()
    .select("name", "total")
    .where("total", ">", 1000)
    .order_by("total", DESC)
    .limit(10)
    .execute()?;
```

### 3. Network Stack via Serial
```minz
// TCP/IP over serial port!
let conn = TcpConnection::dial("bbs.example.com:23")?;
conn.write(b"ATDT5551234\r\n")?;
let response = conn.read_until(b"CONNECT")?;
```

## 📊 Market Analysis

### Target Audiences
1. **RC2014 builders** - Hardware enthusiasts wanting modern software
2. **RunCPM users** - Emulator users on modern PCs
3. **Vintage collectors** - Kaypro, Osborne, Amstrad PCW owners
4. **Business retrocomputing** - People running vintage accounting software

### Competition Analysis
| Tool | Pros | Cons | MinZ Advantage |
|------|------|------|----------------|
| z88dk | Mature, many libs | C is painful on 8-bit | Modern syntax |
| Turbo Pascal | Classic, fast | Dead language | Active development |
| BASIC-80 | Easy to learn | Slow, limited | Compiled performance |
| Assembly | Maximum control | Hard to maintain | High-level with ASM speed |

## 🛠️ Technical Requirements

### Compiler Changes
```diff
+ CP/M target in codegen
+ COM file format output
+ $0100 ORG enforcement
+ BDOS/BIOS bindings
+ 8.3 filename support
```

### Standard Library
```minz
module cpm {
    // Console I/O
    fun getch() -> u8;
    fun putch(c: u8) -> void;
    fun puts(s: str) -> void;
    
    // File I/O
    fun open(name: str, mode: Mode) -> Result<File, Error>;
    fun read(f: File, buf: &[u8]) -> Result<usize, Error>;
    fun write(f: File, data: &[u8]) -> Result<usize, Error>;
    
    // System
    fun exec(cmd: str) -> Result<(), Error>;
    fun get_version() -> Version;
    fun reset() -> !;  // Never returns
}
```

## 💡 Revolutionary Features

### 1. Type-Safe FCB Handling
```minz
// No more manual FCB manipulation!
let file = File::create("DATA.TXT")?;
file.write_all(b"Hello, World!")?;
file.close()?;  // Automatic FCB cleanup
```

### 2. Memory-Mapped I/O Abstractions
```minz
@port(0x00)
struct ConsolePort {
    status: u8,
    data: u8
}

let console = ConsolePort::at(0x00);
while !console.status.ready() { }
console.data = 'A';
```

### 3. Overlay System
```minz
// Automatic overlay management for large programs
@overlay("MENU.OVL")
mod menu {
    fun show() -> Choice { ... }
}

@overlay("EDIT.OVL")  
mod editor {
    fun edit(doc: Document) -> void { ... }
}
```

## 🎯 Success Metrics

### Phase 1 (1 month)
- ✅ "Hello, World" COM file runs on RunCPM
- ✅ Basic file I/O working
- ✅ 10+ example programs

### Phase 2 (3 months)
- ✅ Self-hosted assembler
- ✅ Database demo application
- ✅ 100+ star GitHub repository

### Phase 3 (6 months)
- ✅ Full MinZ compiler on CP/M
- ✅ Commercial software ports
- ✅ "MinZ for CP/M" book deal

## 🚀 Call to Action

CP/M support would transform MinZ from "interesting Z80 experiment" to "the modern language for vintage computing." With our current momentum:

1. **Week 1**: Add CP/M target to compiler
2. **Week 2**: Create cpm module with BDOS bindings
3. **Week 3**: Build killer demo app
4. **Week 4**: Release and marketing blitz

The retrocomputing community is **hungry** for modern tools. MinZ can be the TypeScript of the 8-bit world - familiar, powerful, and pragmatic.

## 📚 Resources

### Essential References
- [CP/M 2.2 Manual](http://www.cpm.z80.de/manuals/cpm22-m.pdf)
- [RunCPM Emulator](https://github.com/MockbaTheBorg/RunCPM)
- [z88dk CP/M Support](https://github.com/z88dk/z88dk/wiki/Platform---CPM)

### Community Hubs
- [CPM Users Group](https://www.retroarchive.org/cpm/)
- [RC2014 Forums](https://groups.google.com/g/rc2014-z80)
- [Vintage Computer Federation](https://www.vcfed.org/)

---

*"MinZ for CP/M: Because your Kaypro II deserves modern software development"* 🖥️✨