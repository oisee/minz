rows: 272, all PASS

## Geometric means

| Left | Right | Cases | T ratio | Bytes ratio |
|---|---|---|---|---|
| minz default | llvm o2 | 26 | 0.764 | 0.914 |
| minz default | llvm os | 26 | 0.721 | 0.925 |
| minz size | llvm os | 26 | 0.837 | 0.872 |
| minz default | sdcc speed | 26 | 0.550 | 0.955 |
| llvm o2 | sdcc speed | 26 | 0.719 | 1.045 |
| llvm os | sdcc size | 26 | 0.725 | 1.034 |
| nanz-idiom default | llvm o2 | 9 | 0.648 | 1.076 |

## Per case (T-states of the workload; program bytes)

| Case | MinZ default T | llvm -O2 T | llvm -Os T | SDCC speed T | MinZ/llvm-O2 T | MinZ B | llvm -O2 B | llvm -Os B | SDCC speed B |
|---|---|---|---|---|---|---|---|---|---|
| abs_diff | 212712 | 227606 | 227606 | 258142 | 0.93 | 26 | 42 | 42 | 33 |
| bitwise8reg | 647168 | 747520 | 747520 | 673792 | 0.87 | 75 | 88 | 88 | 75 |
| checksum16 | 26021176 | 24540535 | 25501865 | 23303736 | 1.06 | 343 | 366 | 362 | 340 |
| copy_bytes | 104132 | 204510 | 204510 | 683670 | 0.51 | 162 | 178 | 178 | 211 |
| count_less | 32244421 | 88779780 | 88779852 | 74877619 | 0.36 | 314 | 341 | 341 | 324 |
| crc8 | 30064104 | 28108785 | 81204025 | 104720964 | 1.07 | 335 | 360 | 331 | 334 |
| fat12_decode | 13952 | 14848 | 14848 | 25024 | 0.94 | 122 | 126 | 126 | 149 |
| find_max | 10093236 | 10372447 | 9261853 | 15939327 | 0.97 | 296 | 303 | 296 | 294 |
| fusion_dot_product | 2463988 | 3784642 | 3784642 | 2834057 | 0.65 | 1638 | 633 | 633 | 586 |
| fusion_filter_count | 960835 | 3171630 | 3173610 | 1818220 | 0.30 | 305 | 327 | 325 | 296 |
| fusion_filter_map_sum | 363920 | 387712 | 391621 | 626142 | 0.94 | 304 | 307 | 301 | 300 |
| fusion_fold_max | 319199 | 377283 | 353892 | 578293 | 0.85 | 298 | 307 | 300 | 298 |
| fusion_fold_min | 295943 | 368561 | 331754 | 558391 | 0.80 | 297 | 306 | 299 | 297 |
| fusion_map_filter_sum | 474557 | 644067 | 642540 | 1014504 | 0.74 | 313 | 337 | 335 | 327 |
| fusion_map_store | 320235 | 302530 | 302530 | 662995 | 1.06 | 561 | 559 | 559 | 552 |
| fusion_two_ptr_xor | 1516345 | 1686955 | 1686955 | 928050 | 0.90 | 597 | 627 | 627 | 578 |
| fusion_window_sum | 534155 | 466350 | 467190 | 1314768 | 1.15 | 358 | 327 | 325 | 369 |
| gcd | 3058821 | 4255738 | 5148130 | 4243840 | 0.72 | 50 | 55 | 57 | 56 |
| in_interval | 333824 | 248046 | 248046 | 302012 | 1.35 | 44 | 50 | 50 | 40 |
| memset_fill | 3578150 | 3686325 | 3701625 | 14807040 | 0.97 | 295 | 340 | 337 | 294 |
| mul8x8 | 744448 | 1804544 | 1742592 | 1012992 | 0.41 | 35 | 74 | 72 | 42 |
| popcount8 | 44136656 | 58682220 | 100142735 | 93942400 | 0.75 | 337 | 358 | 326 | 304 |
| reverse_loop11 | 10611250 | 25538820 | 25538820 | 14021760 | 0.42 | 554 | 585 | 585 | 552 |
| strlen_bytes | 3561600 | 6641920 | 6641920 | 9083520 | 0.54 | 280 | 279 | 279 | 278 |
| struct_roundtrip | 319488 | 380928 | 380928 | 2023424 | 0.84 | 42 | 94 | 94 | 167 |
| sum_array | 6129408 | 5661924 | 5661932 | 7433216 | 1.08 | 299 | 320 | 320 | 316 |

llvm -O2 needs fewer T-states than MinZ default on 6 of 26 cases: checksum16, crc8, fusion_map_store, fusion_window_sum, in_interval, sum_array

llvm -O2 is smaller than MinZ default on 4 of 26 cases: fusion_dot_product, fusion_map_store, fusion_window_sum, strlen_bytes
