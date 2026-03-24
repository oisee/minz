# Report 112: Screen Gallery — MZV + CP/M Verified

**Date:** 2026-03-24
**Session:** ABAP Screens + Open SQL + VT100 Bridge

---

## Summary

All @screen examples verified on MZV (MIR2 VM) and 5/6 on Z80 CP/M via VT100. This report captures the actual rendered output from each screen program.

---

## 1. Selection Screen (`screen_declarative.nanz`)

**7 lines of DSL → full interactive TUI**

```nanz
@screen("Material Report") {
    field "Material" length 10 default "*"
    field "Plant" length 6
    int   "Count" default 10
    button "Execute" key F8
    button "Back" key F3
}
```

```
+---------------------------------------------+
|   Material Report                           |  <- white-on-blue title bar
|                                             |
|   Material    [*         ]                  |  <- cyan label, blue focus
|   Plant       [      ]                      |  <- cyan label
|   Count       [10]                          |  <- cyan label
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- bright buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- white-on-blue status bar
+---------------------------------------------+
  MZV verified | CP/M VT100 verified | 2387B binary
```

---

## 2. Customer Master (`screen_customer.nanz`)

**4-field form with defaults**

```
+---------------------------------------------+
|   Create Customer                           |  <- white-on-blue
|                                             |
|   Name        [ACME Corp           ]        |  <- blue focus on first field
|   City        [               ]             |  <- cyan labels
|   Country     [DE ]                         |
|   Credit      [5000]                        |
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- bright buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- white-on-blue status bar
+---------------------------------------------+
  MZV verified | CP/M VT100 verified | 2709B binary
```

---

## 3. Mini-ALV (`screen_alv.nanz`)

**Table grid with column headers and 5 data rows**

```
+---------------------------------------------+
|   Material List                             |  <- white-on-blue title bar
|                                             |
|   Filter      [*         ]                  |  <- filter field
|   MATNR      MTART  MEINS                   |  <- bright column headers
|   -----------------------                   |
|   MAT-001    FERT   ST                      |     data rows
|   MAT-002    HALB   KG                      |
|   MAT-003    ROH    L                       |
|   MAT-004    FERT   ST                      |
|   MAT-005    VERP   PC                      |
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- bright buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- white-on-blue status bar
+---------------------------------------------+
  MZV verified | 1996B binary
```

---

## 4. The Holy Grail: @screen + SELECT + ALV (`screen_report.nanz`)

**Selection screen → SQLite query → 8-airline grid**

```
+--------------------------------------------------------------+
|   SCARR Report                                               |  <- white-on-blue
|                                                              |
|   Carrier     [*  ]                                          |  <- filter field
|   ID   Carrier Name           URL                            |  <- bright headers
|   ---------------------------------------------------------- |
|   AA   American Airlines      http://www.aa.com              |
|   AF   Air France             http://www.airfrance.com       |
|   BA   British Airways        http://www.ba.com              |
|   EK   Emirates               http://www.emirates.com        |
|   LH   Lufthansa              http://www.lufthansa.com       |
|   QF   Qantas                 http://www.qantas.com.au       |
|   SQ   Singapore Airlines     http://www.singaporeair.com    |
|   UA   United Airlines        http://www.united.com          |
|                                                              |
|   [F8=Execute]  [F3=Back]                                    |  <- buttons
|                                                              |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back                  |  <- status bar
+--------------------------------------------------------------+
  MZV verified | CP/M VT100 verified | 3875B binary
  8 airlines from SQLite, filter via sqlite_query_like
```

---

## 5. Manual ABAP Screen (`abap_screen.nanz`)

**Hand-written PBO/PAI with enums**

```
+---------------------------------------------+
|   Material Report                           |  <- white-on-blue
|                                             |
|   Material    [*         ]                  |  <- cyan label
|   Plant       [      ]                      |
|   Count       [10]                          |
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- bright buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- status bar
+---------------------------------------------+
  MZV verified | CP/M VT100 verified | 2264B binary
```

---

## 6. OOP TUI Screen (`tui_screen.nanz`)

**Direct tui_* API calls — the Level 2 approach**

```
+---------------------------------------------+
|   Material Report                           |  <- white-on-blue
|                                             |
|   Material    [*                 ]          |  <- cyan label
|   Plant       [    ]                        |
|   Count       [10]                          |
|                                             |
|   [F8=Execute]  [F3=Back]                   |  <- buttons
|                                             |
|   TAB=Next  Enter=Edit  F8=Execute  F3=Back |  <- status bar
+---------------------------------------------+
  MZV verified | CP/M VT100 verified | 1239B binary
```

---

## Test Matrix

| Example | MZV | mz | mza | Size | mze CP/M | Notes |
|---------|-----|-----|-----|------|----------|-------|
| screen_declarative.nanz | PASS | PASS | PASS | 2387B | VT100 | @screen DSL |
| screen_customer.nanz | PASS | PASS | PASS | 2709B | VT100 | 4-field form |
| screen_alv.nanz | PASS | PASS | PASS | 2821B | stub | table render WIP |
| screen_report.nanz | PASS | PASS | PASS | 3875B | VT100 | holy grail |
| abap_screen.nanz | PASS | PASS | PASS | 2264B | VT100 | manual PBO/PAI |
| tui_screen.nanz | PASS | PASS | PASS | 1239B | VT100 | direct tui_* API |

**5/6 VT100 verified on CP/M.** All 6/6 on MZV.

---

## Architecture

```
Nanz source                    Z80 binary on CP/M terminal
     |                                    |
     v                                    v
 @screen DSL                    ESC[2J ESC[H         <- tui_clear
     |                          ESC[97;104m          <- tui_color
     v                          ESC[1;1H Title       <- tui_goto + tui_puts
 screen_gen.go                  ESC[0m               <- tui_reset
 (Go codegen)                   ESC[3;3H             <- tui_goto
     |                          ESC[36;40m Label     <- tui_color + tui_puts
     v                          ESC[37;40m [Value]   <- field value
 HIR -> MIR2 -> Z80            ESC[24;1H Status      <- status bar
     |
     v
 import tui.render
 (BDOS VT100 bodies)
     |
     v
 .com binary (1-4KB)
```

---

## Session Commits (2026-03-24)

| Commit | Feature |
|--------|---------|
| b9aa3471 | @screen built-in metafunction |
| 0970a43b | Mini-ALV table display |
| dc40ff1a | MZV-verified ASCII illustrations |
| fcb82067 | Open SQL -> SQLite bridge |
| 1d75c58f | Holy Grail: @screen + SELECT + ALV |
| 9280fead | Filter integration (WHERE from screen) |
| 08c2b640 | SELECT...ENDSELECT loop body |
| e95f1596 | Import resolver (glob alias + __ mangling) |
| e73ad7ff | tui_goto register clobbering fix |
| 0c4a3695 | tui_color register clobbering fix |
| 5049aef4 | @screen call alias remap for imports |

*First time an ABAP-style SE16 report has run on Z80 hardware.*
