# Original 51_impl_cpm_demo: pure methods only

Exact `examples/nanz/51_impl_cpm_demo.nanz` wholemodule сохранён: Circle/Rect area, putchar/print_u8/newline/main, включая original BDOS ASM. Ничего не удалено и source patch отсутствует. SHA/domain зафиксированы до исполнения `d276a089`, base `8db01ae2`. Bookv7 §4.1 byte layout, §4.4 implimplicitself pointer, §3.1 u8 задают независимую модель.

Circle_area: все256 radius при объектах6000 иffff, `(3*r*r)&255`. Rect_area: все65536 width/height при6000 иffff, `(w*h)&255`; inline height приffff находитсяв0000. Wholeobject+adjacentguards сохраняются послекаждоговызова. Raw/final VM loads и native actualfieldreads проверены по адресу/значению/порядку, ниединогоobjectstore. Повторяющиеся radiusreferences обычной RAM допустимо forward, все оставшиеся reads обязаны совпадать. Не приписываем объектам volatile/MMIO semantics.

Ordinary/GraceSize/GraceSpeed: Circle512calls/mode, Rect131072calls/mode =394752actualnativeCALL, столько же raw/final independent checks. Actualparameterhomes andAreturn, callerPC/SP, exactPUSH/CALL wordbytes и полный64KiB outsideobservedstack проверены. Native fetchedPC допускается только внутридвухoriginalmethods, originaldrivers иреальногоcompiler-emitted `__mul8`; printer/main/BDOS paths недопустимы. No fakeBDOS/hostcomputation. __mul8 являетсянастоящим softwaremultiply, его effects включены встоимость иstackпроверку.

**Это не whole-program CP/M успех.** Originalmain/output не исполнялись: BDOS function2, startup/runtime/clobber behavior требуют отдельного environment proof. Полныйmodule остаётсяruntimeblocked, а дваoriginalpureentries получили независимое positive evidence. Local measuredimageORIGIN8000 не объявляется runnableCOM0100. Эта граница сохраненаresultsJSON.

Финальный frozen executable PASS3,08s; productionfix не нужен. BINparity всехтрёхрежимов, новыйGracegain/heldout не заявлен. Source/ASM/BIN/raw/final/ABI/allocation/symbols/resultscosts иcompressedexe сохраняютсяSHAmanifest. Шара `/mnt/safe/minz/grace/2026-10-11/impl-cpm-methods/REPORT.md`. Rootfullgate не перезапускался.
