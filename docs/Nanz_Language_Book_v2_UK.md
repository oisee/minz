# Мова Nanz: Повний посібник (v2)

> Nanz — це типізована системна мова для Z80 та ретро-платформ.
> Вона компілюється через HIR → MIR2 → Z80 assembly — чистий, сучасний конвеєр,
> спільний з фронтендом PL/M-80.

**Версія:** MinZ compiler v0.19.5 (конвеєр MIR2)
**Дата:** 2026-03-10
**Статус:** Активна розробка — основні можливості стабільні, деякі місця ще потребують доопрацювання
**Джерело істини:** `pkg/nanz/parse.go`, `pkg/hir/`, `pkg/mir2/`, `pkg/pipeline/`

---

## Зміст

1. [Що таке Nanz?](#chapter-1-what-is-nanz)
2. [Довідник синтаксису](#chapter-2-syntax-reference)
3. [Система типів](#chapter-3-type-system)
4. [Конвеєр компіляції](#chapter-4-compilation-pipeline)
5. [Проміжне представлення MIR2](#chapter-5-mir2-intermediate-representation)
6. [Проходи оптимізації](#chapter-6-optimization-passes)
7. [Генерація коду Z80](#chapter-7-z80-code-generation)
8. [Ланцюжки ітераторів та злиття](#chapter-8-iterator-chains-and-fusion)
9. [Абстракції з нульовою вартістю](#chapter-9-zero-cost-abstractions)
10. [QBE — оракул коректності](#chapter-10-qbe-correctness-oracle)
11. [PL/M-80: засівання корпусу та ідіоматичний переклад](#chapter-11-plm-80-corpus-seeding-and-idiomatic-translation)
12. [Галерея скомпільованого виводу](#chapter-12-compiled-output-gallery)
13. [Зв'язок із MinZ та PL/M](#chapter-13-relation-to-minz-and-plm)
- [Додаток A: Повна граматика синтаксису](#appendix-a-complete-syntax-grammar)
- [Додаток B: Класи регістрів і таблиця вартості](#appendix-b-register-classes-and-cost-table)
- [Додаток C: Довідник CLI](#appendix-c-cli-reference)
- [Додаток D: Встановлення зовнішніх інструментів](#appendix-d-installing-external-tools)
- [Додаток E: Відомі помилки та обмеження](#appendix-e-known-bugs-and-limitations)

---

<a id="chapter-1-what-is-nanz"></a>

## Розділ 1: Що таке Nanz?

Nanz (розширення `.nanz`) — це активний фронтенд-мова компілятора MinZ. Вона статично типізована, імперативна й розрахована на дві аудиторії:

1. Розробники, які пишуть програми для Z80 / ретро-платформ і хочуть сучасний синтаксис із абстракціями нульової вартості.
2. Команда компілятора MinZ, що розробляє та тестує бекенд MIR2.

### 1.1 Конвеєр компіляції

```
source.nanz
    │  nanz.Parse()
    ▼
*hir.Module          ← High-level IR (structured control flow, named variables)
    │  hir.LowerModule()
    ▼
*mir2.Module         ← Mid-level IR (SSA-like, virtual registers, typed ops)
    │  optimization passes (constant fold, DSE, BranchEquiv, CondRetSink, LUTGen)
    ▼
*mir2.Module         ← optimized
    │  OptimizeContracts() → PBQPAllocate()
    ▼
*mir2.Module         ← allocated (virtual → physical registers)
    │  Z80Codegen()
    ▼
output.a80           ← MZA-compatible Z80 assembly
    │  mza (assembler)
    ▼
output.bin / .tap    ← binary / ZX Spectrum tape image
```

Виклик:

```bash
mz source.nanz -o output.a80
```

### 1.2 Nanz проти MinZ

MinZ (`.minz`) — це оригінальний фронтенд зі своїм парсером і генератором коду, що націлений на старіший MIR1 IR. Цей конвеєр **заморожений** — він працює, але не розвивається.

Nanz націлений на MIR2, який забезпечує:

- SSA-подібний потік даних з параметрами блоків (у стилі Cranelift/MLIR, без phi-вузлів)
- Розподілювач регістрів PBQP зі зваженими векторами вартості
- Генерацію LUT: обчислення під час компіляції для чистих функцій з обмеженим діапазоном
- Міжпроцедурну оптимізацію угод про виклики
- Емулятор Z80, що використовується як обчислювач констант і засіб доведення еквівалентності гілок

**Правило:** Пишіть нові ретро-програми на Nanz. Існуючі `.minz`-програми залишайте як є.

### 1.3 Nanz проти PL/M-80

PL/M-80 (`.plm`) — це мова Intel 1970-х років для CP/M. Компілятор MinZ включає парсер PL/M-80, який компілює через **той самий конвеєр HIR → MIR2 → Z80**, що й Nanz:

```bash
mz legacy.plm -o legacy.a80     # same backend as Nanz
mz legacy.plm --emit=nanz       # translate PL/M to Nanz source
```

### 1.4 Філософія проєктування

Nanz навмисно мінімалістична. Граматика вміщується на одному екрані. Ні збирача сміття, ні середовища виконання, ні динамічної диспетчеризації. Кожна абстракція — лямбди, ітератори, методи структур, інтерфейси — компілюється в прямі інструкції Z80 без жодних додаткових витрат понад те, що ви написали б вручну.

---

<a id="chapter-2-syntax-reference"></a>

## Розділ 2: Довідник синтаксису

Парсер — це рукописний рекурсивно-низхідний парсер з Pratt-парсером виразів (`pkg/nanz/parse.go`). Вихідний код: ~2000 рядків Go.

### 2.1 Структура модуля

Вихідний файл Nanz — це модуль: послідовність оголошень верхнього рівня в довільному порядку.

```nanz
// line comment
/* block comment */

struct Point { x: u8, y: u8 }
interface Drawable { draw }
global counter: u8
fun add(a: u8, b: u8) -> u8 { return a + b }
```

Імпортів немає; компонування виконується на рівні асемблера.

### 2.2 Типи

| Синтаксис | Ширина | Відображення Z80 | Примітки |
|--------|-------|-------------|-------|
| `u8` | 8 біт | A/B/C/D/E/H/L | беззнаковий байт |
| `u16` | 16 біт | HL/DE/BC | беззнакове слово |
| `i8` | 8 біт | ті ж регістри, інша арифметика | знаковий байт |
| `i16` | 16 біт | те ж | знакове слово |
| `bool` | 8 біт | false=0, true=1 | |
| `void` | — | лише тип повернення | |
| `ptr` | 16 біт | HL/DE/BC | нетипізований вказівник |
| `^T` | 16 біт | HL/DE/BC | типізований вказівник на T |
| `[T; N]` | N×width(T) | — | масив фіксованого розміру |
| `u8<lo..hi>` | 8 біт | — | тип з діапазоном (кандидат на LUT) |
| `u16<lo..hi>` | 16 біт | — | тип з діапазоном |
| `StructName` | сума полів | передається за вказівником | тип структури |
| `InterfaceName` | — | визначається під час компіляції | інтерфейс як тип параметра |

**Типи вказівників:** `^T` зберігає тип елемента для розв'язання полів. Коли T — це структура, `^Struct` дозволяє типізований доступ до полів через вказівник (наприклад, `self.val` на отримувачі `^Acc` автоматично розіменовує і визначає зміщення полів). Це НЕ просто синтаксичний цукор — парсер використовує `varPtrElem[paramName]` для відстеження структури, на яку вказує вказівник, для розв'язання полів і диспетчеризації UFCS.

**Типи з діапазоном:** `u8<0..255>` оголошує вхід з діапазоном. Діапазон у вихідному коді включний (`0..255`), внутрішньо зберігається як виключний (`[0, 256)`). Функції з одним параметром діапазонного типу та чистим тілом є кандидатами на генерацію LUT (див. §6.4).

### 2.3 Функції

```nanz
fun name(param1: Type1, param2: Type2) -> ReturnType {
    // body
}
```

`fn` приймається як псевдонім для `fun`. Функції без значення повернення опускають `-> ReturnType` (неявно `void`).

```nanz
fn add(a: u8, b: u8) -> u8 { return a + b }

fun clear(buf: ^u8, n: u8) {
    var i: u8 = 0
    while i < n { buf[i] = 0; i = i + 1 }
}
```

### 2.4 Зовнішні функції

Функції, реалізовані поза модулем Nanz, оголошуються з `@extern`:

```nanz
@extern fun process(x: u8) -> void
@extern fun rom_print(s: ptr) -> void
```

Тіло опущено. Компілятор призначає класи регістрів параметрам згідно зі стандартною угодою про виклики.

**Статус:** Анотована форма `@extern("sym", params=[z80_a], returns=[z80_a])`, описана в деякій документації, **ще не реалізована** в парсері. Наразі `@extern`-функції отримують лише автоматичне призначення регістрів.

### 2.5 Оголошення змінних

**`var` — явний тип, необов'язковий ініціалізатор:**

```nanz
var i: u8 = 0
var buf: ^u8
var port: u8 at(0xFE)          // memory-mapped at absolute address
```

**`let` — тип виводиться з ініціалізатора:**

```nanz
let x = 42                      // x: u8
let ptr = @ptr(u8, 0xFE)       // ptr: ptr
let y: u16 = 1000              // explicit type override
```

Речення `at(addr)` відображає змінну на фіксовану адресу пам'яті. Читання та записи стають `LD (addr), r` / `LD r, (addr)`.

### 2.6 Глобальні змінні

```nanz
global counter: u8
global vram: u8 at(0x4000)            // memory-mapped
global table: [u8; 4] = [1, 2, 4, 8]  // initialized array
global palette: Color                  // struct global
```

Глобальні змінні розміщуються в секції даних виводу `.a80`. Глобальні масиви з ініціалізаторами отримують директиви `DB`.

### 2.7 Літерали

```nanz
42          // decimal — u8 if ≤255, u16 otherwise
0xFF        // hexadecimal
0           // zero (u8)
true        // bool
false       // bool
"hello"     // string → ptr to NUL-terminated bytes (interned)
```

### 2.8 Оператори

**Арифметичні** (ліво-асоціативні):
`+` `-` `*` `/` `%`

**Побітові:**
`&` `|` `^` (XOR) `~` (NOT, унарний) `<<` `>>`

**Порівняння** (повертають `bool`):
`==` `!=` `<` `<=` `>` `>=`

**Логічні:**
`!` (NOT, унарний)

**Пріоритет** (від найвищого до найнижчого):

| Рівень | Оператори |
|-------|-----------|
| 8 | `*` `/` `%` |
| 7 | `+` `-` |
| 6 | `<<` `>>` |
| 5 | `<` `<=` `>` `>=` |
| 4 | `==` `!=` |
| 3 | `&` |
| 2 | `^` (XOR) |
| 1 | `\|` |

Дужки змінюють пріоритет: `(a + b) * c`.

### 2.9 Операції з вказівниками

```nanz
let val = ptr^            // dereference (load byte at ptr)
^ptr = 42                 // store through pointer
let b = buf[3]            // index load (buf + 3)
buf[i] = 0               // index store
let p = &counter          // address-of global
let kb = @ptr(u8, 0xFE)  // typed constant pointer to absolute address
```

`@ptr(T, addr)` — це ідіоматичний спосіб звернення до апаратних регістрів і ROM-підпрограм.

### 2.10 Приведення типів

```nanz
u8(expr)     // truncate to u8
u16(expr)    // zero-extend to u16
i8(expr)     // reinterpret as signed 8-bit
i16(expr)    // reinterpret as signed 16-bit
```

Приведення типів — явні, неявного розширення немає. На Z80 `u8→u16` коштує `LD H, 0 / LD L, A`; компілятор це не приховує.

### 2.11 Керування потоком

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

`for x: u8 in ptr[start..end]` компілюється в цикл DJNZ з `HL` як вказівником і `B` як лічильником — найтісніший цикл Z80.

Гілки switch не «провалюються». Тіло кожного case завершується на наступному `case`, `default` або `}`.

### 2.12 Структури

```nanz
struct Point { x: u8, y: u8 }
struct Vec3d { x: u16, y: u16, z: u8 }
```

Поля розташовуються послідовно, без вирівнювання. Зміщення обчислюються під час парсингу:
- `Point.x` → зміщення 0, `Point.y` → зміщення 1
- `Vec3d.y` → зміщення 2, `Vec3d.z` → зміщення 4

Значення структур завжди передаються **за вказівником** (HL на Z80). Доступ до полів: `Load(ptr + offset)`.

### 2.13 Методи структур і UFCS

Методи оголошуються з синтаксисом `TypeName.methodName`:

```nanz
struct Acc { val: u8 }

fun Acc.add(self: ^Acc, amount: u8) -> u8 {
    self.val = self.val + amount
    return self.val
}
```

Компілятор зберігає це як `Acc_add`. На місцях виклику UFCS переписує:

```nanz
acc_g.add(5)    // → Acc_add(&acc_g, 5) — direct CALL, no vtable
```

**Як працює розв'язання UFCS** (під час парсингу):
1. Парсер бачить `base.method(args)`.
2. Шукає `exprTy(base)` — якщо це структура, перевіряє `methodTable[structName][method]`.
3. Якщо base — це вказівник `^Struct`, перевіряє `varPtrElem[name]` на тип структури.
4. Якщо base — це змінна з типом інтерфейсу, викликає `findImplementors()`.
5. Переписує на `CallExpr{Fn: "StructName_method", Args: [base, args...]}`.

**Отримувачі-вказівники** (`self: ^Acc`):
- `self.val` автоматично розіменовує — визначає зміщення поля через `varPtrElem`.
- `self^.val` також працює (явне розіменування, той самий результат).
- Вказівник подорожує в HL (ClassPointer). Читання полів: `LD reg, (HL)` зі зміщенням 0, `INC HL; LD reg, (HL)` зі зміщенням 1 тощо.

**Отримувачі-значення** (`self: Acc`):
- Структура передається за вказівником — на рівні ABI те саме, що й `^Acc`.
- Парсер записує в `methodTable` з повною інформацією про тип повернення.

### 2.14 Перевантаження операторів

```nanz
fun +(a: Vec2, b: Vec2) -> Vec2 { return a }
fun -(a: Vec2, b: Vec2) -> Vec2 { return a }

fun compute(a: Vec2, b: Vec2) -> Vec2 {
    return a + b    // dispatched to op_add(a, b) because a is Vec2
}
```

Оператори, які можна перевантажити: `+` `-` `*` `/` `%` `==` `!=` `<` `<=` `>` `>=` `&` `|` `^`.

Манглені імена: `op_add`, `op_sub`, `op_mul`, `op_div`, `op_rem`, `op_eq`, `op_ne`, `op_lt`, `op_le`, `op_gt`, `op_ge`, `op_and`, `op_or`, `op_xor`.

**Важливо:** Для примітивних типів (`u8 + u8`) завжди використовується вбудований `BinExpr`, незалежно від перевантажень в області видимості. Перевантаження спрацьовують тільки коли лівий операнд має тип структури.

### 2.15 Інтерфейси

```nanz
interface Animal {
    speak
    move
}
```

Інтерфейси — це **контракти часу компіляції**. Ніякої vtable, ніякого fat pointer, ніякої таблиці диспетчеризації методів.

**Як тип параметра:**

```nanz
fun feed(a: Animal) -> u8 {
    return a.speak()      // monomorphized at compile time
}
```

**Правила розв'язання:**
- **Один реалізатор** → мономорфізація: `a.speak()` → `Dog_speak(a)`.
- **Кілька реалізаторів** → помилка компіляції: `"ambiguous dispatch: ... multiple types implement Animal: [Dog Cat]; use concrete type"`.
- **Нуль реалізаторів** → помилка компіляції.

**Інтерфейс як тип глобальної змінної:**

```nanz
global g_thing: Drawable
```

Працює так само: диспетчеризація UFCS визначає єдиного реалізатора.

### 2.16 Лямбди

```nanz
let double = |x: u8| x * 2          // expression body
let add = |a: u8, b: u8| a + b      // multi-param
let greet = |x: u8| { return x + 1 }  // block body
let inc = |x| x + 1                 // type defaults to u8
```

Кожна лямбда стає функцією верхнього рівня `lambda_N` (послідовний лічильник). Посилання на місці виклику — це `VarRefExpr{Name: "lambda_N"}`.

**Правила захоплення:**
- **Злиті лямбди ітераторів** (forEach/map/filter): можуть захоплювати та змінювати зовнішні локальні змінні. Компілятор виявляє вільні змінні через `hasFreeVars()`, пропускає окреме зниження і пропускає захоплені змінні як параметри блоків через цикл DJNZ. Нуль купи, нуль spill-ів.
- **Незлиті лямбди** (вказівники на функції): не можуть захоплювати зовнішні локальні змінні — немає фрейму виконання. Доступні лише глобальні змінні.

### 2.17 Методи ітераторів (UFCS на вказівниках)

```nanz
buf.forEach(|x: u8| { process(x) }, n)       // execute for each element
buf.map(|x: u8| (x * 2))                     // transform elements
buf.filter(|x: u8| (x > 5))                  // keep matching elements
buf.mapInPlace(|x: u8| (x + 2), n)           // in-place transform
```

Вони розпізнаються парсером як UFCS-виклики методів на виразах-вказівниках. `tryLowerIterChain` знижувача HIR зливає ланцюжки в єдиний цикл DJNZ. Див. Розділ 8.

---

<a id="chapter-3-type-system"></a>

## Розділ 3: Система типів

### 3.1 Типи MIR2

IR MIR2 (`pkg/mir2/types.go`) підтримує:

| Тип | Ширина | Представлення в Go |
|------|-------|-------------------|
| `TyVoid` | 0 | `&IntTy{Bits: 0}` |
| `TyBool` | 8 | `&IntTy{Bits: 8}` |
| `TyU8` | 8 | `&IntTy{Bits: 8, Signed: false}` |
| `TyI8` | 8 | `&IntTy{Bits: 8, Signed: true}` |
| `TyU16` | 16 | `&IntTy{Bits: 16, Signed: false}` |
| `TyI16` | 16 | `&IntTy{Bits: 16, Signed: true}` |
| `TyU24` | 24 | `&IntTy{Bits: 24}` (майбутнє eZ80) |
| `TyPtr` | 16 | `&PtrTy{}` |
| `StructTy` | сума полів | `&StructTy{Name, Fields}` |
| `ArrayTy` | N×elem | `&ArrayTy{Len, Elem, Layout}` |
| `TupleTy` | сума елементів | `&TupleTy{Elems}` (множинне повернення) |

Крім того, типи з діапазоном обертають базовий тип межами `[Lo, Hi)`:
```go
type RangedTy struct {
    Base Ty
    Lo, Hi int  // [Lo, Hi) — exclusive upper bound
}
```

### 3.2 Призначення класів регістрів

Параметрам призначаються класи регістрів на основі позиції та типу (`classForParam` у `hir/lower.go`):

| Позиція | Тип | Клас | Фізичний Z80 |
|----------|------|-------|-------------|
| 1-й | `u8`, `bool` | `ClassAcc` | A |
| 1-й | `u16`, `ptr`, struct | `ClassPointer` | HL |
| 2-й | `u8` | `ClassGeneral` | C |
| 2-й | `u16` | `ClassIndex` | DE |
| 3-й | `u8` | `ClassCounter` | B |
| 3-й+ | `u16` | `ClassPair` | BC/DE |

Значення повернення: `u8` → `ClassAcc` (A), `u16`/`ptr` → `ClassPointer` (HL).

### 3.3 Розташування структур

Поля упаковуються послідовно, без вирівнювання:

```nanz
struct Color { r: u8, g: u8, b: u8 }  // total: 3 bytes
// r at offset 0, g at offset 1, b at offset 2
```

Парсер обчислює зміщення під час парсингу, підсумовуючи `Ty.Width() / 8` для кожного попереднього поля. Поля глобальних структур отримують EQU-мітки в асемблері: `palette__r EQU palette + 0`.

---

<a id="chapter-4-compilation-pipeline"></a>

## Розділ 4: Конвеєр компіляції

Повний конвеєр оркеструється `pkg/pipeline/pipeline.go`.

### 4.1 Етапи конвеєра

```go
func CompileHIRSteps(hm *hir.Module) (Steps, error)
```

**Етап 1 — Зниження HIR → MIR2** (`hir.LowerModule`):
- Іменовані змінні → віртуальні регістри (свіжий регістр на кожне присвоєння)
- Структуроване керування потоком → базові блоки з параметрами блоків
- Мутації циклів → параметри блоків на заголовках циклів (виявлені через `scanMutations`)
- ForEachStmt → цикл вказівник+лічильник, зручний для DJNZ

**Етап 2 — Оптимізації по функціях** (у порядку):
1. `EliminateDeadBlocks` — видалення недосяжних блоків
2. `ReorderBlocks` — покращення «провалювання» (fall-through)
3. **Конвеєр констант** (ітерується до фіксованої точки):
   - `PropagateConstants` — відстеження констант через переміщення
   - `FoldConstants` — обчислення чистих операцій під час компіляції
   - `SimplifyIdentities` — `PtrAdd(x, 0)` → `Move(x)` тощо
   - `ConstantCallElim` — згортання викликів з константними аргументами через VM
4. `DeadStoreElim` — видалення чистих інструкцій з невикористаними результатами (ітеративно)
5. `BranchEquiv` — VM-засноване усунення гілок (доведення надлишкових перевірок)
6. `CondRetSink` — підняття тривіальних else-блоків у умовні повернення

**Етап 3 — Рівень модуля: LUTGen**:
- Чисті функції з параметрами діапазону → таблиці пошуку під час компіляції

**Етап 4 — Верифікація** (`Verify`):
- Унікальні мітки блоків, валідні термінатори, узгодженість типів

**Етап 5 — Міжпроцедурна оптимізація**:
- `OptimizeContracts` — жадібне динамічне програмування на графі викликів для призначення класів регістрів
- `ApplyContracts` — переписування сигнатур функцій

**Етап 6 — Розподіл регістрів**:
- `ComputeLiveness` — зворотний аналіз потоку даних до фіксованої точки
- `PBQPAllocate` — зважене призначення вартості віртуальних регістрів на фізичні регістри Z80

**Етап 7 — Генерація коду**:
- `Z80Codegen` → текст асемблера

### 4.2 Формати виводу

```bash
mz source.nanz --emit=hir        # HIR dump
mz source.nanz --emit=mir2-raw   # MIR2 before optimization
mz source.nanz --emit=mir2       # MIR2 after optimization
mz source.plm  --emit=nanz       # PL/M → Nanz source translation
mz source.nanz -o output.a80     # Z80 assembly (default)
```

---

<a id="chapter-5-mir2-intermediate-representation"></a>

## Розділ 5: Проміжне представлення MIR2

MIR2 — це основний IR. Він використовує **аргументи блоків** (у стилі Cranelift/MLIR) замість phi-вузлів.

### 5.1 Структура модуля

```
Module
├── Funcs[] — each with Blocks[]
├── Globals[] — module-level data
├── Strings[] — interned string pool
└── Structs[] — struct type definitions
```

### 5.2 Інструкції

Кожна інструкція: `%dst = Op %src0, %src1 : Ty [Class]`

**Арифметичні:** `OpAdd`, `OpSub`, `OpMul`, `OpDiv`, `OpSDiv`, `OpMod`
**Побітові:** `OpAnd`, `OpOr`, `OpXor`, `OpShl`, `OpShr`, `OpSar`
**Унарні:** `OpNeg`, `OpNot`
**Перетворення:** `OpExt` (нуль-розширення), `OpSext` (знакове розширення), `OpTrunc`
**Порівняння:** `OpCmp` з умовами: `CmpEq`, `CmpNe`, `CmpLt`, `CmpLe`, `CmpGt`, `CmpGe`, `CmpUlt`, `CmpUle`, `CmpUgt`, `CmpUge`, `CmpSubCarry`
**Переміщення даних:** `OpConst` (безпосередній), `OpMove` (копія регістра)
**Пам'ять:** `OpLoad`, `OpStore`, `OpAddrOf`, `OpAlloca`, `OpField` (ptr+зміщення), `OpPtrAdd` (ptr+зміщення часу виконання), `OpPtrBump` (ptr+крок часу компіляції)
**Виклики:** `OpCall` (прямий), `OpCallIndirect` (через вказівник)
**SMC:** `OpPatchSlot`, `OpLoadPatched`, `OpPatch` (примітиви самомодифікованого коду)

### 5.3 Параметри блоків

```
block @loop_head(%ptr: u16 [pointer], %cnt: u8 [counter]):
    %cond = cmp.gt %cnt, %limit : bool [flag]
    br_if %cond, @exit(), @body(%ptr, %cnt)
```

Аргументи блоків визначають регістри на вході. Аргументи передаються на кожному ребрі (переході/розгалуженні). Це заміняє phi-вузли й робить паралельні копії явними на кожному ребрі.

### 5.4 Термінатори

| Термінатор | Семантика | Генерація Z80 |
|------------|-----------|-------------|
| `TermJmp(target, args)` | Безумовний перехід | `JP label` |
| `TermBrIf(cond, then, else)` | Умовне розгалуження | `JP Z/NZ/C/NC label` |
| `TermDJNZ(counter, body, exit)` | Зменшити B, перейти якщо ненульовий | `DJNZ rel8` |
| `TermCondRet(cond, vals, then)` | Умовне повернення | `RET CC` |
| `TermRet(vals)` | Повернення | `RET` |

`TermCondRet` генерується проходом оптимізації CondRetSink — він задіює одноінструкційне умовне повернення Z80.

---

<a id="chapter-6-optimization-passes"></a>

## Розділ 6: Проходи оптимізації

### 6.1 Конвеєр констант (фіксована точка)

Чотири проходи ітеруються до відсутності змін:

1. **PropagateConstants** — якщо `%r = const 42`, замінити всі використання `%r` на 42.
2. **FoldConstants** — `const(3) + const(5)` → `const(8)`.
3. **SimplifyIdentities** — `PtrAdd(x, Const(0))` → `Move(x)`. Усуває надлишкову арифметику вказівників з нульовим зміщенням (критично для отримувачів `^Struct`).
4. **ConstantCallElim** — чиста функція з усіма константними аргументами → обчислення через MIR2 VM.

### 6.2 Усунення мертвих записів

Видаляє чисті інструкції, результати яких ніколи не використовуються. Ітерується до фіксованої точки, тому що видалення однієї мертвої інструкції може зробити її джерела мертвими.

**Ніколи не видаляються:** `OpStore`, `OpCall`, `OpCallIndirect`, `OpAsm`, `OpPatch` (побічні ефекти).

### 6.3 BranchEquiv (VM-засноване усунення гілок)

Доводить надлишковість умовних розгалужень через вичерпне тестування на VM.

**Приклад:** `abs_diff` з перевіркою `if a == b { return 0 }`. BranchEquiv прогоняє всі 256 вхідних значень `(v, v)` через оригінальну та виправлену функцію. Обидві повертають 0 → гілка доведено надлишкова → замінити `BrIf(eq, @zero, @diff)` на `Jmp(@diff)`.

**Коректно для u8:** вичерпний тест на 256 значень. Для ширших типів: евристична вибірка (безпечно розширити пізніше).

### 6.4 CondRetSink

Знаходить `BrIf(cond, @then, @else)`, де `@else` тривіальний (один попередник, чисті інструкції, `TermRet`). Піднімає інструкції `@else` в поточний блок і замінює `BrIf` на `TermCondRet`.

**Злиті оптимізації, що спрацьовують одразу після підняття:**
- **SubSwapNeg:** Якщо піднятий `sub(a, b)` має зворотний `sub(b, a)` в then-блоці → замінити на `neg(result)`. Економить `LD A,r; SUB r2` → один `NEG`.
- **HoistReorderSubBeforeCmp + CmpSubCarry:** Переставити `sub` перед `cmp_lt` на тих самих операндах → прапор переносу від `SUB` і Є результатом порівняння. Повністю усуває інструкцію `CP`.

### 6.5 LUTGen (рівень модуля)

Чисті функції з одним параметром діапазону → таблиця пошуку.

**Придатність:** 1 параметр типу `u8<lo..hi>` або `u16<lo..hi>`, діапазон ≤ 256, одне повернення, без зовнішніх викликів, без запису в глобальні.

**Процес:**
1. VM-обчислення функції на кожному вхідному значенні діапазону
2. Генерація вирівняної по сторінці таблиці `DB` як глобальної
3. Заміна тіла функції на пошук у таблиці:
   ```asm
   LD H, lut^H    ; 7T — page base (high byte only)
   LD L, C         ; 4T — index
   LD A, (HL)      ; 7T — lookup
   RET
   ```

**Результат:** 18 T-станів незалежно від складності оригінальної функції. Цикл popcount з 8 ітерацій перетворюється на 3 інструкції.

### 6.6 Розподілювач регістрів PBQP

Замінює старий жадібний розподілювач. PBQP (Partitioned Boolean Quadratic Program) мінімізує:

```
Σ nodeCost[r][loc(r)] + Σ edgeCost[interfering pairs]
```

**Вартість вузла:** `useCount[r] × costTable.Cost(r.Class, location)`. Гарячі регістри платять більше за дорогі розташування.

**Правила редукції:**
- **R0:** ступінь 0 (ізольований) → призначити найдешевше розташування одразу.
- **R1:** ступінь 1 (лист) → згорнути вартість ребра в сусіда, відкласти.
- **RN:** ступінь ≥ 2 → жадібно за **дельтою** (`2nd_best − best`). Регістри з великою дельтою (високий штраф за витіснення) розподіляються першими.

**Результат для 4 одночасних регістрів ClassPointer:**
```
p0 → HL (cost 0)
p1 → DE (cost 4)
p2 → BC (cost 6)
p3 → IX (cost 8)  — no $F0xx memory spill
```

### 6.7 Об'єднання копій після розподілу

Після того, як PBQP призначає фізичні розташування, `coalesceAllocResult` усуває надлишкові копії на межах блоків:

- Збирає ребра спорідненості з `OpMove` та пар параметр↔аргумент блоку.
- Однопрохідне перефарбовування: якщо жоден сусід у графі інтерференції не використовує розташування цілі, перефарбувати для відповідності. Блокування `recolored` запобігає циклам ротації у phi-вебах циклів.

### 6.8 Міжпроцедурна оптимізація контрактів

`OptimizeContracts` виконує жадібне динамічне програмування на графі викликів:

1. Топологічне сортування (листя першими).
2. Для кожної функції: перебір кандидатних векторів класів регістрів.
3. Вартість = внутрішня вартість адаптера + вартість ребер по всіх викликачах.
4. Обрати призначення мінімальної вартості.

Це обирає угоди про виклики по функціях глобально, зменшуючи переміщення регістрів на місцях виклику.

---

<a id="chapter-7-z80-code-generation"></a>

## Розділ 7: Генерація коду Z80

`pkg/mir2/z80codegen.go` — перетворює розподілений MIR2 у текст асемблера.

### 7.1 Генерація інструкцій

| Операція MIR2 | Вивід Z80 |
|---------|-----------|
| `OpConst` u8 | `LD r, imm8` |
| `OpConst` u16 | `LD rr, imm16` |
| `OpAdd` u8 | `ADD A, r` |
| `OpAdd` u16 | `ADD HL, rr` |
| `OpSub` u8 | `SUB r` |
| `OpSub` u16 | `AND A; SBC HL, rr` |
| `OpNeg` | `NEG` (лише 8-біт, регістр A) |
| `OpCmp` | `CP r` (результат у прапорцях, ClassFlag) |
| `OpLoad` | `LD A, (HL)` / `LD A, (rr)` |
| `OpStore` | `LD (HL), r` |
| `OpCall` | `CALL label` |

### 7.2 Адресація IX/IY

Коли PBQP призначає вказівник на IX, генератор коду використовує адресацію зі зміщенням:

```asm
LD A, (IX+0)     ; 8-bit load, offset 0
LD (IX+1), A     ; 8-bit store, offset 1
LD L, (IX+0)     ; 16-bit load (lo)
LD H, (IX+1)     ; 16-bit load (hi)
```

**Копіювання 16-бітного регістра↔IX:** DE/BC→IX використовує недокументоване побайтове копіювання:
```asm
LD IXH, D    ; DD 62 — 8T (D not substituted by DD prefix)
LD IXL, E    ; DD 6B — 8T (total 16T)
```

**Побайтове копіювання HL→IX НЕДІЙСНЕ:** Префікс DD підставляє H→IXH та L→IXL як у позиції джерела, так і призначення, тому `LD IXH, H` декодується як `LD IXH, IXH` (NOP). Використовуйте `PUSH HL; POP IX` (21T) замість цього.

### 7.3 Peephole-оптимізації в генераторі коду

**Придушення мертвих констант:** `OpConst`, чиє єдине використання — `OpCmp` → видати `CP imm8` безпосередньо, пропустити `LD r, imm`.

**Поширення копій:** Відстеження `holdsPhys[A] = D` — якщо A вже містить значення D, пропустити `LD A, D`.

**Відстеження останніх прапорців:** Якщо прапорці вже встановлені попередньою інструкцією з тими самими операндами, придушити надлишковий `CP`.

**Вирівняна по сторінці LUT:** Попереднє сканування блоків на шаблони доступу до LUT → `LD H, sym^H; LD L, idx; LD A, (HL)` (18T).

**Пряма адресація полів глобальних структур:** Попереднє сканування на `AddrOf(global) + Field(offset) + Load/Store` → `LD A, (sym__field)` безпосередньо (13T).

**Виявлення HL-ланцюжків:** Кілька послідовних записів полів тієї ж структури → одне `LD HL, sym` + ланцюжок `LD (HL), r; INC HL` (економить перезавантаження HL).

---

<a id="chapter-8-iterator-chains-and-fusion"></a>

## Розділ 8: Ланцюжки ітераторів та злиття

### 8.1 Чотири методи ітераторів

| Метод | Сигнатура | Семантика |
|--------|-----------|-----------|
| `forEach(lambda, n)` | Виконати лямбду для n елементів | Термінальний |
| `map(lambda)` | Перетворити кожен елемент | Проміжний |
| `filter(lambda)` | Залишити елементи, де лямбда повертає true | Проміжний |
| `mapInPlace(lambda, n)` | Перетворити й записати назад | Термінальний |

Вони розпізнаються як UFCS на виразах-вказівниках. `tryLowerIterChain` знижувача HIR зливає ланцюжки.

### 8.2 Злиття

```nanz
buf.map(|x| x * 2).filter(|x| x > 5).forEach(|x| { process(x) }, n)
```

Зливається в єдиний `ForEachStmt`:

```
for x in buf[0..n]:
    let mapped = x * 2         // map body inlined
    if mapped > 5 {            // filter: skip if false
        process(mapped)        // forEach body inlined
    }
```

Це стає єдиним циклом DJNZ:
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

Без проміжного масиву. Без виклику CALL лямбди. Три стадії, один цикл.

### 8.3 Захоплення замикань у злитих ланцюжках

```nanz
fun sum(buf: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    buf.forEach(|x: u8| { s = s + x }, n)
    return s
}
```

`s` — це вільна змінна в лямбді. Компілятор:
1. Виявляє вільне посилання через `hasFreeVars()`.
2. Пропускає окреме зниження `lambda_N`.
3. Пропускає `s` як параметр блоку через цикл DJNZ.

Результат: `s` живе в регістрі (наприклад, C) протягом усього часу — без витіснень у пам'ять, нуль купи.

### 8.4 Вартість на елемент (sum_chain)

```asm
LD D, (HL)     ; 7T — load element
LD A, C        ; 4T
ADD A, D       ; 4T — s + x
LD C, A        ; 4T
INC HL         ; 6T
DJNZ .loop     ; 13T (taken)
; Total: 38T per element
```

Для порівняння: окремий виклик функції на кожен елемент додає CALL(17T) + RET(10T) = 27T накладних витрат до будь-якої роботи.

### 8.5 Зворотний запис mapInPlace

```nanz
buf.mapInPlace(|x: u8| (x + 2), n)
```

Прапорець `MutateInPlace` запускає зворотний запис після тіла лямбди:

```asm
LD A, (HL)     ; load
ADD A, 2       ; transform
LD (HL), A     ; write back
INC HL
DJNZ .loop
```

### 8.6 Відомі зламані ітератори

- **enumerate:** Конфлікт регістра B — B одночасно є і лічильником DJNZ, і індексом нумерації.
- **reduce:** Регістр A перезаписується між двома параметрами SMC.

---

<a id="chapter-9-zero-cost-abstractions"></a>

## Розділ 9: Абстракції з нульовою вартістю

Кожна абстракція в Nanz компілюється в прямі інструкції Z80. Ось докази.

### 9.1 UFCS — нульова вартість

`obj.method(args)` розгортається в `method(obj, args)` під час парсингу. Таблиця методів — це `map[string]map[string]methodInfo`, що використовується тільки під час парсингу.

**Вартість: нуль.** `CALL Acc_add` ідентичний рукописному виклику.

### 9.2 Інтерфейси — нульова вартість

```nanz
interface Animal { speak }
struct Dog {}
fun Dog.speak(self: Dog) -> u8 { return 1 }
```

Без vtable, без fat pointer. При `g_dog.speak()` компілятор розв'язує Dog під час парсингу й генерує `CALL Dog_speak`.

**Вартість: нуль.** 17T на прямий `CALL` проти ~55T для диспетчеризації інтерфейсів у стилі Go.

**Обмеження:** Конкретний тип повинен бути статично відомим. Справжня динамічна диспетчеризація не реалізована (і порушила б принцип нульових накладних витрат).

### 9.3 Лямбди — нульова вартість

Кожна `|x| expr` стає `lambda_N` — звичайною функцією. При використанні в ланцюжках ітераторів тіло вбудовується. При використанні як вказівника на функцію — стандартний `CALL`.

**Вартість: нуль виділень, нуль структур замикань.** Накладні витрати CALL тільки коли не вбудовано.

### 9.4 Методи структур — нульова вартість

`fun Vec2.add(self: Vec2, ...)` зберігається як `Vec2_add`. Манглення імен — єдині «накладні витрати» (під час компіляції, не під час виконання). Згенерований асемблер невідрізнимий від вільної функції.

### 9.5 Перевантаження операторів — нульова вартість

`a + b` на типах структур диспетчеризується в `op_add(a, b)` — звичайний виклик функції. Без перевірки типів під час виконання.

---

<a id="chapter-10-qbe-correctness-oracle"></a>

## Розділ 10: QBE — оракул коректності

### 10.1 Що таке QBE

QBE (https://c9x.me/compile/) — це невеликий, швидкий бекенд компілятора, що перетворює QBE IL (простий формат SSA) у нативний асемблер x86-64 або arm64. Він НЕ є цільовою платформою Nanz — він є **інструментом тестування**.

### 10.2 Конвеєр оракула коректності

```
source.nanz
    ├─→ HIR → MIR2 → Z80Codegen → MZE emulator → Result A
    └─→ HIR → MIR2 → QBE IL → qbe → cc → native binary → Result B

If A ≠ B: Bug is in Z80 codegen (MIR2 semantics proven correct)
If A = B: Both pipelines agree — correctness confirmed
```

Конвеєр QBE зупиняється ПЕРЕД Z80-специфічними кроками (оптимізація контрактів, розподіл регістрів). QBE виконує свій власний розподіл регістрів.

### 10.3 E2E-тести

`pkg/mir2qbe/e2e_test.go` містить 7 E2E-тестів:

| Тест | Що перевіряє |
|------|-----------------|
| `TestE2E_PLM_AbsDiff` | PL/M → HIR → MIR2 → QBE → native |
| `TestE2E_PLM_Fib` | Fibonacci з циклом |
| `TestE2E_Nanz_SumArray` | Арифметика вказівників, цикл `ptr[i]` |
| `TestE2E_Nanz_AbsDiff` | Керування потоком (if/else) |
| `TestE2E_Nanz_StructFields` | Доступ до полів глобальних структур |
| `TestE2E_Nanz_UFCS` | Диспетчеризація методів на глобальних |
| `TestE2E_Nanz_Interface_ZeroCost` | Мономорфізація диспетчеризації інтерфейсів |

Тести автоматично пропускаються, якщо `qbe` відсутній у PATH (`exec.LookPath("qbe")`).

### 10.4 Трансляція MIR2 → QBE

Ключові відповідності (`pkg/mir2qbe/codegen.go`):
- Усі цілочисельні типи (u8, u16, i8, i16, bool) → QBE `w` (32-бітне слово)
- `ptr` → QBE `l` (64-бітний long, нативний вказівник)
- Параметри блоків → phi-вузли (QBE використовує SSA з phi, а не аргументи блоків)
- Z80-специфічні операції (SMC, push/pop, вбудований асемблер) → пропускаються

### 10.5 Встановлення QBE

Див. [Додаток D](#appendix-d-installing-external-tools).

---

<a id="chapter-11-plm-80-corpus-seeding-and-idiomatic-translation"></a>

## Розділ 11: PL/M-80: засівання корпусу та ідіоматичний переклад

### 11.1 Роль PL/M у створенні екосистеми Nanz

PL/M-80 служив **початковим корпусом** для бекенду MIR2. 26 файлів Intel 80 Tools (ALGOL-M, BASIC-80, макроасемблер ML80 тощо) — весь реальний виробничий код 1970-х років — надали 1 338 функцій та 11 661 інструкцій для тестування конвеєра HIR→MIR2→Z80 ще до того, як Nanz взагалі існувала.

Робочий процес:

```
Step 1: Parse PL/M corpus (26/26 files, 100% coverage)
Step 2: Lower to HIR → verify correctness
Step 3: Lower to MIR2 → verify optimizations
Step 4: Emit Z80 → compare with Intel's PL/M-80 V4.0 output
Step 5: --emit=nanz → generate Nanz source from HIR
Step 6: Write idiomatic Nanz by hand, guided by the mechanical translation
```

Це означає, що кожен вузол HIR, кожна оптимізація MIR2 і кожен шаблон генерації коду Z80 були спочатку перевірені на **реальному коді PL/M** до того, як програми Nanz їх використали.

### 11.2 Механічний переклад: `mz program.plm --emit=nanz`

Прапорець `--emit=nanz` виконує `plm.Compile()` → HIR → `nanz.Print()`, генеруючи синтаксично правильний вихідний код Nanz. Переклад структурний — він зберігає логіку програми PL/M точно.

**Вихідний код PL/M:**
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

**Механічний вивід Nanz** (`mz sum.plm --emit=nanz`):
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

**Синтаксичні відповідності:**

| PL/M-80 | Nanz | Примітки |
|---------|------|-------|
| `PROCEDURE name(a,b) BYTE;` | `fun name(a: u8, b: u8) -> u8` | Типи inline |
| `DECLARE X BYTE;` | `var x: u8` | Конвенція нижнього регістру |
| `DECLARE (A,B) WORD;` | `var a: u16; var b: u16` | Множинне оголошення розгорнуто |
| `DO WHILE cond; ... END;` | `while cond { ... }` | |
| `DO I = 0 TO N; ... END;` | `for i in 0..n { ... }` | Лічений цикл |
| `DO CASE X; ... END;` | `switch x { case 0: ...; }` | |
| `IF cond THEN s1; ELSE s2;` | `if cond { ... } else { ... }` | |
| `ARR(I)` | `arr[i]` | Нотація індексації |
| `CALL fn(a,b);` | `fn(a, b)` | |
| `DECLARE X LITERALLY 'Y'` | *(розгорнуто до парсингу)* | Макроси зникли |

Обидва компілюються в **ідентичний асемблер Z80** — той самий HIR, той самий MIR2, та сама генерація коду.

### 11.3 Від механічного до ідіоматичного: три рівні

Той самий алгоритм суми масиву демонструє три рівні:

**Рівень 1 — Механічний переклад PL/M** (індексований цикл):
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

**Рівень 2 — Ідіоматичний Nanz** (послідовне сканування):
```nanz
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    for x: u8 in ptr[0..n] {   // sequential: INC HL per element (6T)
        s = s + x
    }
    return s
}
```

**Рівень 3 — Ланцюжок ітераторів із замиканням** (повне злиття):
```nanz
fun sum_array(ptr: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    ptr.forEach(|x: u8| { s = s + x }, n)  // DJNZ loop, s in register
    return s
}
```

**Вартість Z80 на елемент:**

| Рівень | Ключова інструкція | T-стани/елемент | Примітки |
|-------|----------------|-----------------|-------|
| 1 (індексований) | `ADD HL, DE` | ~64T | Обчислення ptr+i на кожній ітерації |
| 2 (for-each) | `INC HL` | ~43T | Послідовне просування вказівника |
| 3 (forEach) | `INC HL` + `DJNZ` | ~38T | Злитий, s у регістрі |

На 3,5 МГц зі 100 елементами: Рівень 1 = 1,83 мс, Рівень 3 = 1,09 мс — **на 40% швидше** завдяки суто синтаксичній зміні.

### 11.4 Шаблони PL/M → Ланцюжки ітераторів Nanz

Найвпливовіший переклад: ручні цикли DO WHILE у PL/M → ланцюжки ітераторів Nanz.

**PL/M: фільтрація + обробка**
```plm
I = 0;
DO WHILE I < N;
    V = BUF(I) * 2;
    IF V > THRESHOLD THEN CALL PROCESS(V);
    I = I + 1;
END;
```

**Механічний Nanz:**
```nanz
var i: u8 = 0
while i < n {
    let v = buf[i] * 2
    if v > threshold { process(v) }
    i = i + 1
}
```

**Ідіоматичний Nanz (злитий ланцюжок):**
```nanz
buf.map(|x: u8| (x * 2))
   .filter(|x: u8| (x > threshold))
   .forEach(|x: u8| { process(x) }, n)
```

Версія з ланцюжком: **один цикл DJNZ, нуль проміжних масивів, усі лямбди вбудовані.** Три стадії злиті в ~6 інструкцій Z80 на елемент.

**PL/M: знайти максимум**
```plm
MAX = 0;
I = 0;
DO WHILE I < N;
    IF BUF(I) > MAX THEN MAX = BUF(I);
    I = I + 1;
END;
```

**Ідіоматичний Nanz (forEach із захопленням):**
```nanz
var m: u8 = 0
buf.forEach(|x: u8| {
    if x > m { m = x }
}, n)
```

Захоплена змінна `m` пропускається як параметр блоку через цикл DJNZ — вона живе в регістрі, ніколи не розливається в пам'ять.

**PL/M: перетворення на місці**
```plm
I = 0;
DO WHILE I < N;
    BUF(I) = BUF(I) + 2;
    I = I + 1;
END;
```

**Ідіоматичний Nanz:**
```nanz
buf.mapInPlace(|x: u8| (x + 2), n)
```

Один цикл: завантажити, перетворити, записати назад. Прапорець `MutateInPlace` запускає зворотний запис.

### 11.5 Чого PL/M не може виразити

Ці можливості Nanz не мають еквіваленту в PL/M:

| Можливість Nanz | Еквівалент PL/M | Чому це важливо |
|-------------|----------------|----------------|
| Типи з діапазоном `u8<0..255>` | Немає | Дозволяє LUTGen (генерацію таблиць під час компіляції) |
| Типізовані вказівники `^Struct` | BASED (нетипізований) | Розв'язання полів, авто-розіменування, диспетчеризація UFCS |
| `interface Animal { speak }` | Немає | Контракт часу компіляції, диспетчеризація нульової вартості |
| `buf.map().filter().forEach()` | Ручний DO WHILE | Єдиний злитий цикл, без проміжних масивів |
| Захоплення замикання `\|x\| { s = s + x }` | Немає | Змінні, що переносяться через цикл, як параметри блоків |
| Перевантаження операторів | Немає | `a + b` на типах структур |

### 11.6 PL/M-80 V4.0 проти MIR2: якість коду

Реальне порівняння (зі Звіту #036) — **той самий вихідний код PL/M**, скомпільований оригінальним компілятором Intel проти нашого бекенду MIR2:

| Функція | Intel PL/M-80 V4.0 | MIR2 Z80 | Економія |
|----------|-------------------|----------|---------|
| `abs_diff` | 33 байти | 12 байтів | **-64%** |
| `fib` | 47 байтів | 31 байт | **-34%** |
| **Всього** | **80 байтів** | **43 байти** | **-46%** |

Компілятор Intel зберігає всі параметри та локальні змінні в пам'яті (угода про виклики 8080). ABI MIR2 з пріоритетом регістрів тримає значення в A/B/C/D/HL — нуль звернень до пам'яті в гарячих циклах.

---

<a id="chapter-12-compiled-output-gallery"></a>

## Розділ 12: Галерея скомпільованого виводу

Кожен блок коду нижче — це **реальний вивід компілятора** з `mz <file>.nanz -o <file>.a80` на поточній збірці master (2026-03-10). Вихідні файли заархівовані в `reports/showcase-src/2026-03-10/`.

### 12.1 Доступ до полів структур — Оптимізація HL-ланцюжків

**Вихідний код** (`ex1_struct.nanz`):
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

**Скомпільований Z80** (`ex1_struct.a80`):
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

**Оптимізація:** Виявлення HL-ланцюжків зливає три записи полів в одну послідовність `LD HL` + `INC HL`: 53T проти 79T наївних (-33%).

### 12.2 UFCS-диспетчеризація методів — нуль vtable

**Вихідний код** (`ex2_ufcs.nanz`):
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

**Скомпільований Z80** (`ex2_ufcs.a80`):
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

### 12.3 Диспетчеризація інтерфейсів нульової вартості

**Вихідний код** (`ex3_iface.nanz`):
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

**Скомпільований Z80** (`ex3_iface.a80`):
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

17T на диспетчеризацію (лише CALL). Диспетчеризація інтерфейсів у стилі Go: ~55T.

### 12.4 abs_diff — П'ять проходів оптимізації до мінімуму

**Вихідний код** (`ex4a_abs_diff.nanz`):
```nanz
fun abs_diff(a: u8, b: u8) -> u8 {
    if a == b { return 0 }
    if a < b { return b - a }
    return a - b
}
```

**Скомпільований Z80** (`ex4a_abs_diff.a80`):
```z80
abs_diff:
    SUB D               ; A = a - b, carry set if a < b
    LD C, A             ; (regression: contract assigned b→D, result→C)
    RET NC              ; a >= b → return a-b
.abs_diff_if_then3:
    NEG                 ; A = -(a-b) = b-a
    RET
```

BranchEquiv усунув перевірку `a == b`. CondRetSink підняв `sub` перед `cmp`. CmpSubCarry усунув `CP`. Результат: 4 основні інструкції.

### 12.5 LUTGen — Цикл часу компіляції → Пошук у 3 інструкції

**Вихідний код** (`ex5_lut.nanz`):
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

**Скомпільований Z80** (`ex5_lut.a80`):
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

**18T всього.** Цикл while ніколи не виконується під час виконання — його було обчислено під час компіляції VM MIR2 для всіх 256 вхідних значень.

### 12.6 forEach із захопленням замикання — Цикл DJNZ

**Вихідний код** (`ex6_foreach.nanz`):
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

**Скомпільований Z80** (`ex6_foreach.a80`):
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

**sum_chain: 38T/елемент.** Без CALL лямбди. Захоплена `s` живе в C протягом усього часу.

### 12.7 mapInPlace — Перетворення на місці зі зворотним записом

**Вихідний код** (`ex7_mapinplace.nanz`):
```nanz
fun add2_inplace(buf: ^u8, n: u8) -> void {
    buf.mapInPlace(|x: u8| (x + 2), n)
}
```

**Скомпільований Z80** (`ex7_mapinplace.a80`):
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

**Відома регресія:** `LD D, 2` мертвий, `INC C; INC C` замінює `ADD A, 2` (51T проти 40T/елемент). Окрема `lambda_0` також мертвий код.

### 12.8 GCD — Цикл з двома мутабельними змінними

**Вихідний код** (`ex8_gcd.nanz`):
```nanz
fun gcd(a: u8, b: u8) -> u8 {
    while a != b {
        if a > b { a = a - b }
        else     { b = b - a }
    }
    return a
}
```

**Скомпільований Z80** (`ex8_gcd.a80`):
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

Об'єднувач фази 6c усунув усі накладні витрати копіювання на межах блоків у гарячому циклі. Лише трамплін виходу (холодний шлях) має обміни регістрів.

---

<a id="chapter-13-relation-to-minz-and-plm"></a>

## Розділ 13: Зв'язок із MinZ та PL/M

### 13.1 Три фронтенди

| Мова | Розширення | Парсер | Бекенд | Статус |
|----------|-----------|--------|---------|--------|
| MinZ | `.minz` | `pkg/parser/participle/` | MIR1 → old codegen | Заморожений |
| Nanz | `.nanz` | `pkg/nanz/parse.go` | HIR → MIR2 → Z80 | **Активний** |
| PL/M-80 | `.plm` | `pkg/plm/` | HIR → MIR2 → Z80 | Активний |

Маршрутизатор у `cmd/minzc/main.go`:
```go
if ext == ".plm" || ext == ".nanz" {
    return compileViaHIR(src, ext)
} else {
    // old MIR1 path for .minz
}
```

### 13.2 Коли що використовувати

**Nanz** — нові Z80/CP/M-програми, сучасний синтаксис, LUTGen, розподіл PBQP.
**MinZ** — існуючі `.minz`-програми, що працюють. Має метафункції (`@define`, `@print`), яких ще немає в Nanz.
**PL/M-80** — портування застарілого ПЗ CP/M. 100% корпусу Intel 80 Tools парситься.

### 13.3 Розриви у можливостях (Nanz проти MinZ)

Nanz наразі не має:
- Макросів препроцесора `@define`
- Оптимізованого виводу рядків `@print`
- Строкової інтерполяції у стилі Ruby (`"Hello #{name}"`)
- Синтаксису множинних значень повернення (MIR2 підтримує, парсер — ні)
- Перевантаження функцій (MinZ має; Nanz вимагає різних імен)
- `@extern` з анотаціями класів регістрів (задокументовано, але ще не парситься)

### 13.4 Інші бекенди (лише MinZ)

Старий конвеєр MinZ (прапорець `-b`) підтримує кілька цілей. Вони НЕ доступні для Nanz (який завжди проходить через MIR2 → Z80):

| Бекенд | Прапорець | Статус | Вивід |
|---------|------|--------|--------|
| Z80 | `-b z80` | Продакшн | `.a80` |
| 6502 | `-b 6502` | Бета | `.s` |
| 68000 | `-b m68k` | Альфа | `.s` |
| i8080 | `-b i8080` | Бета | `.s` |
| Game Boy | `-b gb` | Активний | `.s` |
| WASM | `-b wasm` | Альфа | `.wat` |
| C | `-b c` | Бета | `.c` |
| Crystal | `-b crystal` | Бета | `.cr` |
| LLVM | `-b llvm` | Заплановано | `.ll` |

Вони використовують старий IR (MIR1), а не MIR2. Довгостроковий план — припинити MIR1 і направити всі фронтенди через HIR → MIR2.

---

<a id="appendix-a-complete-syntax-grammar"></a>

## Додаток A: Повна граматика синтаксису

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

**Лексичні примітки:**
- Коментарі: `//` (рядковий) і `/* */` (блоковий)
- Пробіли не мають значення
- Цілі числа: десяткові або `0x`/`0X` шістнадцяткові
- Рядки: в подвійних лапках, без escape-послідовностей

---

<a id="appendix-b-register-classes-and-cost-table"></a>

## Додаток B: Класи регістрів і таблиця вартості

### Фізичні регістри Z80

| Регістр | Клас | Вартість | Примітки |
|----------|-------|------|-------|
| A | ClassAcc | 0T | Акумулятор ALU, значення повернення, 1-й параметр u8 |
| B | ClassCounter | 0T | Лічильник DJNZ, 3-й параметр |
| C, D, E, H, L | ClassGeneral | 0T | Загальні 8-бітні |
| HL | ClassPointer | 0T | Основний вказівник, 1-й параметр u16/ptr, повернення |
| DE | ClassIndex | 0T | 2-й параметр u16, джерело LDIR |
| BC | ClassPair | 0T | 3-й параметр u16 |
| IX | ClassIX | 8T | Додатковий вказівник (+4T DD-префікс на кожен доступ) |
| IY | ClassIY | 8T | Рідко використовується (зарезервований системою на деяких платформах) |
| IXH/IXL | ClassIXY8 | 8T | Недокументовані 8-бітні половинки |
| $F0xx | ClassMem | 26T | Розширення «регістрового файлу» в пам'яті |

### Ієрархія класів регістрів

| Рівень | Класи | Вартість | Механізм |
|------|---------|------|-----------|
| 0 — Основні | Acc, Counter, General, Pointer, Index, Pair, Flag | 0T | Пряме використання |
| 1 — IX/IY | IX, IY, IXY8 | 4-8T | DD/FD-префікс |
| 2 — Тіньові | Shadow, AccShadow | 8T | EXX / EX AF,AF' |
| 3 — Стек | Stack | 21T | PUSH + POP |
| 4 — Пам'ять | Mem | 26T | LD (addr) / LD addr |

ClassFlag — особливий: він представляє прапорці процесора Z80 (Z, CY тощо) і коштує 0T. Булеві результати порівнянь зберігаються в прапорцях без матеріалізації в регістрі.

---

<a id="appendix-c-cli-reference"></a>

## Додаток C: Довідник CLI

### Компіляція програм Nanz

```bash
mz source.nanz -o output.a80              # compile to Z80 assembly
mz source.nanz -o output.a80 --target=cpm # target CP/M
mz source.nanz -o output.a80 --target=spectrum  # target ZX Spectrum
mz source.nanz -o output.a80 --target=agon      # target Agon Light 2
```

### Проміжні представлення

```bash
mz source.nanz --emit=hir        # HIR dump
mz source.nanz --emit=mir2-raw   # MIR2 before optimization
mz source.nanz --emit=mir2       # MIR2 after optimization
mz source.plm  --emit=nanz       # PL/M → Nanz translation
```

### Прапорці оптимізації

```bash
mz source.nanz --disable-optimize      # disable all optimizations
mz source.nanz --disable-ir-opt        # disable MIR-level opts
mz source.nanz --disable-asm-opt       # disable peephole
mz source.nanz --disable-smc           # disable self-modifying code
mz source.nanz --compile-trace         # show all optimization steps
```

### Запуск тестів

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

### Приклади програм

| Файл | Опис |
|------|-------------|
| `examples/nanz/01_sum_array.nanz` | Цикл while з `ptr[i]` |
| `examples/nanz/02_sum_array_idiomatic.nanz` | For-each та ітератор forEach |
| `examples/nanz/03_filter_map_chain.nanz` | Повний ланцюжок map/filter/forEach |
| `examples/nanz/04_lut_popcount.nanz` | Генерація LUT через тип з діапазоном |
| `examples/nanz/05_four_pointers.nanz` | PBQP: 4 регістри ClassPointer |
| `examples/nanz/06_pbqp_weighted.nanz` | Зважений розподіл вартості |
| `examples/nanz/07_ix_load_store.nanz` | Адресація переповнення IX |

---

<a id="appendix-d-installing-external-tools"></a>

## Додаток D: Встановлення зовнішніх інструментів

### QBE (оракул коректності)

QBE потрібен лише для запуску E2E-тестів коректності (`pkg/mir2qbe/`). Він НЕ потрібен для звичайної компіляції Nanz.

**Linux (збірка з вихідного коду):**
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

**Перевірка:**
```bash
qbe --version        # should print version
echo 'export function w $main() { @start ret 0 }' | qbe
```

**Компілятор C** також потрібен (будь-який C99-компілятор: `gcc`, `clang`). Зазвичай попередньо встановлений.

Якщо `qbe` відсутній у PATH, E2E-тести автоматично пропускаються з `t.Skip("qbe not in PATH")`.

### Асемблер MZA (вбудований)

Зовнішнє встановлення не потрібне. MZA — частина інструментарію MinZ:

```bash
cd minzc && make mza
# or: make install-user (installs all tools to ~/.local/bin/)
```

### Емулятор MZE (вбудований)

```bash
cd minzc && make mze
```

Використовується для: запуску скомпільованих бінарних файлів Z80, обчислення констант усередині компілятора (LUTGen, BranchEquiv).

---

<a id="appendix-e-known-bugs-and-limitations"></a>

## Додаток E: Відомі помилки та обмеження

### Парсер

| Проблема | Статус | Обхідний шлях |
|-------|--------|------------|
| `@extern` з анотаціями `params=`/`returns=` | Не реалізовано | Використовуйте базовий `@extern fun` (автоматичне призначення регістрів) |
| Перевантаження функцій | Не реалізовано | Використовуйте різні імена (`abs_diff`, `abs_diff_u16`) |
| Множинні значення повернення | Не парситься | MIR2 підтримує; парсер — ні |
| Escape-послідовності рядків | Не реалізовано | Без `\n`, `\t` тощо |
| Блоки вбудованого асемблера `@asm` | Не реалізовано | Використовуйте `@extern` з обгортками asm |

### Генерація коду

| Проблема | Статус | Деталі |
|-------|--------|---------|
| `applySubSwapNeg` на u16 | Баг | Примусово встановлює ClassAcc (8-біт) на 16-бітний результат NEG. Відсутня перевірка `Ty.Width() <= 8`. |
| Глобальні структури нульового розміру | Баг | `struct Dog {}` не генерує даних; символ невизначений під час компонування. Виправлення: генерувати `Dog: EQU $`. |
| Сентинел `LD A, mem` | Баг | Помилка розподілу регістрів генерує літеральний рядок "mem" в асемблері. |
| Збереження для 2-го аргументу | Баг | Другий аргумент повторних викликів може бути затертий. |
| mapInPlace constant-add | Регресія | `ADD A, imm` не спрацьовує, коли елемент у C (а не A). |

### Ланцюжки ітераторів

| Проблема | Статус | Деталі |
|-------|--------|---------|
| `enumerate` | Зламаний на Z80 | B = лічильник конфліктує з B = індекс нумерації |
| `reduce` | Зламаний на Z80 | A перезаписується між двома параметрами SMC |
| Захоплення замикань у незлитих лямбдах | Невизначена поведінка | Лямбди, передані як вказівники на функції, не можуть захоплювати зовнішні локальні змінні |

### Розподілювач регістрів

| Проблема | Статус | Деталі |
|-------|--------|---------|
| Регістри в пам'яті | Продуктивність | ~207T фактично проти ~43T ідеально на елемент ітератора, коли регістри розливаються в $F0xx |
| Дрейф оптимізатора контрактів | Відомо | Може призначати субоптимальні класи (наприклад, `b → D` замість `b → B` для abs_diff) |
| Вартості ребер PBQP | Відкладено | Корельовані рішення розподілу (BC* для LUT) ще не змодельовані |

---

*Nanz: Сучасний синтаксис, абстракції нульової вартості, продуктивність Z80.*

*Конвеєр: `.nanz` → `nanz.Parse()` → `*hir.Module` → `hir.LowerModule()` → `*mir2.Module` → оптимізація → розподіл → Z80Codegen → `.a80`*
