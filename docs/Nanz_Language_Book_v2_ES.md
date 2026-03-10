# El Lenguaje Nanz: Guía Completa (v2)

> Nanz es un lenguaje de sistemas tipado para Z80 y plataformas retro.
> Compila a través de HIR → MIR2 → ensamblador Z80 — un pipeline limpio y moderno
> compartido con el frontend de PL/M-80.

**Versión:** MinZ compiler v0.19.5 (pipeline MIR2)
**Fecha:** 2026-03-10
**Estado:** En desarrollo activo — funcionalidades principales estables, algunos detalles sin pulir
**Fuente de verdad:** `pkg/nanz/parse.go`, `pkg/hir/`, `pkg/mir2/`, `pkg/pipeline/`

---

## Tabla de Contenidos

1. [¿Qué es Nanz?](#chapter-1-what-is-nanz)
2. [Referencia de Sintaxis](#chapter-2-syntax-reference)
3. [Sistema de Tipos](#chapter-3-type-system)
4. [Pipeline de Compilación](#chapter-4-compilation-pipeline)
5. [Representación Intermedia MIR2](#chapter-5-mir2-intermediate-representation)
6. [Pasadas de Optimización](#chapter-6-optimization-passes)
7. [Generación de Código Z80](#chapter-7-z80-code-generation)
8. [Cadenas de Iteradores y Fusión](#chapter-8-iterator-chains-and-fusion)
9. [Abstracciones de Coste Cero](#chapter-9-zero-cost-abstractions)
10. [Oráculo de Corrección QBE](#chapter-10-qbe-correctness-oracle)
11. [PL/M-80: Corpus de Pruebas y Traducción Idiomática](#chapter-11-plm-80-corpus-seeding-and-idiomatic-translation)
12. [Galería de Salida Compilada](#chapter-12-compiled-output-gallery)
13. [Relación con MinZ y PL/M](#chapter-13-relation-to-minz-and-plm)
- [Apéndice A: Gramática Completa de la Sintaxis](#appendix-a-complete-syntax-grammar)
- [Apéndice B: Clases de Registros y Tabla de Costes](#appendix-b-register-classes-and-cost-table)
- [Apéndice C: Referencia de CLI](#appendix-c-cli-reference)
- [Apéndice D: Instalación de Herramientas Externas](#appendix-d-installing-external-tools)
- [Apéndice E: Errores Conocidos y Limitaciones](#appendix-e-known-bugs-and-limitations)

---

## Capítulo 1: ¿Qué es Nanz?

Nanz (extensión `.nanz`) es el lenguaje frontend activo del compilador MinZ. Es de tipado estático, imperativo, y está diseñado para dos audiencias:

1. Desarrolladores que escriben programas para Z80 / plataformas retro y desean sintaxis moderna con abstracciones de coste cero.
2. El equipo del compilador MinZ que desarrolla y prueba el backend MIR2.

### 1.1 El Pipeline de Compilación

```
source.nanz
    │  nanz.Parse()
    ▼
*hir.Module          ← High-level IR (flujo de control estructurado, variables con nombre)
    │  hir.LowerModule()
    ▼
*mir2.Module         ← Mid-level IR (tipo SSA, registros virtuales, operaciones tipadas)
    │  optimization passes (constant fold, DSE, BranchEquiv, CondRetSink, LUTGen)
    ▼
*mir2.Module         ← optimizado
    │  OptimizeContracts() → PBQPAllocate()
    ▼
*mir2.Module         ← asignado (registros virtuales → físicos)
    │  Z80Codegen()
    ▼
output.a80           ← ensamblador Z80 compatible con MZA
    │  mza (ensamblador)
    ▼
output.bin / .tap    ← binario / imagen de cinta ZX Spectrum
```

Se invoca con:

```bash
mz source.nanz -o output.a80
```

### 1.2 Nanz vs. MinZ

MinZ (`.minz`) es el frontend original con su propio parser y generador de código que apunta al IR anterior MIR1. Ese pipeline está **congelado** — funciona pero no se está desarrollando.

Nanz apunta a MIR2, que proporciona:

- Flujo de datos tipo SSA con parámetros de bloque (estilo Cranelift/MLIR, no nodos phi)
- Asignador de registros PBQP con vectores de coste ponderados
- Generación de LUT: evaluación en tiempo de compilación para funciones puras con rangos acotados
- Optimización interprocedimental de convenciones de llamada
- Un emulador Z80 usado como evaluador de constantes y demostrador de equivalencia de ramas

**Regla:** Escribe los programas retro nuevos en Nanz. Mantén los programas `.minz` existentes tal como están.

### 1.3 Nanz vs. PL/M-80

PL/M-80 (`.plm`) es el lenguaje de Intel de los años 70 para CP/M. El compilador MinZ incluye un parser de PL/M-80 que compila a través del **mismo pipeline HIR → MIR2 → Z80** que Nanz:

```bash
mz legacy.plm -o legacy.a80     # mismo backend que Nanz
mz legacy.plm --emit=nanz       # traducir PL/M a código fuente Nanz
```

### 1.4 Filosofía de Diseño

Nanz es deliberadamente minimalista. La gramática cabe en una pantalla. Sin recolector de basura, sin runtime, sin dispatch dinámico. Cada abstracción — lambdas, iteradores, métodos de structs, interfaces — compila a instrucciones Z80 directas sin overhead más allá de lo que escribirías a mano.

---

## Capítulo 2: Referencia de Sintaxis

El parser es un parser recursivo descendente escrito a mano con un parser de expresiones Pratt (`pkg/nanz/parse.go`). Fuente: ~2000 LOC en Go.

### 2.1 Estructura de Módulo

Un archivo fuente Nanz es un módulo: una secuencia de declaraciones de nivel superior en cualquier orden.

```nanz
// line comment
/* block comment */

struct Point { x: u8, y: u8 }
interface Drawable { draw }
global counter: u8
fun add(a: u8, b: u8) -> u8 { return a + b }
```

No hay imports; el enlazado se maneja a nivel de ensamblador.

### 2.2 Tipos

| Sintaxis | Ancho | Mapeo Z80 | Notas |
|----------|-------|-----------|-------|
| `u8` | 8 bit | A/B/C/D/E/H/L | byte sin signo |
| `u16` | 16 bit | HL/DE/BC | word sin signo |
| `i8` | 8 bit | mismos regs, aritmética diferente | byte con signo |
| `i16` | 16 bit | igual | word con signo |
| `bool` | 8 bit | false=0, true=1 | |
| `void` | — | solo tipo de retorno | |
| `ptr` | 16 bit | HL/DE/BC | puntero sin tipo |
| `^T` | 16 bit | HL/DE/BC | puntero tipado a T |
| `[T; N]` | N×width(T) | — | array de tamaño fijo |
| `u8<lo..hi>` | 8 bit | — | tipo con rango (candidato a LUT) |
| `u16<lo..hi>` | 16 bit | — | tipo con rango |
| `StructName` | suma de campos | pasado por puntero | tipo struct |
| `InterfaceName` | — | resuelto en tiempo de compilación | interfaz como tipo de parámetro |

**Tipos puntero:** `^T` registra el tipo del elemento para la resolución de campos. Cuando T es un struct, `^Struct` permite el acceso tipado a campos a través del puntero (por ejemplo, `self.val` en un receptor `^Acc` auto-desreferencia y resuelve los offsets de campo). Esto NO es meramente azúcar sintáctico — el parser usa `varPtrElem[paramName]` para rastrear el struct apuntado para la resolución de campos y el dispatch UFCS.

**Tipos con rango:** `u8<0..255>` declara una entrada con rango. El rango es inclusivo en el código fuente (`0..255`), almacenado exclusivo internamente (`[0, 256)`). Las funciones con un único parámetro con rango y cuerpo puro son candidatas para la generación de LUT (ver §6.4).

### 2.3 Funciones

```nanz
fun name(param1: Type1, param2: Type2) -> ReturnType {
    // body
}
```

`fn` se acepta como alias de `fun`. Las funciones sin valor de retorno omiten `-> ReturnType` (implícitamente `void`).

```nanz
fn add(a: u8, b: u8) -> u8 { return a + b }

fun clear(buf: ^u8, n: u8) {
    var i: u8 = 0
    while i < n { buf[i] = 0; i = i + 1 }
}
```

### 2.4 Funciones Extern

Las funciones implementadas fuera del módulo Nanz se declaran con `@extern`:

```nanz
@extern fun process(x: u8) -> void
@extern fun rom_print(s: ptr) -> void
```

Se omite el cuerpo. El compilador asigna clases de registros a los parámetros siguiendo la convención de llamada estándar.

**Estado:** La forma anotada `@extern("sym", params=[z80_a], returns=[z80_a])` descrita en alguna documentación **aún no está implementada** en el parser. Actualmente, las funciones `@extern` solo reciben asignación automática de registros.

### 2.5 Declaraciones de Variables

**`var` — tipo explícito, inicializador opcional:**

```nanz
var i: u8 = 0
var buf: ^u8
var port: u8 at(0xFE)          // memory-mapped at absolute address
```

**`let` — tipo inferido del inicializador:**

```nanz
let x = 42                      // x: u8
let ptr = @ptr(u8, 0xFE)       // ptr: ptr
let y: u16 = 1000              // explicit type override
```

La cláusula `at(addr)` mapea una variable a una dirección de memoria fija. Las lecturas y escrituras se convierten en `LD (addr), r` / `LD r, (addr)`.

### 2.6 Variables Globales

```nanz
global counter: u8
global vram: u8 at(0x4000)            // memory-mapped
global table: [u8; 4] = [1, 2, 4, 8]  // initialized array
global palette: Color                  // struct global
```

Las globales se emiten en la sección de datos de la salida `.a80`. Las globales de array con inicializadores generan directivas `DB`.

### 2.7 Literales

```nanz
42          // decimal — u8 if ≤255, u16 otherwise
0xFF        // hexadecimal
0           // zero (u8)
true        // bool
false       // bool
"hello"     // string → ptr to NUL-terminated bytes (interned)
```

### 2.8 Operadores

**Aritméticos** (asociativos por la izquierda):
`+` `-` `*` `/` `%`

**Bit a bit:**
`&` `|` `^` (XOR) `~` (NOT, unario) `<<` `>>`

**Comparación** (producen `bool`):
`==` `!=` `<` `<=` `>` `>=`

**Lógicos:**
`!` (NOT, unario)

**Precedencia** (de mayor a menor):

| Nivel | Operadores |
|-------|------------|
| 8 | `*` `/` `%` |
| 7 | `+` `-` |
| 6 | `<<` `>>` |
| 5 | `<` `<=` `>` `>=` |
| 4 | `==` `!=` |
| 3 | `&` |
| 2 | `^` (XOR) |
| 1 | `\|` |

Los paréntesis anulan la precedencia: `(a + b) * c`.

### 2.9 Operaciones con Punteros

```nanz
let val = ptr^            // dereference (load byte at ptr)
^ptr = 42                 // store through pointer
let b = buf[3]            // index load (buf + 3)
buf[i] = 0               // index store
let p = &counter          // address-of global
let kb = @ptr(u8, 0xFE)  // typed constant pointer to absolute address
```

`@ptr(T, addr)` es la forma idiomática de referenciar registros de hardware y rutinas ROM.

### 2.10 Conversiones de Tipo (Casts)

```nanz
u8(expr)     // truncate to u8
u16(expr)    // zero-extend to u16
i8(expr)     // reinterpret as signed 8-bit
i16(expr)    // reinterpret as signed 16-bit
```

Las conversiones son explícitas — no hay ensanchamiento implícito. En Z80, `u8→u16` cuesta `LD H, 0 / LD L, A`; el compilador no oculta eso.

### 2.11 Flujo de Control

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

`for x: u8 in ptr[start..end]` compila a un bucle DJNZ con `HL` como puntero y `B` como contador — el bucle Z80 más ajustado posible.

Los casos de switch no caen al siguiente. El cuerpo de cada caso termina en el siguiente `case`, `default`, o `}`.

### 2.12 Structs

```nanz
struct Point { x: u8, y: u8 }
struct Vec3d { x: u16, y: u16, z: u8 }
```

Los campos se disponen secuencialmente, sin relleno. Los offsets se calculan en tiempo de parseo:
- `Point.x` → offset 0, `Point.y` → offset 1
- `Vec3d.y` → offset 2, `Vec3d.z` → offset 4

Los valores de struct siempre se pasan **por puntero** (HL en Z80). Acceso a campos: `Load(ptr + offset)`.

### 2.13 Métodos de Struct y UFCS

Los métodos se declaran con la sintaxis `TypeName.methodName`:

```nanz
struct Acc { val: u8 }

fun Acc.add(self: ^Acc, amount: u8) -> u8 {
    self.val = self.val + amount
    return self.val
}
```

El compilador almacena esto como `Acc_add`. En los puntos de llamada, UFCS reescribe:

```nanz
acc_g.add(5)    // → Acc_add(&acc_g, 5) — direct CALL, no vtable
```

**Cómo funciona la resolución UFCS** (en tiempo de parseo):
1. El parser ve `base.method(args)`.
2. Busca `exprTy(base)` — si es un struct, verifica `methodTable[structName][method]`.
3. Si base es un puntero `^Struct`, verifica `varPtrElem[name]` para el tipo struct.
4. Si base es una variable con tipo de interfaz, llama a `findImplementors()`.
5. Reescribe a `CallExpr{Fn: "StructName_method", Args: [base, args...]}`.

**Receptores por puntero** (`self: ^Acc`):
- `self.val` auto-desreferencia — resuelve el offset del campo a través de `varPtrElem`.
- `self^.val` también funciona (desreferencia explícita, mismo resultado).
- El puntero viaja en HL (ClassPointer). Lecturas de campo: `LD reg, (HL)` en offset 0, `INC HL; LD reg, (HL)` en offset 1, etc.

**Receptores por valor** (`self: Acc`):
- El struct se pasa por puntero — igual que `^Acc` a nivel de ABI.
- El parser registra en `methodTable` con información completa del tipo de retorno.

### 2.14 Sobrecarga de Operadores

```nanz
fun +(a: Vec2, b: Vec2) -> Vec2 { return a }
fun -(a: Vec2, b: Vec2) -> Vec2 { return a }

fun compute(a: Vec2, b: Vec2) -> Vec2 {
    return a + b    // dispatched to op_add(a, b) because a is Vec2
}
```

Operadores sobrecargables: `+` `-` `*` `/` `%` `==` `!=` `<` `<=` `>` `>=` `&` `|` `^`.

Nombres decorados: `op_add`, `op_sub`, `op_mul`, `op_div`, `op_rem`, `op_eq`, `op_ne`, `op_lt`, `op_le`, `op_gt`, `op_ge`, `op_and`, `op_or`, `op_xor`.

**Importante:** Para tipos primitivos (`u8 + u8`), siempre se usa el `BinExpr` integrado, independientemente de las sobrecargas en el ámbito. Las sobrecargas solo se activan cuando el operando izquierdo es un tipo struct.

### 2.15 Interfaces

```nanz
interface Animal {
    speak
    move
}
```

Las interfaces son **contratos en tiempo de compilación**. Sin vtable, sin puntero gordo, sin tabla de dispatch de métodos.

**Como tipo de parámetro:**

```nanz
fun feed(a: Animal) -> u8 {
    return a.speak()      // monomorphized at compile time
}
```

**Reglas de resolución:**
- **Un implementador** → monomorfizar: `a.speak()` → `Dog_speak(a)`.
- **Múltiples implementadores** → error de compilación: `"ambiguous dispatch: ... multiple types implement Animal: [Dog Cat]; use concrete type"`.
- **Cero implementadores** → error de compilación.

**Interfaz como tipo global:**

```nanz
global g_thing: Drawable
```

Funciona igual: el dispatch UFCS se resuelve al implementador único.

### 2.16 Lambdas

```nanz
let double = |x: u8| x * 2          // expression body
let add = |a: u8, b: u8| a + b      // multi-param
let greet = |x: u8| { return x + 1 }  // block body
let inc = |x| x + 1                 // type defaults to u8
```

Cada lambda se convierte en una función de nivel superior `lambda_N` (contador secuencial). La referencia en el punto de llamada es un `VarRefExpr{Name: "lambda_N"}`.

**Reglas de captura:**
- **Lambdas de iteradores fusionados** (forEach/map/filter): pueden capturar y mutar variables locales externas. El compilador detecta variables libres mediante `hasFreeVars()`, omite el lowering independiente, y propaga las variables capturadas como parámetros de bloque a través del bucle DJNZ. Cero heap, cero derrame.
- **Lambdas no fusionadas** (punteros a función): no pueden capturar variables locales externas — sin frame en tiempo de ejecución. Solo accesibles las globales.

### 2.17 Métodos de Iterador (UFCS sobre Punteros)

```nanz
buf.forEach(|x: u8| { process(x) }, n)       // execute for each element
buf.map(|x: u8| (x * 2))                     // transform elements
buf.filter(|x: u8| (x > 5))                  // keep matching elements
buf.mapInPlace(|x: u8| (x + 2), n)           // in-place transform
```

Estos se reconocen por el parser como llamadas a métodos UFCS sobre expresiones de puntero. El lowerer de HIR `tryLowerIterChain` fusiona las cadenas en un único bucle DJNZ. Ver Capítulo 8.

---

## Capítulo 3: Sistema de Tipos

### 3.1 Tipos MIR2

El IR MIR2 (`pkg/mir2/types.go`) soporta:

| Tipo | Ancho | Representación Go |
|------|-------|-------------------|
| `TyVoid` | 0 | `&IntTy{Bits: 0}` |
| `TyBool` | 8 | `&IntTy{Bits: 8}` |
| `TyU8` | 8 | `&IntTy{Bits: 8, Signed: false}` |
| `TyI8` | 8 | `&IntTy{Bits: 8, Signed: true}` |
| `TyU16` | 16 | `&IntTy{Bits: 16, Signed: false}` |
| `TyI16` | 16 | `&IntTy{Bits: 16, Signed: true}` |
| `TyU24` | 24 | `&IntTy{Bits: 24}` (futuro eZ80) |
| `TyPtr` | 16 | `&PtrTy{}` |
| `StructTy` | suma de campos | `&StructTy{Name, Fields}` |
| `ArrayTy` | N×elem | `&ArrayTy{Len, Elem, Layout}` |
| `TupleTy` | suma de elems | `&TupleTy{Elems}` (retorno múltiple) |

Adicionalmente, los tipos con rango envuelven un tipo base con límites `[Lo, Hi)`:
```go
type RangedTy struct {
    Base Ty
    Lo, Hi int  // [Lo, Hi) — exclusive upper bound
}
```

### 3.2 Asignación de Clases de Registros

Los parámetros se asignan a clases de registros según su posición y tipo (`classForParam` en `hir/lower.go`):

| Posición | Tipo | Clase | Físico Z80 |
|----------|------|-------|------------|
| 1.° | `u8`, `bool` | `ClassAcc` | A |
| 1.° | `u16`, `ptr`, struct | `ClassPointer` | HL |
| 2.° | `u8` | `ClassGeneral` | C |
| 2.° | `u16` | `ClassIndex` | DE |
| 3.° | `u8` | `ClassCounter` | B |
| 3.°+ | `u16` | `ClassPair` | BC/DE |

Valores de retorno: `u8` → `ClassAcc` (A), `u16`/`ptr` → `ClassPointer` (HL).

### 3.3 Disposición de Structs

Los campos se empaquetan secuencialmente, sin relleno de alineación:

```nanz
struct Color { r: u8, g: u8, b: u8 }  // total: 3 bytes
// r at offset 0, g at offset 1, b at offset 2
```

El parser calcula los offsets en tiempo de parseo sumando `Ty.Width() / 8` por cada campo precedente. Los campos de structs globales reciben etiquetas EQU en ensamblador: `palette__r EQU palette + 0`.

---

## Capítulo 4: Pipeline de Compilación

El pipeline completo es orquestado por `pkg/pipeline/pipeline.go`.

### 4.1 Etapas del Pipeline

```go
func CompileHIRSteps(hm *hir.Module) (Steps, error)
```

**Etapa 1 — Lowering HIR → MIR2** (`hir.LowerModule`):
- Variables con nombre → registros virtuales (registro nuevo por cada asignación)
- Flujo de control estructurado → bloques básicos con parámetros de bloque
- Mutaciones en bucles → parámetros de bloque en cabeceras de bucle (detectados vía `scanMutations`)
- ForEachStmt → bucle amigable con DJNZ con puntero+contador

**Etapa 2 — Optimizaciones por función** (en orden):
1. `EliminateDeadBlocks` — eliminar bloques inalcanzables
2. `ReorderBlocks` — mejorar fall-through
3. **Pipeline de constantes** (iterado hasta punto fijo):
   - `PropagateConstants` — rastrear constantes a través de movimientos
   - `FoldConstants` — evaluar operaciones puras en tiempo de compilación
   - `SimplifyIdentities` — `PtrAdd(x, 0)` → `Move(x)`, etc.
   - `ConstantCallElim` — plegar llamadas con args constantes vía VM
4. `DeadStoreElim` — eliminar instrucciones puras con resultados no usados (iterado)
5. `BranchEquiv` — eliminación de ramas basada en VM (demuestra guardas redundantes)
6. `CondRetSink` — elevar bloques else triviales a retornos condicionales

**Etapa 3 — Nivel de módulo: LUTGen**:
- Funciones puras con parámetros con rango → tablas de búsqueda en tiempo de compilación

**Etapa 4 — Verificación** (`Verify`):
- Etiquetas de bloque únicas, terminadores válidos, consistencia de tipos

**Etapa 5 — Optimización Interprocedimental**:
- `OptimizeContracts` — DP voraz sobre el grafo de llamadas para asignación de clases de registros
- `ApplyContracts` — reescribir firmas de funciones

**Etapa 6 — Asignación de Registros**:
- `ComputeLiveness` — flujo de datos hacia atrás hasta punto fijo
- `PBQPAllocate` — asignación ponderada de virtuales a registros físicos Z80

**Etapa 7 — Generación de Código**:
- `Z80Codegen` → texto ensamblador

### 4.2 Formatos de Emisión

```bash
mz source.nanz --emit=hir        # HIR dump
mz source.nanz --emit=mir2-raw   # MIR2 before optimization
mz source.nanz --emit=mir2       # MIR2 after optimization
mz source.plm  --emit=nanz       # PL/M → Nanz source translation
mz source.nanz -o output.a80     # Z80 assembly (default)
```

---

## Capítulo 5: Representación Intermedia MIR2

MIR2 es el IR central. Usa **argumentos de bloque** (estilo Cranelift/MLIR) en lugar de nodos phi.

### 5.1 Estructura del Módulo

```
Module
├── Funcs[] — cada una con Blocks[]
├── Globals[] — datos a nivel de módulo
├── Strings[] — pool de cadenas internadas
└── Structs[] — definiciones de tipos struct
```

### 5.2 Instrucciones

Cada instrucción: `%dst = Op %src0, %src1 : Ty [Class]`

**Aritméticas:** `OpAdd`, `OpSub`, `OpMul`, `OpDiv`, `OpSDiv`, `OpMod`
**Bit a bit:** `OpAnd`, `OpOr`, `OpXor`, `OpShl`, `OpShr`, `OpSar`
**Unarias:** `OpNeg`, `OpNot`
**Conversión:** `OpExt` (extensión con ceros), `OpSext` (extensión con signo), `OpTrunc`
**Comparación:** `OpCmp` con condiciones: `CmpEq`, `CmpNe`, `CmpLt`, `CmpLe`, `CmpGt`, `CmpGe`, `CmpUlt`, `CmpUle`, `CmpUgt`, `CmpUge`, `CmpSubCarry`
**Movimiento de datos:** `OpConst` (inmediato), `OpMove` (copia de registro)
**Memoria:** `OpLoad`, `OpStore`, `OpAddrOf`, `OpAlloca`, `OpField` (ptr+offset), `OpPtrAdd` (ptr+offset en runtime), `OpPtrBump` (ptr+stride en tiempo de compilación)
**Llamadas:** `OpCall` (directa), `OpCallIndirect` (a través de puntero)
**SMC:** `OpPatchSlot`, `OpLoadPatched`, `OpPatch` (primitivas de código auto-modificable)

### 5.3 Parámetros de Bloque

```
block @loop_head(%ptr: u16 [pointer], %cnt: u8 [counter]):
    %cond = cmp.gt %cnt, %limit : bool [flag]
    br_if %cond, @exit(), @body(%ptr, %cnt)
```

Los argumentos de bloque definen registros en la entrada. Los argumentos se pasan en cada arista (salto/ramificación). Esto reemplaza los nodos phi y hace las copias paralelas explícitas por arista.

### 5.4 Terminadores

| Terminador | Semántica | Emisión Z80 |
|------------|-----------|-------------|
| `TermJmp(target, args)` | Salto incondicional | `JP label` |
| `TermBrIf(cond, then, else)` | Ramificación condicional | `JP Z/NZ/C/NC label` |
| `TermDJNZ(counter, body, exit)` | Decrementar B, saltar si no es cero | `DJNZ rel8` |
| `TermCondRet(cond, vals, then)` | Retorno condicional | `RET CC` |
| `TermRet(vals)` | Retorno | `RET` |

`TermCondRet` es producido por la pasada de optimización CondRetSink — permite el retorno condicional de instrucción única del Z80.

---

## Capítulo 6: Pasadas de Optimización

### 6.1 Pipeline de Constantes (Punto Fijo)

Cuatro pasadas iteran hasta que no hay cambios:

1. **PropagateConstants** — si `%r = const 42`, reemplazar todos los usos de `%r` con 42.
2. **FoldConstants** — `const(3) + const(5)` → `const(8)`.
3. **SimplifyIdentities** — `PtrAdd(x, Const(0))` → `Move(x)`. Elimina aritmética redundante de puntero con offset cero (crítico para receptores `^Struct`).
4. **ConstantCallElim** — función pura con todos los args constantes → evaluar vía VM de MIR2.

### 6.2 Eliminación de Almacenamientos Muertos

Elimina instrucciones puras cuyos resultados nunca se usan. Se itera hasta punto fijo porque eliminar una instrucción muerta puede hacer que sus fuentes queden muertas.

**Nunca se eliminan:** `OpStore`, `OpCall`, `OpCallIndirect`, `OpAsm`, `OpPatch` (efectos secundarios).

### 6.3 BranchEquiv (Eliminación de Ramas Basada en VM)

Demuestra que las ramas condicionales son redundantes mediante pruebas exhaustivas con la VM.

**Ejemplo:** `abs_diff` con una guarda `if a == b { return 0 }`. BranchEquiv ejecuta todas las 256 entradas `(v, v)` a través de la función original y la parcheada. Ambas devuelven 0 → rama demostrablemente redundante → reemplazar `BrIf(eq, @zero, @diff)` con `Jmp(@diff)`.

**Correcto para u8:** prueba exhaustiva de 256 valores. Para tipos más anchos: muestreo heurístico (seguro para extender después).

### 6.4 CondRetSink

Encuentra `BrIf(cond, @then, @else)` donde `@else` es trivial (un solo predecesor, instrucciones puras, `TermRet`). Eleva las instrucciones de `@else` al bloque actual y reemplaza `BrIf` con `TermCondRet`.

**Optimizaciones fusionadas activadas inmediatamente después de la elevación:**
- **SubSwapNeg:** Si el `sub(a, b)` elevado tiene un `sub(b, a)` invertido en el bloque then → reemplazar con `neg(result)`. Ahorra `LD A,r; SUB r2` → un solo `NEG`.
- **HoistReorderSubBeforeCmp + CmpSubCarry:** Reordenar `sub` antes de `cmp_lt` sobre los mismos operandos → el flag de acarreo de `SUB` ES el resultado de la comparación. Elimina una instrucción `CP` por completo.

### 6.5 LUTGen (Nivel de Módulo)

Funciones puras con un único parámetro con rango → tabla de búsqueda.

**Elegibilidad:** 1 parámetro de `u8<lo..hi>` o `u16<lo..hi>`, rango ≤ 256, retorno único, sin llamadas extern, sin escrituras a globales.

**Proceso:**
1. Evaluar la función con la VM para cada entrada en el rango
2. Emitir tabla `DB` alineada a página como global
3. Reemplazar el cuerpo de la función con búsqueda en tabla:
   ```asm
   LD H, lut^H    ; 7T — page base (high byte only)
   LD L, C         ; 4T — index
   LD A, (HL)      ; 7T — lookup
   RET
   ```

**Resultado:** 18 T-states independientemente de la complejidad original de la función. Un bucle popcount de 8 iteraciones se convierte en 3 instrucciones.

### 6.6 Asignador de Registros PBQP

Reemplaza el antiguo asignador voraz. PBQP (Partitioned Boolean Quadratic Program) minimiza:

```
Σ nodeCost[r][loc(r)] + Σ edgeCost[interfering pairs]
```

**Coste de nodo:** `useCount[r] × costTable.Cost(r.Class, location)`. Los registros calientes pagan más por ubicaciones costosas.

**Reglas de reducción:**
- **R0:** grado 0 (aislado) → asignar la ubicación más barata inmediatamente.
- **R1:** grado 1 (hoja) → plegar coste de arista en el vecino, diferir.
- **RN:** grado ≥ 2 → voraz por **delta** (`2.°_mejor − mejor`). Los registros con delta grande (alta penalización si se desplazan) se asignan primero.

**Resultado para 4 registros ClassPointer simultáneos:**
```
p0 → HL (cost 0)
p1 → DE (cost 4)
p2 → BC (cost 6)
p3 → IX (cost 8)  — no $F0xx memory spill
```

### 6.7 Coalescencia de Copias Post-Asignación

Después de que PBQP asigna ubicaciones físicas, `coalesceAllocResult` elimina copias redundantes en los límites de bloques:

- Recopilar aristas de afinidad de `OpMove` y pares parámetro↔argumento de bloque.
- Recoloración en una pasada: si ningún vecino en el grafo de interferencia usa la ubicación del objetivo, recolorear para coincidir. Un bloqueo `recolored` previene ciclos de rotación en phi-webs de bucles.

### 6.8 Optimización Interprocedimental de Contratos

`OptimizeContracts` realiza DP voraz sobre el grafo de llamadas:

1. Ordenación topológica (hojas primero).
2. Para cada función: enumerar vectores candidatos de clases de registros.
3. Coste = coste interno de adaptador + coste de arista sobre todos los llamadores.
4. Elegir la asignación de coste mínimo.

Esto elige convenciones de llamada por función de forma global, reduciendo movimientos de registros en los puntos de llamada.

---

## Capítulo 7: Generación de Código Z80

`pkg/mir2/z80codegen.go` — convierte MIR2 asignado a texto ensamblador.

### 7.1 Emisión de Instrucciones

| Op MIR2 | Salida Z80 |
|---------|-----------|
| `OpConst` u8 | `LD r, imm8` |
| `OpConst` u16 | `LD rr, imm16` |
| `OpAdd` u8 | `ADD A, r` |
| `OpAdd` u16 | `ADD HL, rr` |
| `OpSub` u8 | `SUB r` |
| `OpSub` u16 | `AND A; SBC HL, rr` |
| `OpNeg` | `NEG` (solo 8 bits, registro A) |
| `OpCmp` | `CP r` (resultado en flags, ClassFlag) |
| `OpLoad` | `LD A, (HL)` / `LD A, (rr)` |
| `OpStore` | `LD (HL), r` |
| `OpCall` | `CALL label` |

### 7.2 Direccionamiento IX/IY

Cuando PBQP asigna un puntero a IX, el codegen usa direccionamiento con desplazamiento:

```asm
LD A, (IX+0)     ; 8-bit load, offset 0
LD (IX+1), A     ; 8-bit store, offset 1
LD L, (IX+0)     ; 16-bit load (lo)
LD H, (IX+1)     ; 16-bit load (hi)
```

**Copia de registro de 16 bits ↔ IX:** DE/BC→IX usa copia byte a byte no documentada:
```asm
LD IXH, D    ; DD 62 — 8T (D not substituted by DD prefix)
LD IXL, E    ; DD 6B — 8T (total 16T)
```

**La copia byte a byte HL→IX es INVÁLIDA:** El prefijo DD sustituye H→IXH y L→IXL tanto en la posición del operando fuente como destino, por lo que `LD IXH, H` se decodifica como `LD IXH, IXH` (NOP). Usar `PUSH HL; POP IX` (21T) en su lugar.

### 7.3 Optimizaciones Peephole en Codegen

**Supresión de constantes muertas:** `OpConst` cuyo único uso es `OpCmp` → emitir `CP imm8` directamente, omitir `LD r, imm`.

**Propagación de copias:** Rastrear `holdsPhys[A] = D` — si A ya contiene el valor de D, omitir `LD A, D`.

**Rastreo de último flags:** Si los flags ya están establecidos por una instrucción previa con los mismos operandos, suprimir `CP` redundante.

**LUT alineada a página:** Pre-escanear bloques para patrones de acceso a LUT → `LD H, sym^H; LD L, idx; LD A, (HL)` (18T).

**Direccionamiento directo de campos de struct global:** Pre-escanear para `AddrOf(global) + Field(offset) + Load/Store` → `LD A, (sym__field)` directamente (13T).

**Detección de cadena HL:** Múltiples almacenamientos consecutivos de campo al mismo struct → un solo `LD HL, sym` + cadena `LD (HL), r; INC HL` (ahorra recargar HL).

---

## Capítulo 8: Cadenas de Iteradores y Fusión

### 8.1 Los Cuatro Métodos de Iterador

| Método | Firma | Semántica |
|--------|-------|-----------|
| `forEach(lambda, n)` | Ejecutar lambda para n elementos | Terminal |
| `map(lambda)` | Transformar cada elemento | Intermedio |
| `filter(lambda)` | Mantener elementos donde lambda devuelve true | Intermedio |
| `mapInPlace(lambda, n)` | Transformar y escribir de vuelta | Terminal |

Estos se reconocen como UFCS sobre expresiones de puntero. El lowerer de HIR `tryLowerIterChain` fusiona las cadenas.

### 8.2 Fusión

```nanz
buf.map(|x| x * 2).filter(|x| x > 5).forEach(|x| { process(x) }, n)
```

Fusionado en un único `ForEachStmt`:

```
for x in buf[0..n]:
    let mapped = x * 2         // map body inlined
    if mapped > 5 {            // filter: skip if false
        process(mapped)        // forEach body inlined
    }
```

Esto se convierte en un único bucle DJNZ:
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

Sin array intermedio. Sin CALL a lambda. Tres etapas, un bucle.

### 8.3 Captura de Closures en Cadenas Fusionadas

```nanz
fun sum(buf: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    buf.forEach(|x: u8| { s = s + x }, n)
    return s
}
```

`s` es una variable libre en la lambda. El compilador:
1. Detecta la referencia libre mediante `hasFreeVars()`.
2. Omite el lowering independiente de `lambda_N`.
3. Propaga `s` como parámetro de bloque a través del bucle DJNZ.

Resultado: `s` vive en un registro (por ejemplo, C) durante todo el proceso — cero derrame, cero heap.

### 8.4 Coste por Elemento (sum_chain)

```asm
LD D, (HL)     ; 7T — load element
LD A, C        ; 4T
ADD A, D       ; 4T — s + x
LD C, A        ; 4T
INC HL         ; 6T
DJNZ .loop     ; 13T (taken)
; Total: 38T per element
```

Comparación: una llamada a función separada por elemento añade CALL(17T) + RET(10T) = 27T de overhead antes de cualquier trabajo.

### 8.5 Escritura de Vuelta con mapInPlace

```nanz
buf.mapInPlace(|x: u8| (x + 2), n)
```

El flag `MutateInPlace` activa una escritura de vuelta después del cuerpo de la lambda:

```asm
LD A, (HL)     ; load
ADD A, 2       ; transform
LD (HL), A     ; write back
INC HL
DJNZ .loop
```

### 8.6 Iteradores Rotos Conocidos

- **enumerate:** Conflicto del registro B — B es tanto el contador DJNZ como el índice de enumeración.
- **reduce:** Registro A sobreescrito entre dos parámetros SMC.

---

## Capítulo 9: Abstracciones de Coste Cero

Cada abstracción en Nanz compila a instrucciones Z80 directas. Aquí está la demostración.

### 9.1 UFCS — Coste Cero

`obj.method(args)` se desazucara a `method(obj, args)` en tiempo de parseo. La tabla de métodos es un `map[string]map[string]methodInfo` consultado solo durante el parseo.

**Coste: cero.** `CALL Acc_add` es idéntico a una llamada escrita a mano.

### 9.2 Interfaces — Coste Cero

```nanz
interface Animal { speak }
struct Dog {}
fun Dog.speak(self: Dog) -> u8 { return 1 }
```

Sin vtable, sin puntero gordo. En `g_dog.speak()`, el compilador resuelve Dog en tiempo de parseo y emite `CALL Dog_speak`.

**Coste: cero.** 17T para un `CALL` directo frente a ~55T para dispatch de interfaz estilo Go.

**Limitación:** El tipo concreto debe ser conocido estáticamente. El dispatch dinámico verdadero no está implementado (y violaría el principio de overhead cero).

### 9.3 Lambdas — Coste Cero

Cada `|x| expr` se convierte en `lambda_N` — una función regular. Cuando se usa en cadenas de iteradores, el cuerpo se inlinea. Cuando se usa como puntero a función, es un `CALL` estándar.

**Coste: cero asignaciones, cero struct de closure.** Overhead de CALL solo cuando no se inlinea.

### 9.4 Métodos de Struct — Coste Cero

`fun Vec2.add(self: Vec2, ...)` se almacena como `Vec2_add`. La decoración de nombres es el único "overhead" (en tiempo de compilación, no en tiempo de ejecución). El ensamblador generado es indistinguible de una función libre.

### 9.5 Sobrecarga de Operadores — Coste Cero

`a + b` sobre tipos struct despacha a `op_add(a, b)` — una llamada a función regular. Sin verificación de tipos en tiempo de ejecución.

---

## Capítulo 10: Oráculo de Corrección QBE

### 10.1 Qué es QBE

QBE (https://c9x.me/compile/) es un backend de compilador pequeño y rápido que convierte QBE IL (un formato SSA simple) a ensamblador nativo x86-64 o arm64. NO es un objetivo de Nanz — es una **herramienta de pruebas**.

### 10.2 El Pipeline del Oráculo de Corrección

```
source.nanz
    ├─→ HIR → MIR2 → Z80Codegen → MZE emulator → Result A
    └─→ HIR → MIR2 → QBE IL → qbe → cc → native binary → Result B

Si A ≠ B: El bug está en el codegen Z80 (semántica MIR2 demostrada correcta)
Si A = B: Ambos pipelines coinciden — corrección confirmada
```

El pipeline QBE se detiene ANTES de los pasos específicos de Z80 (optimización de contratos, asignación de registros). QBE hace su propia asignación de registros.

### 10.3 Tests E2E

`pkg/mir2qbe/e2e_test.go` contiene 7 tests E2E:

| Test | Qué verifica |
|------|-------------|
| `TestE2E_PLM_AbsDiff` | PL/M → HIR → MIR2 → QBE → nativo |
| `TestE2E_PLM_Fib` | Fibonacci con bucle |
| `TestE2E_Nanz_SumArray` | Aritmética de punteros, bucle `ptr[i]` |
| `TestE2E_Nanz_AbsDiff` | Flujo de control (if/else) |
| `TestE2E_Nanz_StructFields` | Acceso a campos de struct global |
| `TestE2E_Nanz_UFCS` | Dispatch de métodos sobre globales |
| `TestE2E_Nanz_Interface_ZeroCost` | Monomorfización de dispatch de interfaz |

Los tests se omiten automáticamente si `qbe` no está en el PATH (`exec.LookPath("qbe")`).

### 10.4 Traducción MIR2 → QBE

Mapeos clave (`pkg/mir2qbe/codegen.go`):
- Todos los tipos enteros (u8, u16, i8, i16, bool) → QBE `w` (word de 32 bits)
- `ptr` → QBE `l` (long de 64 bits, puntero nativo)
- Parámetros de bloque → nodos phi (QBE usa SSA con phi, no argumentos de bloque)
- Operaciones específicas de Z80 (SMC, push/pop, asm inline) → omitidas

### 10.5 Instalar QBE

Ver [Apéndice D](#appendix-d-installing-external-tools).

---

## Capítulo 11: PL/M-80: Corpus de Pruebas y Traducción Idiomática

### 11.1 El Rol de PL/M en la Creación del Ecosistema Nanz

PL/M-80 sirvió como el **corpus de arranque** para el backend MIR2. Los 26 archivos de Intel 80 Tools (ALGOL-M, BASIC-80, ensamblador de macros ML80, etc.) — todo código de producción real de los años 70 — proporcionaron 1.338 funciones y 11.661 sentencias para probar el pipeline HIR→MIR2→Z80 antes de que Nanz siquiera existiera.

El flujo de trabajo:

```
Paso 1: Parsear corpus PL/M (26/26 archivos, 100% de cobertura)
Paso 2: Bajar a HIR → verificar corrección
Paso 3: Bajar a MIR2 → verificar optimizaciones
Paso 4: Emitir Z80 → comparar con la salida de Intel PL/M-80 V4.0
Paso 5: --emit=nanz → generar código fuente Nanz desde HIR
Paso 6: Escribir Nanz idiomático a mano, guiado por la traducción mecánica
```

Esto significa que cada nodo HIR, cada optimización MIR2, y cada patrón de codegen Z80 fue primero validado con **código PL/M real** antes de que los programas Nanz lo usaran.

### 11.2 Traducción Mecánica: `mz program.plm --emit=nanz`

El flag `--emit=nanz` ejecuta `plm.Compile()` → HIR → `nanz.Print()`, produciendo código fuente Nanz sintácticamente válido. La traducción es estructural — preserva la lógica del programa PL/M exactamente.

**Código fuente PL/M:**
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

**Salida Nanz mecánica** (`mz sum.plm --emit=nanz`):
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

**Mapeo sintáctico:**

| PL/M-80 | Nanz | Notas |
|---------|------|-------|
| `PROCEDURE name(a,b) BYTE;` | `fun name(a: u8, b: u8) -> u8` | Tipos inline |
| `DECLARE X BYTE;` | `var x: u8` | Convención minúsculas |
| `DECLARE (A,B) WORD;` | `var a: u16; var b: u16` | Multi-declaración expandida |
| `DO WHILE cond; ... END;` | `while cond { ... }` | |
| `DO I = 0 TO N; ... END;` | `for i in 0..n { ... }` | Bucle contado |
| `DO CASE X; ... END;` | `switch x { case 0: ...; }` | |
| `IF cond THEN s1; ELSE s2;` | `if cond { ... } else { ... }` | |
| `ARR(I)` | `arr[i]` | Notación de índice |
| `CALL fn(a,b);` | `fn(a, b)` | |
| `DECLARE X LITERALLY 'Y'` | *(expandido antes del parseo)* | Macros eliminadas |

Ambos compilan a **ensamblador Z80 idéntico** — mismo HIR, mismo MIR2, mismo codegen.

### 11.3 De Mecánico a Idiomático: Tres Niveles

El mismo algoritmo de suma de array demuestra los tres niveles:

**Nivel 1 — Traducción mecánica de PL/M** (bucle indexado):
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

**Nivel 2 — Nanz idiomático** (escaneo secuencial):
```nanz
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    for x: u8 in ptr[0..n] {   // sequential: INC HL per element (6T)
        s = s + x
    }
    return s
}
```

**Nivel 3 — Cadena de iteradores con closure** (completamente fusionado):
```nanz
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    ptr.forEach(|x: u8| { s = s + x }, n)  // DJNZ loop, s in register
    return s
}
```

**Coste Z80 por elemento:**

| Nivel | Instrucción clave | T-states/elemento | Notas |
|-------|-------------------|-------------------|-------|
| 1 (indexado) | `ADD HL, DE` | ~64T | Calcular ptr+i en cada iteración |
| 2 (for-each) | `INC HL` | ~43T | Avance secuencial del puntero |
| 3 (forEach) | `INC HL` + `DJNZ` | ~38T | Fusionado, s en registro |

A 3,5 MHz con 100 elementos: Nivel 1 = 1,83ms, Nivel 3 = 1,09ms — **40% más rápido** con un cambio puramente sintáctico.

### 11.4 Patrones PL/M → Cadenas de Iteradores Nanz

La traducción más impactante: los bucles manuales DO WHILE de PL/M → cadenas de iteradores Nanz.

**PL/M: filtrar + procesar**
```plm
I = 0;
DO WHILE I < N;
    V = BUF(I) * 2;
    IF V > THRESHOLD THEN CALL PROCESS(V);
    I = I + 1;
END;
```

**Nanz mecánico:**
```nanz
var i: u8 = 0
while i < n {
    let v = buf[i] * 2
    if v > threshold { process(v) }
    i = i + 1
}
```

**Nanz idiomático (cadena fusionada):**
```nanz
buf.map(|x: u8| (x * 2))
   .filter(|x: u8| (x > threshold))
   .forEach(|x: u8| { process(x) }, n)
```

La versión con cadena: **un bucle DJNZ, cero arrays intermedios, todas las lambdas inlineadas.** Tres etapas fusionadas en ~6 instrucciones Z80 por elemento.

**PL/M: encontrar máximo**
```plm
MAX = 0;
I = 0;
DO WHILE I < N;
    IF BUF(I) > MAX THEN MAX = BUF(I);
    I = I + 1;
END;
```

**Nanz idiomático (forEach con captura):**
```nanz
var m: u8 = 0
buf.forEach(|x: u8| {
    if x > m { m = x }
}, n)
```

La variable capturada `m` se propaga como parámetro de bloque a través del bucle DJNZ — vive en un registro, nunca se derrama a memoria.

**PL/M: transformar en su lugar**
```plm
I = 0;
DO WHILE I < N;
    BUF(I) = BUF(I) + 2;
    I = I + 1;
END;
```

**Nanz idiomático:**
```nanz
buf.mapInPlace(|x: u8| (x + 2), n)
```

Un bucle: cargar, transformar, escribir de vuelta. El flag `MutateInPlace` activa la escritura de vuelta.

### 11.5 Lo Que PL/M No Puede Expresar

Estas funcionalidades de Nanz no tienen equivalente en PL/M:

| Funcionalidad Nanz | Equivalente PL/M | Por qué importa |
|---------------------|------------------|-----------------|
| Tipos con rango `u8<0..255>` | Ninguno | Habilita LUTGen (generación de tablas en tiempo de compilación) |
| Punteros tipados `^Struct` | BASED (sin tipo) | Resolución de campos, auto-deref, dispatch UFCS |
| `interface Animal { speak }` | Ninguno | Contrato en tiempo de compilación, dispatch de coste cero |
| `buf.map().filter().forEach()` | DO WHILE manual | Un solo bucle fusionado, sin arrays intermedios |
| Captura de closure `\|x\| { s = s + x }` | Ninguno | Variables de bucle como parámetros de bloque |
| Sobrecarga de operadores | Ninguno | `a + b` sobre tipos struct |

### 11.6 PL/M-80 V4.0 vs MIR2: Calidad de Código

Comparación real (del Informe #036) — el **mismo código fuente PL/M** compilado por el compilador original de Intel vs nuestro backend MIR2:

| Función | Intel PL/M-80 V4.0 | MIR2 Z80 | Ahorro |
|---------|-------------------|----------|--------|
| `abs_diff` | 33 bytes | 12 bytes | **−64%** |
| `fib` | 47 bytes | 31 bytes | **−34%** |
| **Total** | **80 bytes** | **43 bytes** | **−46%** |

El compilador de Intel almacena todos los parámetros y locales en memoria (convención de llamada 8080). La ABI registro-primero de MIR2 mantiene los valores en A/B/C/D/HL — cero tráfico de memoria en bucles calientes.

---

## Capítulo 12: Galería de Salida Compilada

Cada bloque de código a continuación es **salida real del compilador** de `mz <file>.nanz -o <file>.a80` en el build master actual (2026-03-10). Archivos fuente archivados en `reports/showcase-src/2026-03-10/`.

### 12.1 Acceso a Campos de Struct — Optimización de Cadena HL

**Fuente** (`ex1_struct.nanz`):
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

**Z80 compilado** (`ex1_struct.a80`):
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

**Optimización:** La detección de cadena HL fusiona tres almacenamientos de campo en una secuencia `LD HL` + `INC HL`: 53T vs 79T naíf (−33%).

### 12.2 Dispatch de Métodos UFCS — Cero Vtable

**Fuente** (`ex2_ufcs.nanz`):
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

**Z80 compilado** (`ex2_ufcs.a80`):
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

### 12.3 Dispatch de Interfaz de Coste Cero

**Fuente** (`ex3_iface.nanz`):
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

**Z80 compilado** (`ex3_iface.a80`):
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

17T para el dispatch (solo CALL). Dispatch de interfaz estilo Go: ~55T.

### 12.4 abs_diff — Cinco Pasadas de Optimización hasta el Mínimo

**Fuente** (`ex4a_abs_diff.nanz`):
```nanz
fun abs_diff(a: u8, b: u8) -> u8 {
    if a == b { return 0 }
    if a < b { return b - a }
    return a - b
}
```

**Z80 compilado** (`ex4a_abs_diff.a80`):
```z80
abs_diff:
    SUB D               ; A = a - b, carry set if a < b
    LD C, A             ; (regression: contract assigned b→D, result→C)
    RET NC              ; a >= b → return a-b
.abs_diff_if_then3:
    NEG                 ; A = -(a-b) = b-a
    RET
```

BranchEquiv eliminó la guarda `a == b`. CondRetSink elevó `sub` antes de `cmp`. CmpSubCarry eliminó `CP`. Resultado: 4 instrucciones centrales.

### 12.5 LUTGen — Bucle en Tiempo de Compilación → Búsqueda de 3 Instrucciones

**Fuente** (`ex5_lut.nanz`):
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

**Z80 compilado** (`ex5_lut.a80`):
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

**18T en total.** El bucle while nunca se ejecuta en tiempo de ejecución — fue evaluado en tiempo de compilación por la VM de MIR2 para las 256 entradas.

### 12.6 forEach con Captura de Closure — Bucle DJNZ

**Fuente** (`ex6_foreach.nanz`):
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

**Z80 compilado** (`ex6_foreach.a80`):
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

**sum_chain: 38T/elemento.** Sin CALL a lambda. La variable capturada `s` vive en C durante todo el proceso.

### 12.7 mapInPlace — Transformación En Su Lugar con Escritura de Vuelta

**Fuente** (`ex7_mapinplace.nanz`):
```nanz
fun add2_inplace(buf: ^u8, n: u8) -> void {
    buf.mapInPlace(|x: u8| (x + 2), n)
}
```

**Z80 compilado** (`ex7_mapinplace.a80`):
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

**Regresión conocida:** `LD D, 2` es código muerto, `INC C; INC C` reemplaza a `ADD A, 2` (51T vs 40T/elemento). La `lambda_0` independiente también es código muerto.

### 12.8 GCD — Bucle con Dos Variables Mutables

**Fuente** (`ex8_gcd.nanz`):
```nanz
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else     { b = b - a }
    }
    return a
}
```

**Z80 compilado** (`ex8_gcd.a80`):
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

El coalescente de fase 6c eliminó todo el overhead de copia en los límites de bloque del bucle caliente. Solo el trampolín de salida (camino frío) tiene intercambios de registros.

---

## Capítulo 13: Relación con MinZ y PL/M

### 11.1 Los Tres Frontends

| Lenguaje | Extensión | Parser | Backend | Estado |
|----------|-----------|--------|---------|--------|
| MinZ | `.minz` | `pkg/parser/participle/` | MIR1 → codegen antiguo | Congelado |
| Nanz | `.nanz` | `pkg/nanz/parse.go` | HIR → MIR2 → Z80 | **Activo** |
| PL/M-80 | `.plm` | `pkg/plm/` | HIR → MIR2 → Z80 | Activo |

El enrutador en `cmd/minzc/main.go`:
```go
if ext == ".plm" || ext == ".nanz" {
    return compileViaHIR(src, ext)
} else {
    // old MIR1 path for .minz
}
```

### 11.2 Cuándo Usar Cada Uno

**Nanz** — programas nuevos para Z80/CP/M, sintaxis moderna, LUTGen, asignación PBQP.
**MinZ** — programas `.minz` existentes que funcionan. Tiene metafunciones (`@define`, `@print`) que aún no están en Nanz.
**PL/M-80** — portado de software legacy de CP/M. El 100% del corpus de Intel 80 Tools se parsea.

### 11.3 Diferencias de Funcionalidades (Nanz vs. MinZ)

Nanz actualmente carece de:
- Macros de preprocesador `@define`
- Salida optimizada de cadenas `@print`
- Interpolación de cadenas estilo Ruby (`"Hello #{name}"`)
- Sintaxis de valores de retorno múltiple (MIR2 lo soporta, el parser no)
- Sobrecarga de funciones (MinZ la tiene; Nanz requiere nombres distintos)
- `@extern` con anotaciones de clase de registro (documentado pero aún no parseado)

### 11.4 Otros Backends (Solo MinZ)

El pipeline antiguo de MinZ (flag `-b`) soporta múltiples objetivos. Estos NO están disponibles para Nanz (que siempre va por MIR2 → Z80):

| Backend | Flag | Estado | Salida |
|---------|------|--------|--------|
| Z80 | `-b z80` | Producción | `.a80` |
| 6502 | `-b 6502` | Beta | `.s` |
| 68000 | `-b m68k` | Alpha | `.s` |
| i8080 | `-b i8080` | Beta | `.s` |
| Game Boy | `-b gb` | Activo | `.s` |
| WASM | `-b wasm` | Alpha | `.wat` |
| C | `-b c` | Beta | `.c` |
| Crystal | `-b crystal` | Beta | `.cr` |
| LLVM | `-b llvm` | Planificado | `.ll` |

Estos usan el IR antiguo (MIR1), no MIR2. El plan a largo plazo es retirar MIR1 y enrutar todos los frontends a través de HIR → MIR2.

---

## Apéndice A: Gramática Completa de la Sintaxis

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

**Notas léxicas:**
- Comentarios: `//` (de línea) y `/* */` (de bloque)
- Los espacios en blanco no son significativos
- Enteros: decimal o hexadecimal `0x`/`0X`
- Cadenas: entre comillas dobles, sin secuencias de escape

---

## Apéndice B: Clases de Registros y Tabla de Costes

### Registros Físicos Z80

| Registro | Clase | Coste | Notas |
|----------|-------|-------|-------|
| A | ClassAcc | 0T | Acumulador ALU, valor de retorno, 1er parámetro u8 |
| B | ClassCounter | 0T | Contador DJNZ, 3er parámetro |
| C, D, E, H, L | ClassGeneral | 0T | 8 bits de propósito general |
| HL | ClassPointer | 0T | Puntero primario, 1er parámetro u16/ptr, retorno |
| DE | ClassIndex | 0T | 2.° parámetro u16, fuente LDIR |
| BC | ClassPair | 0T | 3er parámetro u16 |
| IX | ClassIX | 8T | Puntero de desbordamiento (+4T prefijo DD por acceso) |
| IY | ClassIY | 8T | Raramente usado (reservado por el sistema en algunas plataformas) |
| IXH/IXL | ClassIXY8 | 8T | Mitades de 8 bits no documentadas |
| $F0xx | ClassMem | 26T | "Extensión de archivo de registros" respaldada por memoria |

### Jerarquía de Clases de Registros

| Nivel | Clases | Coste | Mecanismo |
|-------|--------|-------|-----------|
| 0 — Primario | Acc, Counter, General, Pointer, Index, Pair, Flag | 0T | Uso directo |
| 1 — IX/IY | IX, IY, IXY8 | 4-8T | Prefijo DD/FD |
| 2 — Sombra | Shadow, AccShadow | 8T | EXX / EX AF,AF' |
| 3 — Pila | Stack | 21T | PUSH + POP |
| 4 — Memoria | Mem | 26T | LD (addr) / LD addr |

ClassFlag es especial: representa los flags de la CPU Z80 (Z, CY, etc.) y cuesta 0T. Los resultados booleanos de comparaciones se mantienen en flags sin materializar a un registro.

---

## Apéndice C: Referencia de CLI

### Compilando Programas Nanz

```bash
mz source.nanz -o output.a80              # compile to Z80 assembly
mz source.nanz -o output.a80 --target=cpm # target CP/M
mz source.nanz -o output.a80 --target=spectrum  # target ZX Spectrum
mz source.nanz -o output.a80 --target=agon      # target Agon Light 2
```

### Representaciones Intermedias

```bash
mz source.nanz --emit=hir        # HIR dump
mz source.nanz --emit=mir2-raw   # MIR2 before optimization
mz source.nanz --emit=mir2       # MIR2 after optimization
mz source.plm  --emit=nanz       # PL/M → Nanz translation
```

### Flags de Optimización

```bash
mz source.nanz --disable-optimize      # disable all optimizations
mz source.nanz --disable-ir-opt        # disable MIR-level opts
mz source.nanz --disable-asm-opt       # disable peephole
mz source.nanz --disable-smc           # disable self-modifying code
mz source.nanz --compile-trace         # show all optimization steps
```

### Ejecutando Tests

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

### Programas de Ejemplo

| Archivo | Descripción |
|---------|-------------|
| `examples/nanz/01_sum_array.nanz` | Bucle while con `ptr[i]` |
| `examples/nanz/02_sum_array_idiomatic.nanz` | For-each e iterador forEach |
| `examples/nanz/03_filter_map_chain.nanz` | Cadena completa map/filter/forEach |
| `examples/nanz/04_lut_popcount.nanz` | Generación de LUT vía tipo con rango |
| `examples/nanz/05_four_pointers.nanz` | PBQP: 4 registros ClassPointer |
| `examples/nanz/06_pbqp_weighted.nanz` | Asignación con coste ponderado |
| `examples/nanz/07_ix_load_store.nanz` | Direccionamiento de desbordamiento IX |

---

## Apéndice D: Instalación de Herramientas Externas

### QBE (Oráculo de Corrección)

QBE solo es necesario para ejecutar tests de corrección E2E (`pkg/mir2qbe/`). NO es necesario para la compilación normal de Nanz.

**Linux (compilar desde fuente):**
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

**Verificar:**
```bash
qbe --version        # should print version
echo 'export function w $main() { @start ret 0 }' | qbe
```

**Compilador C** también es necesario (cualquier compilador C99: `gcc`, `clang`). Normalmente preinstalado.

Si `qbe` no está en el PATH, los tests E2E se omiten automáticamente con `t.Skip("qbe not in PATH")`.

### Ensamblador MZA (Integrado)

No se necesita instalación externa. MZA es parte del toolchain MinZ:

```bash
cd minzc && make mza
# or: make install-user (installs all tools to ~/.local/bin/)
```

### Emulador MZE (Integrado)

```bash
cd minzc && make mze
```

Se usa para: ejecutar binarios Z80 compilados, evaluación de constantes dentro del compilador (LUTGen, BranchEquiv).

---

## Apéndice E: Errores Conocidos y Limitaciones

### Parser

| Problema | Estado | Solución alternativa |
|----------|--------|---------------------|
| `@extern` con anotaciones `params=`/`returns=` | No implementado | Usar `@extern fun` básico (asignación automática de registros) |
| Sobrecarga de funciones | No implementado | Usar nombres distintos (`abs_diff`, `abs_diff_u16`) |
| Valores de retorno múltiple | No parseado | MIR2 lo soporta; el parser no |
| Secuencias de escape en cadenas | No implementado | Sin `\n`, `\t`, etc. |
| Bloques de ensamblador inline `@asm` | No implementado | Usar `@extern` con wrappers en asm |

### Codegen

| Problema | Estado | Detalles |
|----------|--------|----------|
| `applySubSwapNeg` en u16 | Bug | Fuerza ClassAcc (8 bits) en resultado NEG de 16 bits. Falta guarda `Ty.Width() <= 8`. |
| Globales de struct de tamaño cero | Bug | `struct Dog {}` no emite datos; símbolo indefinido en tiempo de enlace. Solución: emitir `Dog: EQU $`. |
| Sentinel `LD A, mem` | Bug | Fallo de asignación de registros emite literal "mem" en ensamblador. |
| Caller-save para 2.° arg | Bug | El segundo argumento en llamadas repetidas puede ser sobrescrito. |
| mapInPlace constant-add | Regresión | `ADD A, imm` no se activa cuando el elemento está en C (no en A). |

### Cadenas de Iteradores

| Problema | Estado | Detalles |
|----------|--------|----------|
| `enumerate` | Roto en Z80 | B = contador entra en conflicto con B = índice de enumeración |
| `reduce` | Roto en Z80 | A sobrescrito entre dos parámetros SMC |
| Captura de closure no fusionada | Comportamiento indefinido | Las lambdas pasadas como punteros a función no pueden capturar variables locales externas |

### Asignador de Registros

| Problema | Estado | Detalles |
|----------|--------|----------|
| Registros respaldados por memoria | Rendimiento | ~207T reales vs ~43T ideales por elemento de iterador cuando los registros se derraman a $F0xx |
| Deriva del optimizador de contratos | Conocido | Puede asignar clases subóptimas (por ejemplo, `b → D` en lugar de `b → B` para abs_diff) |
| Costes de arista PBQP | Diferido | Decisiones de asignación correlacionadas (BC★ para LUT) aún no modeladas |

---

*Nanz: Sintaxis moderna, abstracciones de coste cero, rendimiento Z80.*

*Pipeline: `.nanz` → `nanz.Parse()` → `*hir.Module` → `hir.LowerModule()` → `*mir2.Module` → optimización → asignación → Z80Codegen → `.a80`*
