# Jazyk Nanz: Kompletný sprievodca (v2)

> Nanz je typovaný systémový jazyk pre Z80 a retro ciele.
> Kompiluje cez HIR → MIR2 → Z80 assembly — čistý, moderný pipeline
> zdieľaný s frontendom PL/M-80.

**Verzia:** MinZ compiler v0.19.5 (MIR2 pipeline)
**Dátum:** 2026-03-10
**Stav:** Aktívny vývoj — jadro stabilné, niektoré okrajové časti nedoladené
**Zdroj pravdy:** `pkg/nanz/parse.go`, `pkg/hir/`, `pkg/mir2/`, `pkg/pipeline/`

---

## Obsah

1. [Čo je Nanz?](#chapter-1-what-is-nanz)
2. [Referencia syntaxe](#chapter-2-syntax-reference)
3. [Typový systém](#chapter-3-type-system)
4. [Kompilačný pipeline](#chapter-4-compilation-pipeline)
5. [MIR2 medzilehlá reprezentácia](#chapter-5-mir2-intermediate-representation)
6. [Optimalizačné prechody](#chapter-6-optimization-passes)
7. [Generovanie Z80 kódu](#chapter-7-z80-code-generation)
8. [Reťazce iterátorov a fúzia](#chapter-8-iterator-chains-and-fusion)
9. [Abstrakcie s nulovými nákladmi](#chapter-9-zero-cost-abstractions)
10. [QBE oracle správnosti](#chapter-10-qbe-correctness-oracle)
11. [PL/M-80: Seedovanie korpusu a idiomatický preklad](#chapter-11-plm-80-corpus-seeding-and-idiomatic-translation)
12. [Galéria skompilovaného výstupu](#chapter-12-compiled-output-gallery)
13. [Vzťah k MinZ a PL/M](#chapter-13-relation-to-minz-and-plm)
- [Príloha A: Kompletná gramatika syntaxe](#appendix-a-complete-syntax-grammar)
- [Príloha B: Triedy registrov a tabuľka nákladov](#appendix-b-register-classes-and-cost-table)
- [Príloha C: Referencia CLI](#appendix-c-cli-reference)
- [Príloha D: Inštalácia externých nástrojov](#appendix-d-installing-external-tools)
- [Príloha E: Známe chyby a obmedzenia](#appendix-e-known-bugs-and-limitations)

---

<a id="chapter-1-what-is-nanz"></a>

## Kapitola 1: Čo je Nanz?

Nanz (prípona `.nanz`) je aktívny frontendový jazyk kompilátora MinZ. Je staticky typovaný, imperatívny a navrhnutý pre dve cieľové skupiny:

1. Vývojári píšuci programy pre Z80 / retro ciele, ktorí chcú modernú syntax s abstrakciami s nulovými nákladmi.
2. Tím kompilátora MinZ, ktorý vyvíja a testuje backend MIR2.

### 1.1 Kompilačný pipeline

```
source.nanz
    │  nanz.Parse()
    ▼
*hir.Module          ← High-level IR (štruktúrovaný tok riadenia, pomenované premenné)
    │  hir.LowerModule()
    ▼
*mir2.Module         ← Mid-level IR (SSA-like, virtuálne registre, typované operácie)
    │  optimization passes (constant fold, DSE, BranchEquiv, CondRetSink, LUTGen)
    ▼
*mir2.Module         ← optimalizovaný
    │  OptimizeContracts() → PBQPAllocate()
    ▼
*mir2.Module         ← alokovaný (virtuálne → fyzické registre)
    │  Z80Codegen()
    ▼
output.a80           ← Z80 assembly kompatibilný s MZA
    │  mza (assembler)
    ▼
output.bin / .tap    ← binárka / ZX Spectrum tape image
```

Spustenie:

```bash
mz source.nanz -o output.a80
```

### 1.2 Nanz vs. MinZ

MinZ (`.minz`) je pôvodný frontend s vlastným parserom a generátorom kódu cieľujúcim na starší MIR1 IR. Tento pipeline je **zmrazený** — funguje, ale nie je ďalej vyvíjaný.

Nanz cieli na MIR2, ktorý poskytuje:

- SSA-like dataflow s parametrami blokov (štýl Cranelift/MLIR, nie phi uzly)
- PBQP alokátor registrov s váženými nákladovými vektormi
- LUT generovanie: vyhodnotenie v čase kompilácie pre čisté, rozsahovo ohraničené funkcie
- Interprocedurálna optimalizácia volacích konvencií
- Z80 emulátor používaný ako vyhodnocovač konštánt a dokazovač ekvivalencie vetiev

**Pravidlo:** Nové retro programy píšte v Nanz. Existujúce `.minz` programy nechajte tak.

### 1.3 Nanz vs. PL/M-80

PL/M-80 (`.plm`) je jazyk Intel z 1970-tych rokov pre CP/M. Kompilér MinZ obsahuje parser PL/M-80, ktorý kompiluje cez **rovnaký HIR → MIR2 → Z80 pipeline** ako Nanz:

```bash
mz legacy.plm -o legacy.a80     # rovnaký backend ako Nanz
mz legacy.plm --emit=nanz       # preklad PL/M do zdrojového kódu Nanz
```

### 1.4 Dizajnová filozofia

Nanz je zámerne minimalistický. Gramatika sa zmestí na jednu obrazovku. Žiadny garbage collector, žiadny runtime, žiadny dynamický dispatch. Každá abstrakcia — lambdy, iterátory, metódy štruktúr, rozhrania — sa kompiluje na priame Z80 inštrukcie bez réžie navyše oproti tomu, čo by ste napísali ručne.

---

<a id="chapter-2-syntax-reference"></a>

## Kapitola 2: Referencia syntaxe

Parser je ručne písaný rekurzívny zostupný parser s Prattovým parserom výrazov (`pkg/nanz/parse.go`). Zdrojový kód: ~2000 riadkov Go.

### 2.1 Štruktúra modulu

Zdrojový súbor Nanz je modul: postupnosť deklarácií najvyššej úrovne v ľubovoľnom poradí.

```nanz
// line comment
/* block comment */

struct Point { x: u8, y: u8 }
interface Drawable { draw }
global counter: u8
fun add(a: u8, b: u8) -> u8 { return a + b }
```

Neexistujú žiadne importy; linkovanie sa rieši na úrovni assemblera.

### 2.2 Typy

| Syntax | Šírka | Mapovanie na Z80 | Poznámky |
|--------|-------|-------------------|----------|
| `u8` | 8 bitov | A/B/C/D/E/H/L | unsigned bajt |
| `u16` | 16 bitov | HL/DE/BC | unsigned slovo |
| `i8` | 8 bitov | rovnaké registre, odlišná aritmetika | signed bajt |
| `i16` | 16 bitov | rovnaké | signed slovo |
| `bool` | 8 bitov | false=0, true=1 | |
| `void` | — | len návratový typ | |
| `ptr` | 16 bitov | HL/DE/BC | netypovaný ukazovateľ |
| `^T` | 16 bitov | HL/DE/BC | typovaný ukazovateľ na T |
| `[T; N]` | N×width(T) | — | pole pevnej veľkosti |
| `u8<lo..hi>` | 8 bitov | — | rozsahový typ (kandidát na LUT) |
| `u16<lo..hi>` | 16 bitov | — | rozsahový typ |
| `StructName` | súčet polí | odovzdaný ukazovateľom | typ štruktúry |
| `InterfaceName` | — | vyriešený v čase kompilácie | rozhranie ako typ parametra |

**Typy ukazovateľov:** `^T` zaznamenáva typ elementu pre rozlíšenie polí. Keď je T štruktúra, `^Struct` umožňuje typovaný prístup k poliam cez ukazovateľ (napr. `self.val` na prijímači `^Acc` automaticky dereferencuje a vyriešuje offsety polí). Toto NIE JE len syntaktický cukor — parser používa `varPtrElem[paramName]` na sledovanie štruktúry, na ktorú sa ukazuje, pre rozlíšenie polí a UFCS dispatch.

**Rozsahové typy:** `u8<0..255>` deklaruje rozsahový vstup. Rozsah je v zdrojovom kóde inkluzívny (`0..255`), interne uložený exkluzívne (`[0, 256)`). Funkcie s jedným rozsahovým parametrom a čistým telom sú kandidátmi na LUT generovanie (viď §6.4).

### 2.3 Funkcie

```nanz
fun name(param1: Type1, param2: Type2) -> ReturnType {
    // body
}
```

`fn` je akceptovaný ako alias pre `fun`. Funkcie bez návratovej hodnoty vynechávajú `-> ReturnType` (implicitne `void`).

```nanz
fn add(a: u8, b: u8) -> u8 { return a + b }

fun clear(buf: ^u8, n: u8) {
    var i: u8 = 0
    while i < n { buf[i] = 0; i = i + 1 }
}
```

### 2.4 Externé funkcie

Funkcie implementované mimo modul Nanz sa deklarujú pomocou `@extern`:

```nanz
@extern fun process(x: u8) -> void
@extern fun rom_print(s: ptr) -> void
```

Telo sa vynecháva. Kompilér priraďuje triedy registrov parametrom podľa štandardnej volacej konvencie.

**Stav:** Anotovaná forma `@extern("sym", params=[z80_a], returns=[z80_a])` opísaná v niektorej dokumentácii **zatiaľ nie je implementovaná** v parseri. V súčasnosti `@extern` funkcie dostávajú len automatické priradenie registrov.

### 2.5 Deklarácie premenných

**`var` — explicitný typ, voliteľný inicializátor:**

```nanz
var i: u8 = 0
var buf: ^u8
var port: u8 at(0xFE)          // memory-mapped at absolute address
```

**`let` — typ odvodený z inicializátora:**

```nanz
let x = 42                      // x: u8
let ptr = @ptr(u8, 0xFE)       // ptr: ptr
let y: u16 = 1000              // explicit type override
```

Klauzula `at(addr)` mapuje premennú na pevnú adresu v pamäti. Čítania a zápisy sa stávajú `LD (addr), r` / `LD r, (addr)`.

### 2.6 Globálne premenné

```nanz
global counter: u8
global vram: u8 at(0x4000)            // memory-mapped
global table: [u8; 4] = [1, 2, 4, 8]  // initialized array
global palette: Color                  // struct global
```

Globálne premenné sa emitujú v dátovej sekcii výstupu `.a80`. Globálne polia s inicializátormi dostávajú direktívy `DB`.

### 2.7 Literály

```nanz
42          // decimal — u8 if ≤255, u16 otherwise
0xFF        // hexadecimal
0           // zero (u8)
true        // bool
false       // bool
"hello"     // string → ptr to NUL-terminated bytes (interned)
```

### 2.8 Operátory

**Aritmetické** (ľavo-asociatívne):
`+` `-` `*` `/` `%`

**Bitové:**
`&` `|` `^` (XOR) `~` (NOT, unárny) `<<` `>>`

**Porovnávacie** (produkujú `bool`):
`==` `!=` `<` `<=` `>` `>=`

**Logické:**
`!` (NOT, unárny)

**Priorita** (od najvyššej po najnižšiu):

| Úroveň | Operátory |
|---------|-----------|
| 8 | `*` `/` `%` |
| 7 | `+` `-` |
| 6 | `<<` `>>` |
| 5 | `<` `<=` `>` `>=` |
| 4 | `==` `!=` |
| 3 | `&` |
| 2 | `^` (XOR) |
| 1 | `\|` |

Zátvorky prepíšu prioritu: `(a + b) * c`.

### 2.9 Operácie s ukazovateľmi

```nanz
let val = ptr^            // dereference (load byte at ptr)
^ptr = 42                 // store through pointer
let b = buf[3]            // index load (buf + 3)
buf[i] = 0               // index store
let p = &counter          // address-of global
let kb = @ptr(u8, 0xFE)  // typed constant pointer to absolute address
```

`@ptr(T, addr)` je idiomatický spôsob referencovania hardvérových registrov a ROM rutín.

### 2.10 Pretypovanie

```nanz
u8(expr)     // truncate to u8
u16(expr)    // zero-extend to u16
i8(expr)     // reinterpret as signed 8-bit
i16(expr)    // reinterpret as signed 16-bit
```

Pretypovanie je explicitné — žiadne implicitné rozširovanie. Na Z80 stojí `u8→u16` inštrukcie `LD H, 0 / LD L, A`; kompilér to neskryje.

### 2.11 Tok riadenia

```nanz
// if / else
if condition { /* then */ } else { /* else */ }

// while
while condition { /* body */ }

// for range (exclusive end)
for i in 0..n { /* i = 0, 1, ..., n-1 */ }

// for each (sequential memory scan)
for x: u8 in buf[0..n] { /* x loaded from buf[0]..buf[n-1] */ }

// break and continue
while true { if done { break } if skip { continue } }

// return
return           // void
return expr      // value

// switch
switch value {
    case 0: return 10
    case 1: return 20
    default: return 0
}
```

`for x: u8 in ptr[start..end]` sa kompiluje na DJNZ slučku s `HL` ako ukazovateľom a `B` ako počítadlom — najkompaktnejšia Z80 slučka.

Prípady switch nemajú prepad (fall-through). Každé telo prípadu sa ukončí pri ďalšom `case`, `default` alebo `}`.

### 2.12 Štruktúry

```nanz
struct Point { x: u8, y: u8 }
struct Vec3d { x: u16, y: u16, z: u8 }
```

Polia sú uložené sekvenčne, bez zarovnávania. Offsety sa počítajú v čase parsovania:
- `Point.x` → offset 0, `Point.y` → offset 1
- `Vec3d.y` → offset 2, `Vec3d.z` → offset 4

Hodnoty štruktúr sa vždy odovzdávajú **ukazovateľom** (HL na Z80). Prístup k poliam: `Load(ptr + offset)`.

### 2.13 Metódy štruktúr a UFCS

Metódy sa deklarujú syntaxou `TypeName.methodName`:

```nanz
struct Acc { val: u8 }

fun Acc.add(self: ^Acc, amount: u8) -> u8 {
    self.val = self.val + amount
    return self.val
}
```

Kompilér to uloží ako `Acc_add`. Na miestach volania UFCS prepíše:

```nanz
acc_g.add(5)    // → Acc_add(&acc_g, 5) — direct CALL, no vtable
```

**Ako funguje rozlíšenie UFCS** (v čase parsovania):
1. Parser vidí `base.method(args)`.
2. Vyhľadá `exprTy(base)` — ak je to štruktúra, skontroluje `methodTable[structName][method]`.
3. Ak je base ukazovateľ `^Struct`, skontroluje `varPtrElem[name]` pre typ štruktúry.
4. Ak je base premenná s typom rozhrania, zavolá `findImplementors()`.
5. Prepíše na `CallExpr{Fn: "StructName_method", Args: [base, args...]}`.

**Prijímač ukazovateľom** (`self: ^Acc`):
- `self.val` automaticky dereferencuje — vyriešuje offset poľa cez `varPtrElem`.
- `self^.val` tiež funguje (explicitná dereferencia, rovnaký výsledok).
- Ukazovateľ cestuje v HL (ClassPointer). Čítanie polí: `LD reg, (HL)` na offsete 0, `INC HL; LD reg, (HL)` na offsete 1, atď.

**Prijímač hodnotou** (`self: Acc`):
- Štruktúra sa odovzdáva ukazovateľom — na úrovni ABI rovnaké ako `^Acc`.
- Parser zaznamenáva v `methodTable` s plnou informáciou o návratovom type.

### 2.14 Preťažovanie operátorov

```nanz
fun +(a: Vec2, b: Vec2) -> Vec2 { return a }
fun -(a: Vec2, b: Vec2) -> Vec2 { return a }

fun compute(a: Vec2, b: Vec2) -> Vec2 {
    return a + b    // dispatched to op_add(a, b) because a is Vec2
}
```

Preťažiteľné operátory: `+` `-` `*` `/` `%` `==` `!=` `<` `<=` `>` `>=` `&` `|` `^`.

Manglované názvy: `op_add`, `op_sub`, `op_mul`, `op_div`, `op_rem`, `op_eq`, `op_ne`, `op_lt`, `op_le`, `op_gt`, `op_ge`, `op_and`, `op_or`, `op_xor`.

**Dôležité:** Pre primitívne typy (`u8 + u8`) sa vždy používa vstavaný `BinExpr`, bez ohľadu na preťaženia v dosahu. Preťaženia sa uplatňujú len keď je ľavý operand typu štruktúry.

### 2.15 Rozhrania

```nanz
interface Animal {
    speak
    move
}
```

Rozhrania sú **kontrakty v čase kompilácie**. Žiadna vtable, žiadny fat pointer, žiadna tabuľka dispatchovania metód.

**Ako typ parametra:**

```nanz
fun feed(a: Animal) -> u8 {
    return a.speak()      // monomorphized at compile time
}
```

**Pravidlá rozlíšenia:**
- **Jeden implementátor** → monomorfizácia: `a.speak()` → `Dog_speak(a)`.
- **Viacero implementátorov** → chyba kompilácie: `"ambiguous dispatch: ... multiple types implement Animal: [Dog Cat]; use concrete type"`.
- **Žiadny implementátor** → chyba kompilácie.

**Rozhranie ako typ globálnej premennej:**

```nanz
global g_thing: Drawable
```

Funguje rovnako: UFCS dispatch rozlíši na jediného implementátora.

### 2.16 Lambdy

```nanz
let double = |x: u8| x * 2          // expression body
let add = |a: u8, b: u8| a + b      // multi-param
let greet = |x: u8| { return x + 1 }  // block body
let inc = |x| x + 1                 // type defaults to u8
```

Každá lambda sa stáva funkciou najvyššej úrovne `lambda_N` (sekvenčné počítadlo). Referencia na mieste volania je `VarRefExpr{Name: "lambda_N"}`.

**Pravidlá zachytávania:**
- **Lambdy fúzovaných iterátorov** (forEach/map/filter): môžu zachytávať a meniť vonkajšie lokálne premenné. Kompilér deteguje voľné premenné cez `hasFreeVars()`, preskočí samostatné zníženie a prevádza zachytené premenné ako parametre blokov cez DJNZ slučku. Nulová halda, nulový spill.
- **Nefúzované lambdy** (ukazovatele na funkcie): nemôžu zachytávať vonkajšie lokálne premenné — žiadny runtime rámec. Prístupné sú len globálne premenné.

### 2.17 Iterátorové metódy (UFCS na ukazovateľoch)

```nanz
buf.forEach(|x: u8| { process(x) }, n)       // execute for each element
buf.map(|x: u8| (x * 2))                     // transform elements
buf.filter(|x: u8| (x > 5))                  // keep matching elements
buf.mapInPlace(|x: u8| (x + 2), n)           // in-place transform
```

Tieto sú rozpoznávané parserom ako UFCS volania metód na ukazovateľových výrazoch. `tryLowerIterChain` v HIR lowereri fúzuje reťazce do jednej DJNZ slučky. Viď Kapitola 8.

---

<a id="chapter-3-type-system"></a>

## Kapitola 3: Typový systém

### 3.1 Typy MIR2

MIR2 IR (`pkg/mir2/types.go`) podporuje:

| Typ | Šírka | Go reprezentácia |
|-----|-------|------------------|
| `TyVoid` | 0 | `&IntTy{Bits: 0}` |
| `TyBool` | 8 | `&IntTy{Bits: 8}` |
| `TyU8` | 8 | `&IntTy{Bits: 8, Signed: false}` |
| `TyI8` | 8 | `&IntTy{Bits: 8, Signed: true}` |
| `TyU16` | 16 | `&IntTy{Bits: 16, Signed: false}` |
| `TyI16` | 16 | `&IntTy{Bits: 16, Signed: true}` |
| `TyU24` | 24 | `&IntTy{Bits: 24}` (eZ80 budúcnosť) |
| `TyPtr` | 16 | `&PtrTy{}` |
| `StructTy` | súčet polí | `&StructTy{Name, Fields}` |
| `ArrayTy` | N×elem | `&ArrayTy{Len, Elem, Layout}` |
| `TupleTy` | súčet elemov | `&TupleTy{Elems}` (multi-return) |

Okrem toho rozsahové typy obaľujú základný typ s hranicami `[Lo, Hi)`:
```go
type RangedTy struct {
    Base Ty
    Lo, Hi int  // [Lo, Hi) — exclusive upper bound
}
```

### 3.2 Priradenie tried registrov

Parametrom sa priraďujú triedy registrov na základe pozície a typu (`classForParam` v `hir/lower.go`):

| Pozícia | Typ | Trieda | Z80 fyzický |
|---------|-----|--------|-------------|
| 1. | `u8`, `bool` | `ClassAcc` | A |
| 1. | `u16`, `ptr`, struct | `ClassPointer` | HL |
| 2. | `u8` | `ClassGeneral` | C |
| 2. | `u16` | `ClassIndex` | DE |
| 3. | `u8` | `ClassCounter` | B |
| 3.+ | `u16` | `ClassPair` | BC/DE |

Návratové hodnoty: `u8` → `ClassAcc` (A), `u16`/`ptr` → `ClassPointer` (HL).

### 3.3 Rozloženie štruktúry

Polia sú zabalené sekvenčne, bez zarovnávacích medzier:

```nanz
struct Color { r: u8, g: u8, b: u8 }  // total: 3 bytes
// r at offset 0, g at offset 1, b at offset 2
```

Parser počíta offsety v čase parsovania sčítaním `Ty.Width() / 8` pre každé predchádzajúce pole. Polia globálnych štruktúr dostávajú EQU štítky v assembly: `palette__r EQU palette + 0`.

---

<a id="chapter-4-compilation-pipeline"></a>

## Kapitola 4: Kompilačný pipeline

Celý pipeline je orchestrovaný súborom `pkg/pipeline/pipeline.go`.

### 4.1 Fázy pipeline

```go
func CompileHIRSteps(hm *hir.Module) (Steps, error)
```

**Fáza 1 — Zníženie HIR → MIR2** (`hir.LowerModule`):
- Pomenované premenné → virtuálne registre (nový register pri každom priradení)
- Štruktúrovaný tok riadenia → základné bloky s parametrami blokov
- Mutácie v slučkách → parametre blokov v hlavičkách slučiek (detekované cez `scanMutations`)
- ForEachStmt → slučka pointer+counter priateľská k DJNZ

**Fáza 2 — Optimalizácie na úrovni funkcií** (v poradí):
1. `EliminateDeadBlocks` — odstránenie nedosiahnuteľných blokov
2. `ReorderBlocks` — zlepšenie fall-through
3. **Konštantný pipeline** (iterovaný do fixpointu):
   - `PropagateConstants` — sledovanie konštánt cez presuny
   - `FoldConstants` — vyhodnotenie čistých operácií v čase kompilácie
   - `SimplifyIdentities` — `PtrAdd(x, 0)` → `Move(x)`, atď.
   - `ConstantCallElim` — zloženie volaní s konštantnými argumentmi cez VM
4. `DeadStoreElim` — odstránenie čistých inštrukcií s nepoužitými výsledkami (iterované)
5. `BranchEquiv` — eliminácia vetiev založená na VM (dokazuje redundantné stráže)
6. `CondRetSink` — vytiahnutie triviálnych else-blokov do podmienených návratov

**Fáza 3 — Na úrovni modulu: LUTGen**:
- Čisté funkcie s rozsahovými parametrami → vyhľadávacie tabuľky v čase kompilácie

**Fáza 4 — Verifikácia** (`Verify`):
- Unikátne štítky blokov, platné terminátory, konzistencia typov

**Fáza 5 — Interprocedurálna optimalizácia**:
- `OptimizeContracts` — greedy DP na grafe volaní pre priradenie tried registrov
- `ApplyContracts` — prepísanie signatúr funkcií

**Fáza 6 — Alokácia registrov**:
- `ComputeLiveness` — spätný dataflow do fixpointu
- `PBQPAllocate` — vážené priradenie nákladov virtuálnych registrov na fyzické Z80

**Fáza 7 — Generovanie kódu**:
- `Z80Codegen` → text assembly

### 4.2 Formáty výstupu

```bash
mz source.nanz --emit=hir        # HIR dump
mz source.nanz --emit=mir2-raw   # MIR2 before optimization
mz source.nanz --emit=mir2       # MIR2 after optimization
mz source.plm  --emit=nanz       # PL/M → Nanz source translation
mz source.nanz -o output.a80     # Z80 assembly (default)
```

---

<a id="chapter-5-mir2-intermediate-representation"></a>

## Kapitola 5: MIR2 medzilehlá reprezentácia

MIR2 je jadrový IR. Používa **argumenty blokov** (štýl Cranelift/MLIR) namiesto phi uzlov.

### 5.1 Štruktúra modulu

```
Module
├── Funcs[] — každý s Blocks[]
├── Globals[] — dáta na úrovni modulu
├── Strings[] — internovaný pool reťazcov
└── Structs[] — definície typov štruktúr
```

### 5.2 Inštrukcie

Každá inštrukcia: `%dst = Op %src0, %src1 : Ty [Class]`

**Aritmetické:** `OpAdd`, `OpSub`, `OpMul`, `OpDiv`, `OpSDiv`, `OpMod`
**Bitové:** `OpAnd`, `OpOr`, `OpXor`, `OpShl`, `OpShr`, `OpSar`
**Unárne:** `OpNeg`, `OpNot`
**Konverzné:** `OpExt` (zero-extend), `OpSext` (sign-extend), `OpTrunc`
**Porovnávacie:** `OpCmp` s podmienkami: `CmpEq`, `CmpNe`, `CmpLt`, `CmpLe`, `CmpGt`, `CmpGe`, `CmpUlt`, `CmpUle`, `CmpUgt`, `CmpUge`, `CmpSubCarry`
**Presun dát:** `OpConst` (immediate), `OpMove` (kópia registra)
**Pamäť:** `OpLoad`, `OpStore`, `OpAddrOf`, `OpAlloca`, `OpField` (ptr+offset), `OpPtrAdd` (ptr+runtime offset), `OpPtrBump` (ptr+compile-time stride)
**Volania:** `OpCall` (priame), `OpCallIndirect` (cez ukazovateľ)
**SMC:** `OpPatchSlot`, `OpLoadPatched`, `OpPatch` (primitívy samo-modifikujúceho kódu)

### 5.3 Parametre blokov

```
block @loop_head(%ptr: u16 [pointer], %cnt: u8 [counter]):
    %cond = cmp.gt %cnt, %limit : bool [flag]
    br_if %cond, @exit(), @body(%ptr, %cnt)
```

Argumenty blokov definujú registre na vstupe. Argumenty sa odovzdávajú na každej hrane (jump/branch). Toto nahrádza phi uzly a robí paralelné kópie explicitnými pre každú hranu.

### 5.4 Terminátory

| Terminátor | Sémantika | Z80 emisia |
|------------|-----------|------------|
| `TermJmp(target, args)` | Nepodmienený skok | `JP label` |
| `TermBrIf(cond, then, else)` | Podmienená vetva | `JP Z/NZ/C/NC label` |
| `TermDJNZ(counter, body, exit)` | Dekrementuj B, skoč ak nenulové | `DJNZ rel8` |
| `TermCondRet(cond, vals, then)` | Podmienený návrat | `RET CC` |
| `TermRet(vals)` | Návrat | `RET` |

`TermCondRet` je produkovaný optimalizačným prechodom CondRetSink — umožňuje Z80 jednoinštrukčný podmienený návrat.

---

<a id="chapter-6-optimization-passes"></a>

## Kapitola 6: Optimalizačné prechody

### 6.1 Konštantný pipeline (fixpoint)

Štyri prechody iterujú kým sa nič nezmení:

1. **PropagateConstants** — ak `%r = const 42`, nahradí všetky použitia `%r` hodnotou 42.
2. **FoldConstants** — `const(3) + const(5)` → `const(8)`.
3. **SimplifyIdentities** — `PtrAdd(x, Const(0))` → `Move(x)`. Eliminuje redundantnú nulovú offsetovú aritmetiku ukazovateľov (kritické pre prijímače `^Struct`).
4. **ConstantCallElim** — čistá funkcia so všetkými konštantnými argumentmi → vyhodnotenie cez MIR2 VM.

### 6.2 Eliminácia mŕtvych zápisov

Odstraňuje čisté inštrukcie, ktorých výsledky sa nikdy nepoužijú. Iterované do fixpointu, pretože odstránenie jednej mŕtvej inštrukcie môže spraviť jej zdroje mŕtvymi.

**Nikdy neodstránené:** `OpStore`, `OpCall`, `OpCallIndirect`, `OpAsm`, `OpPatch` (vedľajšie efekty).

### 6.3 BranchEquiv (eliminácia vetiev na základe VM)

Dokazuje redundanciu podmienených vetiev vyčerpávajúcim testovaním na VM.

**Príklad:** `abs_diff` so strážou `if a == b { return 0 }`. BranchEquiv spustí všetkých 256 vstupov `(v, v)` cez pôvodnú aj opravenú funkciu. Obe vrátia 0 → vetva je dokázateľne redundantná → nahradí `BrIf(eq, @zero, @diff)` výrazom `Jmp(@diff)`.

**Spoľahlivé pre u8:** vyčerpávajúci 256-hodnotový test. Pre širšie typy: heuristické vzorkovanie (bezpečné na rozšírenie neskôr).

### 6.4 CondRetSink

Hľadá `BrIf(cond, @then, @else)` kde `@else` je triviálny (jeden predchodca, čisté inštrukcie, `TermRet`). Vytiahne inštrukcie `@else` do aktuálneho bloku a nahradí `BrIf` výrazom `TermCondRet`.

**Fúzované optimalizácie spustené ihneď po vytiahnutí:**
- **SubSwapNeg:** Ak vytiahnuté `sub(a, b)` má obrátené `sub(b, a)` v then-bloku → nahradí `neg(result)`. Ušetrí `LD A,r; SUB r2` → jedno `NEG`.
- **HoistReorderSubBeforeCmp + CmpSubCarry:** Preusporiadanie `sub` pred `cmp_lt` na rovnakých operandoch → carry flag zo `SUB` JE výsledok porovnania. Eliminuje inštrukciu `CP` úplne.

### 6.5 LUTGen (na úrovni modulu)

Čisté funkcie s jedným rozsahovým parametrom → vyhľadávacia tabuľka.

**Oprávnenosť:** 1 parameter typu `u8<lo..hi>` alebo `u16<lo..hi>`, rozsah ≤ 256, jeden návrat, žiadne externé volania, žiadny zápis do globálnych premenných.

**Proces:**
1. VM-vyhodnotenie funkcie pre každý vstup v rozsahu
2. Emisia stránkovo zarovnanej `DB` tabuľky ako globálu
3. Nahradenie tela funkcie vyhľadávaním v tabuľke:
   ```asm
   LD H, lut^H    ; 7T — page base (high byte only)
   LD L, C         ; 4T — index
   LD A, (HL)      ; 7T — lookup
   RET
   ```

**Výsledok:** 18 T-taktov bez ohľadu na pôvodnú zložitosť funkcie. 8-iteračná slučka popcount sa stáva 3 inštrukciami.

### 6.6 PBQP alokátor registrov

Nahrádza starý greedy alokátor. PBQP (Partitioned Boolean Quadratic Program) minimalizuje:

```
Σ nodeCost[r][loc(r)] + Σ edgeCost[interfering pairs]
```

**Náklady uzla:** `useCount[r] × costTable.Cost(r.Class, location)`. Horúce registre platia viac za drahé umiestnenia.

**Redukčné pravidlá:**
- **R0:** stupeň 0 (izolovaný) → priradiť najlacnejšie umiestnenie okamžite.
- **R1:** stupeň 1 (list) → zložiť náklady hrany do suseda, odložiť.
- **RN:** stupeň ≥ 2 → greedy podľa **delta** (`2. najlepšia − najlepšia`). Registre s veľkou deltou (vysoký postih pri vytlačení) sa alokujú prvé.

**Výsledok pre 4 simultánne ClassPointer registre:**
```
p0 → HL (cost 0)
p1 → DE (cost 4)
p2 → BC (cost 6)
p3 → IX (cost 8)  — no $F0xx memory spill
```

### 6.7 Koalescencia kópií po alokácii

Po priradení fyzických umiestnení PBQP `coalesceAllocResult` eliminuje redundantné kópie na hraniciach blokov:

- Zhromaždí afinentné hrany z `OpMove` a párov parameter_bloku↔argument.
- Jednopriechodové prefarbenie: ak žiadny sused v interference grafe nepoužíva umiestnenie cieľa, prefarbí na zhodu. Zámok `recolored` zabraňuje rotačným cyklom vo phi-weboch slučiek.

### 6.8 Interprocedurálna optimalizácia kontraktov

`OptimizeContracts` vykonáva greedy DP na grafe volaní:

1. Topologické triedenie (listy prvé).
2. Pre každú funkciu: enumerácia kandidátskych vektorov tried registrov.
3. Náklady = interné náklady adaptéra + náklady hrán cez všetkých volajúcich.
4. Výber priradenia s minimálnymi nákladmi.

Toto vyberá volacie konvencie pre jednotlivé funkcie globálne, čím sa znižujú presuny registrov na miestach volaní.

---

<a id="chapter-7-z80-code-generation"></a>

## Kapitola 7: Generovanie Z80 kódu

`pkg/mir2/z80codegen.go` — konvertuje alokovaný MIR2 na text assembly.

### 7.1 Emisia inštrukcií

| MIR2 Op | Z80 výstup |
|---------|-----------|
| `OpConst` u8 | `LD r, imm8` |
| `OpConst` u16 | `LD rr, imm16` |
| `OpAdd` u8 | `ADD A, r` |
| `OpAdd` u16 | `ADD HL, rr` |
| `OpSub` u8 | `SUB r` |
| `OpSub` u16 | `AND A; SBC HL, rr` |
| `OpNeg` | `NEG` (len 8-bitový, register A) |
| `OpCmp` | `CP r` (výsledok vo flagoch, ClassFlag) |
| `OpLoad` | `LD A, (HL)` / `LD A, (rr)` |
| `OpStore` | `LD (HL), r` |
| `OpCall` | `CALL label` |

### 7.2 Adresovanie IX/IY

Keď PBQP priradí ukazovateľ do IX, codegen používa adresovanie s posunom:

```asm
LD A, (IX+0)     ; 8-bit load, offset 0
LD (IX+1), A     ; 8-bit store, offset 1
LD L, (IX+0)     ; 16-bit load (lo)
LD H, (IX+1)     ; 16-bit load (hi)
```

**Kópia 16-bitového registra↔IX:** DE/BC→IX používa nedokumentovanú bajtovú kópiu:
```asm
LD IXH, D    ; DD 62 — 8T (D not substituted by DD prefix)
LD IXL, E    ; DD 6B — 8T (total 16T)
```

**Bajtová kópia HL→IX je NEPLATNÁ:** DD prefix nahrádza H→IXH a L→IXL v pozíciách zdrojového aj cieľového operandu, takže `LD IXH, H` sa dekóduje ako `LD IXH, IXH` (NOP). Namiesto toho použite `PUSH HL; POP IX` (21T).

### 7.3 Peephole optimalizácie v codegen

**Potlačenie mŕtvych konštánt:** `OpConst`, ktorého jediné použitie je `OpCmp` → emitovať `CP imm8` priamo, preskočiť `LD r, imm`.

**Propagácia kópií:** Sledovanie `holdsPhys[A] = D` — ak A už obsahuje hodnotu D, preskočiť `LD A, D`.

**Sledovanie posledných flagov:** Ak sú flagy už nastavené predchádzajúcou inštrukciou s rovnakými operandmi, potlačí redundantné `CP`.

**Stránkovo zarovnaný LUT:** Predsken blokov pre vzory prístupu k LUT → `LD H, sym^H; LD L, idx; LD A, (HL)` (18T).

**Priame adresovanie polí globálnych štruktúr:** Predsken pre `AddrOf(global) + Field(offset) + Load/Store` → `LD A, (sym__field)` priamo (13T).

**Detekcia HL-reťazca:** Viaceré po sebe idúce zápisy polí do tej istej štruktúry → jedno `LD HL, sym` + `LD (HL), r; INC HL` reťazec (ušetrí opätovné načítanie HL).

---

<a id="chapter-8-iterator-chains-and-fusion"></a>

## Kapitola 8: Reťazce iterátorov a fúzia

### 8.1 Štyri iterátorové metódy

| Metóda | Signatúra | Sémantika |
|--------|-----------|-----------|
| `forEach(lambda, n)` | Vykonaj lambdu pre n elementov | Terminálna |
| `map(lambda)` | Transformuj každý element | Medziľahlá |
| `filter(lambda)` | Ponechaj elementy kde lambda vráti true | Medziľahlá |
| `mapInPlace(lambda, n)` | Transformácia na mieste | Terminálna |

Tieto sú rozpoznávané ako UFCS na ukazovateľových výrazoch. `tryLowerIterChain` v HIR lowereri fúzuje reťazce.

### 8.2 Fúzia

```nanz
buf.map(|x| x * 2).filter(|x| x > 5).forEach(|x| { process(x) }, n)
```

Fúzovaný do jedného `ForEachStmt`:

```
for x in buf[0..n]:
    let mapped = x * 2         // map body inlined
    if mapped > 5 {            // filter: skip if false
        process(mapped)        // forEach body inlined
    }
```

Toto sa stáva jednou DJNZ slučkou:
```asm
.loop:
    LD A, (HL)     ; load element
    INC HL
    ADD A, A       ; map: x * 2
    CP 6           ; filter: x > 5 → x >= 6
    JR C, .skip
    CALL process   ; forEach body
.skip:
    DJNZ .loop
```

Žiadne medziľahlé pole. Žiadne volanie lambdy CALL. Tri fázy, jedna slučka.

### 8.3 Zachytávanie uzáverov vo fúzovaných reťazcoch

```nanz
fun sum(buf: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    buf.forEach(|x: u8| { s = s + x }, n)
    return s
}
```

`s` je voľná premenná v lambde. Kompilér:
1. Deteguje voľnú referenciu cez `hasFreeVars()`.
2. Preskočí samostatné zníženie `lambda_N`.
3. Prevedie `s` ako parameter bloku cez DJNZ slučku.

Výsledok: `s` žije v registri (napr. C) po celý čas — nulový spill, nulová halda.

### 8.4 Náklady na element (sum_chain)

```asm
LD D, (HL)     ; 7T — load element
LD A, C        ; 4T
ADD A, D       ; 4T — s + x
LD C, A        ; 4T
INC HL         ; 6T
DJNZ .loop     ; 13T (taken)
; Total: 38T per element
```

Pre porovnanie: samostatné volanie funkcie na element pridáva CALL(17T) + RET(10T) = 27T réžie pred akoukoľvek prácou.

### 8.5 mapInPlace spätný zápis

```nanz
buf.mapInPlace(|x: u8| (x + 2), n)
```

Flag `MutateInPlace` spúšťa spätný zápis po tele lambdy:

```asm
LD A, (HL)     ; load
ADD A, 2       ; transform
LD (HL), A     ; write back
INC HL
DJNZ .loop
```

### 8.6 Známe nefunkčné iterátory

- **enumerate:** Konflikt registra B — B je zároveň DJNZ počítadlo aj enumeračný index.
- **reduce:** Register A prepísaný medzi dvoma SMC parametrami.

---

<a id="chapter-9-zero-cost-abstractions"></a>

## Kapitola 9: Abstrakcie s nulovými nákladmi

Každá abstrakcia v Nanz sa kompiluje na priame Z80 inštrukcie. Tu je dôkaz.

### 9.1 UFCS — nulové náklady

`obj.method(args)` sa odcukorí na `method(obj, args)` v čase parsovania. Tabuľka metód je `map[string]map[string]methodInfo` konzultovaná len počas parsovania.

**Náklady: nula.** `CALL Acc_add` je identické s ručne napísaným volaním.

### 9.2 Rozhrania — nulové náklady

```nanz
interface Animal { speak }
struct Dog {}
fun Dog.speak(self: Dog) -> u8 { return 1 }
```

Žiadna vtable, žiadny fat pointer. Pri `g_dog.speak()` kompilér rozlíši Dog v čase parsovania a emituje `CALL Dog_speak`.

**Náklady: nula.** 17T pre priame `CALL` oproti ~55T pre Go-štýl interface dispatch.

**Obmedzenie:** Konkrétny typ musí byť staticky známy. Skutočný dynamický dispatch nie je implementovaný (a porušil by princíp nulovej réžie).

### 9.3 Lambdy — nulové náklady

Každý `|x| expr` sa stáva `lambda_N` — bežnou funkciou. Pri použití v reťazcoch iterátorov sa telo inlinuje. Pri použití ako ukazovateľ na funkciu je to štandardné `CALL`.

**Náklady: nulová alokácia, nulová štruktúra uzáveru.** Réžia CALL len keď sa neinlinuje.

### 9.4 Metódy štruktúr — nulové náklady

`fun Vec2.add(self: Vec2, ...)` sa uloží ako `Vec2_add`. Manglovanie názvov je jedinou „réžiou" (v čase kompilácie, nie runtime). Vygenerovaný assembly je nerozoznateľný od voľnej funkcie.

### 9.5 Preťažovanie operátorov — nulové náklady

`a + b` na typoch štruktúr dispatchuje na `op_add(a, b)` — bežné volanie funkcie. Žiadna runtime kontrola typov.

---

<a id="chapter-10-qbe-correctness-oracle"></a>

## Kapitola 10: QBE oracle správnosti

### 10.1 Čo je QBE

QBE (https://c9x.me/compile/) je malý, rýchly backendový kompilér, ktorý konvertuje QBE IL (jednoduchý SSA formát) na natívny x86-64 alebo arm64 assembly. NIE JE cieľom Nanz — je to **testovací nástroj**.

### 10.2 Pipeline oracle správnosti

```
source.nanz
    ├─→ HIR → MIR2 → Z80Codegen → MZE emulator → Result A
    └─→ HIR → MIR2 → QBE IL → qbe → cc → native binary → Result B

If A ≠ B: Bug is in Z80 codegen (MIR2 semantics proven correct)
If A = B: Both pipelines agree — correctness confirmed
```

QBE pipeline sa zastaví PRED Z80-špecifickými krokmi (optimalizácia kontraktov, alokácia registrov). QBE robí vlastnú alokáciu registrov.

### 10.3 E2E testy

`pkg/mir2qbe/e2e_test.go` obsahuje 7 E2E testov:

| Test | Čo overuje |
|------|-----------|
| `TestE2E_PLM_AbsDiff` | PL/M → HIR → MIR2 → QBE → native |
| `TestE2E_PLM_Fib` | Fibonacci so slučkou |
| `TestE2E_Nanz_SumArray` | Ukazovateľová aritmetika, `ptr[i]` slučka |
| `TestE2E_Nanz_AbsDiff` | Tok riadenia (if/else) |
| `TestE2E_Nanz_StructFields` | Prístup k poliam globálnej štruktúry |
| `TestE2E_Nanz_UFCS` | Dispatch metód na globáloch |
| `TestE2E_Nanz_Interface_ZeroCost` | Monomorfizácia interface dispatchu |

Testy sa automaticky preskočia ak `qbe` nie je v PATH (`exec.LookPath("qbe")`).

### 10.4 Preklad MIR2 → QBE

Kľúčové mapovania (`pkg/mir2qbe/codegen.go`):
- Všetky celočíselné typy (u8, u16, i8, i16, bool) → QBE `w` (32-bitové slovo)
- `ptr` → QBE `l` (64-bitový long, natívny ukazovateľ)
- Parametre blokov → phi uzly (QBE používa SSA s phi, nie argumenty blokov)
- Z80-špecifické operácie (SMC, push/pop, inline asm) → preskočené

### 10.5 Inštalácia QBE

Viď [Príloha D](#appendix-d-installing-external-tools).

---

<a id="chapter-11-plm-80-corpus-seeding-and-idiomatic-translation"></a>

## Kapitola 11: PL/M-80: Seedovanie korpusu a idiomatický preklad

### 11.1 Úloha PL/M pri vytváraní ekosystému Nanz

PL/M-80 slúžil ako **bootstrap korpus** pre backend MIR2. 26 súborov Intel 80 Tools (ALGOL-M, BASIC-80, ML80 macro assembler atď.) — všetko reálny produkčný kód zo 1970-tych rokov — poskytlo 1 338 funkcií a 11 661 príkazov na testovanie pipeline HIR→MIR2→Z80 ešte pred tým, ako Nanz vôbec existoval.

Pracovný postup:

```
Step 1: Parse PL/M corpus (26/26 files, 100% coverage)
Step 2: Lower to HIR → verify correctness
Step 3: Lower to MIR2 → verify optimizations
Step 4: Emit Z80 → compare with Intel's PL/M-80 V4.0 output
Step 5: --emit=nanz → generate Nanz source from HIR
Step 6: Write idiomatic Nanz by hand, guided by the mechanical translation
```

To znamená, že každý HIR uzol, každá MIR2 optimalizácia a každý vzor codegen Z80 bol najprv validovaný na **reálnom PL/M kóde** pred tým, ako ho použili programy Nanz.

### 11.2 Mechanický preklad: `mz program.plm --emit=nanz`

Flag `--emit=nanz` spustí `plm.Compile()` → HIR → `nanz.Print()`, čo produkuje syntakticky platný zdrojový kód Nanz. Preklad je štrukturálny — presne zachováva logiku PL/M programu.

**PL/M zdrojový kód:**
```plm
SUM_ARRAY: PROCEDURE (PTR, N) BYTE;
    DECLARE PTR ADDRESS;
    DECLARE (N, S, I) BYTE;
    S = 0;
    I = 0;
    DO WHILE I < N;
        S = S + PTR(I);
        I = I + 1;
    END;
    RETURN S;
END SUM_ARRAY;
```

**Mechanický výstup Nanz** (`mz sum.plm --emit=nanz`):
```nanz
fun sum_array(ptr: u16, n: u8) -> u8 {
    var s: u8 = 0
    var i: u8 = 0
    while i < n {
        s = s + ptr[i]
        i = i + 1
    }
    return s
}
```

**Syntaktické mapovanie:**

| PL/M-80 | Nanz | Poznámky |
|---------|------|----------|
| `PROCEDURE name(a,b) BYTE;` | `fun name(a: u8, b: u8) -> u8` | Typy inline |
| `DECLARE X BYTE;` | `var x: u8` | Konvencia malých písmen |
| `DECLARE (A,B) WORD;` | `var a: u16; var b: u16` | Multi-deklarácia rozbalená |
| `DO WHILE cond; ... END;` | `while cond { ... }` | |
| `DO I = 0 TO N; ... END;` | `for i in 0..n { ... }` | Počítaná slučka |
| `DO CASE X; ... END;` | `switch x { case 0: ...; }` | |
| `IF cond THEN s1; ELSE s2;` | `if cond { ... } else { ... }` | |
| `ARR(I)` | `arr[i]` | Notácia indexov |
| `CALL fn(a,b);` | `fn(a, b)` | |
| `DECLARE X LITERALLY 'Y'` | *(rozbalené pred parsovaním)* | Makrá odstránené |

Oba sa kompilujú na **identický Z80 assembly** — rovnaký HIR, rovnaký MIR2, rovnaký codegen.

### 11.3 Od mechanického k idiomatickému: Tri úrovne

Rovnaký algoritmus súčtu poľa demonštruje tri úrovne:

**Úroveň 1 — Mechanický PL/M preklad** (indexovaná slučka):
```nanz
fun sum_array(ptr: u16, n: u8) -> u8 {
    var s: u8 = 0
    var i: u8 = 0
    while i < n {
        s = s + ptr[i]      // random-access: ADD HL,DE per element (~15-20T)
        i = i + 1
    }
    return s
}
```

**Úroveň 2 — Idiomatický Nanz** (sekvenčný sken):
```nanz
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    for x: u8 in ptr[0..n] {   // sequential: INC HL per element (6T)
        s = s + x
    }
    return s
}
```

**Úroveň 3 — Reťazec iterátorov s uzáverom** (plne fúzovaný):
```nanz
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    ptr.forEach(|x: u8| { s = s + x }, n)  // DJNZ loop, s in register
    return s
}
```

**Náklady Z80 na element:**

| Úroveň | Kľúčová inštrukcia | T-takty/element | Poznámky |
|---------|-------------------|-----------------|----------|
| 1 (indexovaná) | `ADD HL, DE` | ~64T | Výpočet ptr+i každú iteráciu |
| 2 (for-each) | `INC HL` | ~43T | Sekvenčný posun ukazovateľa |
| 3 (forEach) | `INC HL` + `DJNZ` | ~38T | Fúzovaný, s v registri |

Pri 3,5 MHz so 100 elementmi: Úroveň 1 = 1,83 ms, Úroveň 3 = 1,09 ms — **o 40 % rýchlejšie** čisto syntaktickou zmenou.

### 11.4 PL/M vzory → Reťazce iterátorov Nanz

Najvplyvnejší preklad: manuálne PL/M slučky DO WHILE → reťazce iterátorov Nanz.

**PL/M: filtrovanie + spracovanie**
```plm
I = 0;
DO WHILE I < N;
    V = BUF(I) * 2;
    IF V > THRESHOLD THEN CALL PROCESS(V);
    I = I + 1;
END;
```

**Nanz mechanický:**
```nanz
var i: u8 = 0
while i < n {
    let v = buf[i] * 2
    if v > threshold { process(v) }
    i = i + 1
}
```

**Nanz idiomatický (fúzovaný reťazec):**
```nanz
buf.map(|x: u8| (x * 2))
   .filter(|x: u8| (x > threshold))
   .forEach(|x: u8| { process(x) }, n)
```

Verzia s reťazcom: **jedna DJNZ slučka, nulové medziľahlé polia, všetky lambdy inlinované.** Tri fázy fúzované do ~6 Z80 inštrukcií na element.

**PL/M: nájdenie maxima**
```plm
MAX = 0;
I = 0;
DO WHILE I < N;
    IF BUF(I) > MAX THEN MAX = BUF(I);
    I = I + 1;
END;
```

**Nanz idiomatický (forEach so zachytávaním):**
```nanz
var m: u8 = 0
buf.forEach(|x: u8| {
    if x > m { m = x }
}, n)
```

Zachytená premenná `m` je prevedená ako parameter bloku cez DJNZ slučku — žije v registri, nikdy sa nespilluje do pamäte.

**PL/M: transformácia na mieste**
```plm
I = 0;
DO WHILE I < N;
    BUF(I) = BUF(I) + 2;
    I = I + 1;
END;
```

**Nanz idiomatický:**
```nanz
buf.mapInPlace(|x: u8| (x + 2), n)
```

Jedna slučka: načítanie, transformácia, spätný zápis. Flag `MutateInPlace` spúšťa spätný zápis.

### 11.5 Čo PL/M nedokáže vyjadriť

Tieto vlastnosti Nanz nemajú PL/M ekvivalent:

| Vlastnosť Nanz | PL/M ekvivalent | Prečo je to dôležité |
|----------------|-----------------|---------------------|
| `u8<0..255>` rozsahové typy | Žiadny | Umožňuje LUTGen (generovanie tabuliek v čase kompilácie) |
| `^Struct` typované ukazovatele | BASED (netypovaný) | Rozlíšenie polí, auto-deref, UFCS dispatch |
| `interface Animal { speak }` | Žiadny | Kontrakt v čase kompilácie, dispatch s nulovými nákladmi |
| `buf.map().filter().forEach()` | Manuálne DO WHILE | Jedna fúzovaná slučka, žiadne medziľahlé polia |
| `\|x\| { s = s + x }` zachytávanie uzáverom | Žiadny | Premenné nesené slučkou ako parametre blokov |
| Preťažovanie operátorov | Žiadne | `a + b` na typoch štruktúr |

### 11.6 PL/M-80 V4.0 vs MIR2: Kvalita kódu

Reálne porovnanie (z Reportu #036) — **rovnaký PL/M zdrojový kód** skompilovaný pôvodným kompilátrom Intel vs naším backendom MIR2:

| Funkcia | Intel PL/M-80 V4.0 | MIR2 Z80 | Úspora |
|---------|-------------------|----------|--------|
| `abs_diff` | 33 bajtov | 12 bajtov | **−64 %** |
| `fib` | 47 bajtov | 31 bajtov | **−34 %** |
| **Celkom** | **80 bajtov** | **43 bajtov** | **−46 %** |

Kompilér Intel ukladá všetky parametre a lokálne premenné do pamäte (volacia konvencia 8080). ABI MIR2 uprednostňujúce registre drží hodnoty v A/B/C/D/HL — nulová pamäťová prevádzka v horúcich slučkách.

---

<a id="chapter-12-compiled-output-gallery"></a>

## Kapitola 12: Galéria skompilovaného výstupu

Každý blok kódu nižšie je **skutočný výstup kompilátora** z `mz <file>.nanz -o <file>.a80` na aktuálnom master builde (2026-03-10). Zdrojové súbory archivované v `reports/showcase-src/2026-03-10/`.

### 12.1 Prístup k poliam štruktúry — Optimalizácia HL-reťazca

**Zdrojový kód** (`ex1_struct.nanz`):
```nanz
struct Color { r: u8, g: u8, b: u8 }
global palette: Color

fun set_rgb(rv: u8, gv: u8, bv: u8) -> void {
    palette.r = rv
    palette.g = gv
    palette.b = bv
}
fun get_r() -> u8 { return palette.r }
fun get_g() -> u8 { return palette.g }
fun get_b() -> u8 { return palette.b }
```

**Skompilovaný Z80** (`ex1_struct.a80`):
```z80
set_rgb:
    LD HL, palette      ; one base load for all three fields     10T
    LD (HL), C          ; palette.r = rv                          7T
    INC HL              ;                                         6T
    LD (HL), D          ; palette.g = gv                          7T
    INC HL              ;                                         6T
    LD (HL), E          ; palette.b = bv                          7T
    RET                 ;                                        10T

get_r:
    LD A, (palette__r)  ; direct addressing via EQU label        13T
    RET
get_g:
    LD A, (palette__g)
    RET
get_b:
    LD A, (palette__b)
    RET

; globals
palette:
    DB 0, 0, 0
palette__r    EQU  palette
palette__g    EQU  palette + 1
palette__b    EQU  palette + 2
```

**Optimalizácia:** Detekcia HL-reťazca fúzuje tri zápisy polí do jednej sekvencie `LD HL` + `INC HL`: 53T oproti 79T naivne (−33 %).

### 12.2 UFCS dispatch metód — Nulová vtable

**Zdrojový kód** (`ex2_ufcs.nanz`):
```nanz
struct Acc { val: u8 }
global acc_g: Acc

fun Acc.add(self: ^Acc, amount: u8) -> u8 {
    self.val = self.val + amount
    return self.val
}
fun Acc.reset(self: ^Acc) -> void { self.val = 0 }

fun sum_two(a: u8, b: u8) -> u8 {
    acc_g.reset()       // UFCS → Acc_reset(&acc_g)
    acc_g.add(a)        // UFCS → Acc_add(&acc_g, a)
    acc_g.add(b)
    return acc_g.val
}
```

**Skompilovaný Z80** (`ex2_ufcs.a80`):
```z80
Acc_add:
    LD D, (HL)          ; load self.val (HL = pointer to Acc)
    LD A, D
    ADD A, C            ; + amount (C = 2nd param)
    LD C, A
    LD (HL), C          ; store back
    LD A, (HL)          ; return value
    RET

Acc_reset:
    LD C, 0
    LD (HL), C          ; self.val = 0
    RET

sum_two:
    LD HL, acc_g        ; addr_of(acc_g) — direct CALL, no vtable
    CALL Acc_reset
    LD HL, acc_g
    LD A, C             ; a
    CALL Acc_add
    LD HL, acc_g
    LD A, mem           ; b  (known bug: register spill for 2nd arg)
    CALL Acc_add
    LD A, (acc_g__val)  ; direct-address return
    RET

; globals
acc_g:
    DB 0
acc_g__val    EQU  acc_g
```

### 12.3 Dispatch rozhraní s nulovými nákladmi

**Zdrojový kód** (`ex3_iface.nanz`):
```nanz
interface Animal { speak }
struct Dog {}
struct Cat {}
global g_dog: Dog
global g_cat: Cat

fun Dog.speak(self: Dog) -> u8 { return 1 }
fun Cat.speak(self: Cat) -> u8 { return 2 }

fun demo() -> u8 { return g_dog.speak() }
```

**Skompilovaný Z80** (`ex3_iface.a80`):
```z80
Dog_speak:
    LD C, 1
    LD A, C
    RET

Cat_speak:
    LD C, 2
    LD A, C
    RET

demo:
    LD HL, g_dog
    CALL Dog_speak      ; direct CALL — no vtable, no indirection
    RET
```

17T pre dispatch (len CALL). Go-štýl interface dispatch: ~55T.

### 12.4 abs_diff — Päť optimalizačných prechodov k minimu

**Zdrojový kód** (`ex4a_abs_diff.nanz`):
```nanz
fun abs_diff(a: u8, b: u8) -> u8 {
    if a == b { return 0 }
    if a < b { return b - a }
    return a - b
}
```

**Skompilovaný Z80** (`ex4a_abs_diff.a80`):
```z80
abs_diff:
    SUB D               ; A = a - b, carry set if a < b
    LD C, A             ; (regression: contract assigned b→D, result→C)
    RET NC              ; a >= b → return a-b
.abs_diff_if_then3:
    NEG                 ; A = -(a-b) = b-a
    RET
```

BranchEquiv eliminoval stráž `a == b`. CondRetSink vytiahol `sub` pred `cmp`. CmpSubCarry eliminoval `CP`. Výsledok: 4 jadrové inštrukcie.

### 12.5 LUTGen — Slučka v čase kompilácie → 3-inštrukčné vyhľadávanie

**Zdrojový kód** (`ex5_lut.nanz`):
```nanz
fun popcount(x: u8<0..255>) -> u8 {
    var n: u8 = 0
    var v: u8 = x
    while v != 0 {
        n = n + (v & 1)
        v = v >> 1
    }
    return n
}
```

**Skompilovaný Z80** (`ex5_lut.a80`):
```z80
popcount:
    LD H, popcount_lut^H    ; 7T — page base (high byte only)
    LD L, C                 ; 4T — index (param in C)
    LD A, (HL)              ; 7T — table lookup
    RET

    ALIGN 256
popcount_lut:
    DB 0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, 2, 3, 3, 4, ...  ; 256 bytes
```

**18T celkom.** Slučka while nikdy nebeží za runtime — bola vyhodnotená v čase kompilácie MIR2 VM pre všetkých 256 vstupov.

### 12.6 forEach so zachytávaním uzáveru — DJNZ slučka

**Zdrojový kód** (`ex6_foreach.nanz`):
```nanz
fun sum_chain(buf: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    buf.forEach(|x: u8| { s = (s + x) }, n)
    return s
}

fun max_chain(buf: ^u8, n: u8) -> u8 {
    var m: u8 = 0
    buf.forEach(|x: u8| {
        if x > m { m = x }
    }, n)
    return m
}
```

**Skompilovaný Z80** (`ex6_foreach.a80`):
```z80
sum_chain:
    LD D, 0             ; s = 0
    LD B, C             ; B = n (DJNZ counter)
    LD C, D             ; C = s
.sum_chain_fe_head1:
    LD A, B
    AND A               ; n == 0?
    JRS Z, .sum_chain_fe_exit4
.sum_chain_fe_body2:
    LD D, (HL)          ; x = *buf
    LD A, C
    ADD A, D            ; s + x  (lambda body inlined)
    LD C, A
.sum_chain_fe_cont3:
    INC HL              ; buf++
    DJNZ .sum_chain_fe_body2
.sum_chain_fe_exit4:
    LD A, C
    RET

max_chain:
    LD D, 0             ; m = 0
    LD B, C             ; B = n
    LD C, D             ; C = m
.max_chain_fe_head1:
    LD A, B
    AND A
    JRS Z, .max_chain_fe_exit4
.max_chain_fe_body2:
    LD D, (HL)          ; x = *buf
    LD A, C
    CP D                ; m > x?
    JRS NC, .max_chain_trmp0
.max_chain_if_then5:
.max_chain_fe_cont3:
    INC HL
    DEC B
    LD C, D             ; m = x (captured var update)
    JRS .max_chain_fe_head1
.max_chain_fe_exit4:
    LD A, C
    RET
.max_chain_trmp0:
    LD D, C
    JRS .max_chain_if_join6
```

**sum_chain: 38T/element.** Žiadne CALL na lambdu. Zachytená `s` žije v C po celý čas.

### 12.7 mapInPlace — Transformácia na mieste so spätným zápisom

**Zdrojový kód** (`ex7_mapinplace.nanz`):
```nanz
fun add2_inplace(buf: ^u8, n: u8) -> void {
    buf.mapInPlace(|x: u8| (x + 2), n)
}
```

**Skompilovaný Z80** (`ex7_mapinplace.a80`):
```z80
add2_inplace:
    LD B, C             ; B = n
.add2_inplace_fe_head1:
    LD A, B
    AND A
    JRS Z, .add2_inplace_fe_exit4
.add2_inplace_fe_body2:
    LD C, (HL)          ; load element
    LD D, 2             ; (dead load — regression)
    INC C               ; +1
    INC C               ; +1 (INC C × 2 instead of ADD A,2)
    LD (HL), C          ; write back
.add2_inplace_fe_cont3:
    INC HL
    DJNZ .add2_inplace_fe_body2
.add2_inplace_fe_exit4:
    RET

lambda_0:               ; standalone lambda emitted but never called
    LD C, 2
    ADD A, C
    LD C, A
    RET
```

**Známa regresia:** `LD D, 2` je mŕtve, `INC C; INC C` nahrádza `ADD A, 2` (51T oproti 40T/element). Samostatná `lambda_0` je tiež mŕtvy kód.

### 12.8 GCD — Slučka s dvoma mutovateľnými premennými

**Zdrojový kód** (`ex8_gcd.nanz`):
```nanz
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else     { b = b - a }
    }
    return a
}
```

**Skompilovaný Z80** (`ex8_gcd.a80`):
```z80
gcd:
.gcd_loop_head1:
    LD A, C
    CP D                ; a == b?
    JRS Z, .gcd_trmp0
.gcd_loop_body2:
    LD A, D
    CP C                ; a > b?
    JRS NC, .gcd_if_else6
.gcd_if_then4:
    LD A, C
    SUB D               ; a = a - b
    LD C, A
.gcd_if_join5:
    JRS .gcd_loop_head1
.gcd_if_else6:
    LD A, D
    SUB C               ; b = b - a
    LD D, A
    JRS .gcd_if_join5
.gcd_loop_exit3:
    LD A, D
    RET
.gcd_trmp0:
    LD A, C
    LD C, D
    LD D, A
    JRS .gcd_loop_exit3
```

Koalescer fázy 6c eliminoval všetku réžiu kopírovania na hraniciach blokov z horúcej slučky. Len výstupný trampolín (studená cesta) má výmeny registrov.

---

<a id="chapter-13-relation-to-minz-and-plm"></a>

## Kapitola 13: Vzťah k MinZ a PL/M

### 13.1 Tri frontendy

| Jazyk | Prípona | Parser | Backend | Stav |
|-------|---------|--------|---------|------|
| MinZ | `.minz` | `pkg/parser/participle/` | MIR1 → starý codegen | Zmrazený |
| Nanz | `.nanz` | `pkg/nanz/parse.go` | HIR → MIR2 → Z80 | **Aktívny** |
| PL/M-80 | `.plm` | `pkg/plm/` | HIR → MIR2 → Z80 | Aktívny |

Smerovač v `cmd/minzc/main.go`:
```go
if ext == ".plm" || ext == ".nanz" {
    return compileViaHIR(src, ext)
} else {
    // old MIR1 path for .minz
}
```

### 13.2 Kedy použiť ktorý

**Nanz** — nové Z80/CP/M programy, moderná syntax, LUTGen, PBQP alokácia.
**MinZ** — existujúce `.minz` programy, ktoré fungujú. Má metafunkcie (`@define`, `@print`), ktoré zatiaľ nie sú v Nanz.
**PL/M-80** — portovanie staršieho CP/M softvéru. 100 % korpusu Intel 80 Tools sa parsuje.

### 13.3 Chýbajúce vlastnosti (Nanz vs. MinZ)

Nanz v súčasnosti nemá:
- `@define` preprocesorové makrá
- `@print` optimalizovaný výstup reťazcov
- Ruby-štýl interpolácia reťazcov (`"Hello #{name}"`)
- Syntax pre viacero návratových hodnôt (MIR2 to podporuje, parser nie)
- Preťažovanie funkcií (MinZ to má; Nanz vyžaduje odlišné názvy)
- `@extern` s anotáciami tried registrov (dokumentované, ale zatiaľ neparsované)

### 13.4 Ďalšie backendy (len MinZ)

Starý pipeline MinZ (flag `-b`) podporuje viacero cieľov. Tieto NIE SÚ dostupné pre Nanz (ktorý vždy prechádza cez MIR2 → Z80):

| Backend | Flag | Stav | Výstup |
|---------|------|------|--------|
| Z80 | `-b z80` | Produkcia | `.a80` |
| 6502 | `-b 6502` | Beta | `.s` |
| 68000 | `-b m68k` | Alfa | `.s` |
| i8080 | `-b i8080` | Beta | `.s` |
| Game Boy | `-b gb` | Aktívny | `.s` |
| WASM | `-b wasm` | Alfa | `.wat` |
| C | `-b c` | Beta | `.c` |
| Crystal | `-b crystal` | Beta | `.cr` |
| LLVM | `-b llvm` | Plánovaný | `.ll` |

Tieto používajú starý IR (MIR1), nie MIR2. Dlhodobý plán je vyradiť MIR1 a smerovať všetky frontendy cez HIR → MIR2.

---

<a id="appendix-a-complete-syntax-grammar"></a>

## Príloha A: Kompletná gramatika syntaxe

```ebnf
module      = top_decl*
top_decl    = struct_decl
            | interface_decl
            | global_decl
            | fun_decl
            | '@extern' 'fun' fun_decl_inner

struct_decl    = 'struct' IDENT '{' field_decl* '}'
field_decl     = IDENT ':' type ','?

interface_decl = 'interface' IDENT '{' method_name* '}'
method_name    = 'fun'? IDENT ','?

global_decl    = 'global' IDENT ':' type at_clause? init_clause?
at_clause      = 'at' '(' expr ')'
init_clause    = '=' ('[' expr (',' expr)* ']' | expr)

fun_decl       = ('fun' | 'fn') fun_decl_inner
fun_decl_inner = (op_symbol | IDENT ('.' IDENT)?) '(' params ')' ('->' type)?
                 ('{' stmt* '}' | /* extern: no body */)
params         = (IDENT ':' type (',' IDENT ':' type)*)?
op_symbol      = '+' | '-' | '*' | '/' | '%'
               | '==' | '!=' | '<' | '<=' | '>' | '>='
               | '&' | '|' | '^'

type           = '^' type
               | '[' type ';' INT ']'
               | 'u8' ('<' INT '..' INT '>')?
               | 'u16' ('<' INT '..' INT '>')?
               | 'i8' | 'i16' | 'bool' | 'void' | 'ptr'
               | IDENT     (* struct or interface name *)

stmt           = var_decl | let_decl | if_stmt | while_stmt
               | for_stmt | return_stmt | 'break' | 'continue'
               | switch_stmt | block | expr_stmt

var_decl       = 'var' IDENT ':' type at_clause? ('=' (array_init | expr))?
let_decl       = 'let' IDENT (':' type)? '=' expr
array_init     = '[' expr (',' expr)* ']'

if_stmt        = 'if' expr block ('else' block)?
while_stmt     = 'while' expr block
for_stmt       = 'for' IDENT (':' type)? 'in'
                 (expr '[' expr? '..' expr ']' block    (* ForEachStmt *)
                 | expr '..' expr block)                 (* ForRangeStmt *)
return_stmt    = 'return' expr?
switch_stmt    = 'switch' expr '{' case_clause* default_clause? '}'
case_clause    = 'case' INT ':' stmt*
default_clause = 'default' ':' stmt*
block          = '{' stmt* '}'

expr_stmt      = expr ('=' expr)?     (* assignment or bare call *)

expr           = binary_expr
binary_expr    = unary_expr (binop binary_expr)*
binop          = '+' | '-' | '*' | '/' | '%' | '&' | '|' | '^'
               | '<<' | '>>' | '==' | '!=' | '<' | '<=' | '>' | '>='

unary_expr     = '-' unary_expr
               | '!' unary_expr
               | '~' unary_expr
               | '&' IDENT
               | postfix_expr

postfix_expr   = primary
                 ( '^'                      (* dereference *)
                 | '[' expr ']'             (* index *)
                 | '.' IDENT               (* field access *)
                 | '.' IDENT '(' args ')'  (* UFCS method call *)
                 | '(' args ')'            (* function call *)
                 )*

primary        = INT | 'true' | 'false' | STRING
               | ('u8' | 'u16' | 'i8' | 'i16') '(' expr ')'   (* cast *)
               | '@ptr' '(' type ',' expr ')'
               | '|' lambda_params '|' (block | expr)
               | '(' expr ')'
               | IDENT

lambda_params  = (IDENT (':' type)? (',' IDENT (':' type)?)*)?
args           = (expr (',' expr)*)?
```

**Lexikálne poznámky:**
- Komentáre: `//` (riadkový) a `/* */` (blokový)
- Biele znaky sú bezvýznamné
- Celé čísla: desiatkové alebo `0x`/`0X` šestnástkové
- Reťazce: v úvodzovkách, bez escape sekvencií

---

<a id="appendix-b-register-classes-and-cost-table"></a>

## Príloha B: Triedy registrov a tabuľka nákladov

### Fyzické registre Z80

| Register | Trieda | Náklady | Poznámky |
|----------|--------|---------|----------|
| A | ClassAcc | 0T | ALU akumulátor, návratová hodnota, 1. u8 parameter |
| B | ClassCounter | 0T | DJNZ počítadlo, 3. parameter |
| C, D, E, H, L | ClassGeneral | 0T | Všeobecné 8-bitové |
| HL | ClassPointer | 0T | Primárny ukazovateľ, 1. u16/ptr parameter, návrat |
| DE | ClassIndex | 0T | 2. u16 parameter, zdroj LDIR |
| BC | ClassPair | 0T | 3. u16 parameter |
| IX | ClassIX | 8T | Záložný ukazovateľ (+4T DD prefix za prístup) |
| IY | ClassIY | 8T | Zriedka používaný (na niektorých platformách rezervovaný systémom) |
| IXH/IXL | ClassIXY8 | 8T | Nedokumentované 8-bitové polovice |
| $F0xx | ClassMem | 26T | Pamäťou podložené „rozšírenie súboru registrov" |

### Hierarchia tried registrov

| Úroveň | Triedy | Náklady | Mechanizmus |
|---------|--------|---------|-------------|
| 0 — Primárne | Acc, Counter, General, Pointer, Index, Pair, Flag | 0T | Priame použitie |
| 1 — IX/IY | IX, IY, IXY8 | 4-8T | DD/FD prefix |
| 2 — Tieňové | Shadow, AccShadow | 8T | EXX / EX AF,AF' |
| 3 — Zásobník | Stack | 21T | PUSH + POP |
| 4 — Pamäť | Mem | 26T | LD (addr) / LD addr |

ClassFlag je špeciálny: reprezentuje Z80 CPU flagy (Z, CY, atď.) a stojí 0T. Booleovské výsledky porovnaní sa uchovávajú vo flagoch bez materializácie do registra.

---

<a id="appendix-c-cli-reference"></a>

## Príloha C: Referencia CLI

### Kompilácia programov Nanz

```bash
mz source.nanz -o output.a80              # compile to Z80 assembly
mz source.nanz -o output.a80 --target=cpm # target CP/M
mz source.nanz -o output.a80 --target=spectrum  # target ZX Spectrum
mz source.nanz -o output.a80 --target=agon      # target Agon Light 2
```

### Medziprezentácie

```bash
mz source.nanz --emit=hir        # HIR dump
mz source.nanz --emit=mir2-raw   # MIR2 before optimization
mz source.nanz --emit=mir2       # MIR2 after optimization
mz source.plm  --emit=nanz       # PL/M → Nanz translation
```

### Optimalizačné flagy

```bash
mz source.nanz --disable-optimize      # disable all optimizations
mz source.nanz --disable-ir-opt        # disable MIR-level opts
mz source.nanz --disable-asm-opt       # disable peephole
mz source.nanz --disable-smc           # disable self-modifying code
mz source.nanz --compile-trace         # show all optimization steps
```

### Spúšťanie testov

```bash
cd minzc

# All Go tests (23+ packages)
go test ./pkg/... -vet=off

# Nanz parser tests only
go test ./pkg/nanz/... -vet=off -v

# MIR2 tests (LUTGen, contracts, PBQP)
go test ./pkg/mir2/... -vet=off -v

# QBE E2E tests (requires qbe and cc in PATH)
go test ./pkg/mir2qbe/... -vet=off -v
```

### Príklady programov

| Súbor | Popis |
|-------|-------|
| `examples/nanz/01_sum_array.nanz` | While slučka s `ptr[i]` |
| `examples/nanz/02_sum_array_idiomatic.nanz` | For-each a forEach iterátor |
| `examples/nanz/03_filter_map_chain.nanz` | Kompletný reťazec map/filter/forEach |
| `examples/nanz/04_lut_popcount.nanz` | LUT generovanie cez rozsahový typ |
| `examples/nanz/05_four_pointers.nanz` | PBQP: 4 ClassPointer registre |
| `examples/nanz/06_pbqp_weighted.nanz` | Vážená alokácia nákladov |
| `examples/nanz/07_ix_load_store.nanz` | IX overflow adresovanie |

---

<a id="appendix-d-installing-external-tools"></a>

## Príloha D: Inštalácia externých nástrojov

### QBE (oracle správnosti)

QBE je potrebný len na spúšťanie E2E testov správnosti (`pkg/mir2qbe/`). NIE JE potrebný pre bežnú kompiláciu Nanz.

**Linux (zostavenie zo zdrojového kódu):**
```bash
git clone git://c9x.me/qbe.git
cd qbe
make
sudo cp qbe /usr/local/bin/
```

**macOS:**
```bash
brew install qbe
```

**Overenie:**
```bash
qbe --version        # should print version
echo 'export function w $main() { @start ret 0 }' | qbe
```

**C kompilér** je tiež potrebný (akýkoľvek C99 kompilér: `gcc`, `clang`). Zvyčajne predinštalovaný.

Ak `qbe` nie je v PATH, E2E testy sa automaticky preskočia s `t.Skip("qbe not in PATH")`.

### Assembler MZA (vstavaný)

Žiadna externá inštalácia nie je potrebná. MZA je súčasťou toolchainu MinZ:

```bash
cd minzc && make mza
# or: make install-user (installs all tools to ~/.local/bin/)
```

### Emulátor MZE (vstavaný)

```bash
cd minzc && make mze
```

Používa sa na: spúšťanie skompilovaných Z80 bináriek, vyhodnocovanie konštánt vnútri kompilátora (LUTGen, BranchEquiv).

---

<a id="appendix-e-known-bugs-and-limitations"></a>

## Príloha E: Známe chyby a obmedzenia

### Parser

| Problém | Stav | Riešenie |
|---------|------|----------|
| `@extern` s anotáciami `params=`/`returns=` | Neimplementované | Použite základný `@extern fun` (automatické priradenie registrov) |
| Preťažovanie funkcií | Neimplementované | Použite odlišné názvy (`abs_diff`, `abs_diff_u16`) |
| Viacero návratových hodnôt | Neparsované | MIR2 to podporuje; parser nie |
| Escape sekvencie v reťazcoch | Neimplementované | Žiadne `\n`, `\t` atď. |
| `@asm` bloky inline assembly | Neimplementované | Použite `@extern` s asm wrappermi |

### Codegen

| Problém | Stav | Detaily |
|---------|------|---------|
| `applySubSwapNeg` na u16 | Chyba | Vynucuje ClassAcc (8-bit) na 16-bitovom výsledku NEG. Chýba stráž `Ty.Width() <= 8`. |
| Globály štruktúr s nulovou veľkosťou | Chyba | `struct Dog {}` neemituje žiadne dáta; symbol nedefinovaný v čase linkovania. Oprava: emitovať `Dog: EQU $`. |
| Sentinel `LD A, mem` | Chyba | Zlyhanie alokácie registrov emituje doslovný reťazec "mem" v assembly. |
| Caller-save pre 2. argument | Chyba | Druhý argument pri opakovaných volaniach môže byť prepísaný. |
| Konštantné sčítanie mapInPlace | Regresia | `ADD A, imm` sa nespustí keď je element v C (nie A). |

### Reťazce iterátorov

| Problém | Stav | Detaily |
|---------|------|---------|
| `enumerate` | Nefunkčný na Z80 | B = počítadlo konflikty s B = enumeračný index |
| `reduce` | Nefunkčný na Z80 | A prepísaný medzi dvoma SMC parametrami |
| Zachytávanie uzáverom v nefúzovaných | Nedefinované správanie | Lambdy odovzdané ako ukazovatele na funkcie nemôžu zachytávať vonkajšie lokálne premenné |

### Alokátor registrov

| Problém | Stav | Detaily |
|---------|------|---------|
| Pamäťou podložené registre | Výkon | ~207T skutočné oproti ~43T ideálne na element iterátora pri spille do $F0xx |
| Drift optimalizátora kontraktov | Známy | Môže priradiť suboptimálne triedy (napr. `b → D` namiesto `b → B` pre abs_diff) |
| Náklady hrán PBQP | Odložené | Korelované alokačné rozhodnutia (BC* pre LUT) zatiaľ nemodelované |

---

*Nanz: Moderná syntax, abstrakcie s nulovými nákladmi, výkon Z80.*

*Pipeline: `.nanz` → `nanz.Parse()` → `*hir.Module` → `hir.LowerModule()` → `*mir2.Module` → optimalizácia → alokácia → Z80Codegen → `.a80`*
