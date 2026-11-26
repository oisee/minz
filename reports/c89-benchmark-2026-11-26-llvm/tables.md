rows: 272, all PASS

## Geometric means

| Left | Right | Cases | T ratio | Bytes ratio |
|---|---|---|---|---|
| minz default | llvm o2 | 26 | 0.874 | 0.939 |
| minz default | llvm os | 26 | 0.825 | 0.950 |
| minz size | llvm os | 26 | 0.909 | 0.895 |
| minz default | sdcc speed | 26 | 0.629 | 0.981 |
| llvm o2 | sdcc speed | 26 | 0.719 | 1.045 |
| llvm os | sdcc size | 26 | 0.725 | 1.034 |
| nanz-idiom default | llvm o2 | 9 | 0.650 | 1.085 |

## Per case (T-states of the workload; program bytes)

| Case | MinZ default T | llvm -O2 T | llvm -Os T | SDCC speed T | MinZ/llvm-O2 T | MinZ B | llvm -O2 B | llvm -Os B | SDCC speed B |
|---|---|---|---|---|---|---|---|---|---|
| abs_diff | 220904 | 227606 | 227606 | 258142 | 0.97 | 27 | 42 | 42 | 33 |
| bitwise8reg | 655360 | 747520 | 747520 | 673792 | 0.88 | 76 | 88 | 88 | 75 |
| checksum16 | 26021176 | 24540535 | 25501865 | 23303736 | 1.06 | 343 | 366 | 362 | 340 |
| copy_bytes | 108292 | 204510 | 204510 | 683670 | 0.53 | 170 | 178 | 178 | 211 |
| count_less | 32433349 | 88779780 | 88779852 | 74877619 | 0.37 | 321 | 341 | 341 | 324 |
| crc8 | 52259304 | 28108785 | 81204025 | 104720964 | 1.86 | 312 | 360 | 331 | 334 |
| fat12_decode | 24384 | 14848 | 14848 | 25024 | 1.64 | 148 | 126 | 126 | 149 |
| find_max | 10098356 | 10372447 | 9261853 | 15939327 | 0.97 | 297 | 303 | 296 | 294 |
| fusion_dot_product | 2463988 | 3784642 | 3784642 | 2834057 | 0.65 | 1638 | 633 | 633 | 586 |
| fusion_filter_count | 967135 | 3171630 | 3173610 | 1818220 | 0.30 | 312 | 327 | 325 | 296 |
| fusion_filter_map_sum | 365180 | 387712 | 391621 | 626142 | 0.94 | 306 | 307 | 301 | 300 |
| fusion_fold_max | 319439 | 377283 | 353892 | 578293 | 0.85 | 299 | 307 | 300 | 298 |
| fusion_fold_min | 296183 | 368561 | 331754 | 558391 | 0.80 | 298 | 306 | 299 | 297 |
| fusion_map_filter_sum | 1162630 | 644067 | 642540 | 1014504 | 1.81 | 328 | 337 | 335 | 327 |
| fusion_map_store | 684135 | 302530 | 302530 | 662995 | 2.26 | 556 | 559 | 559 | 552 |
| fusion_two_ptr_xor | 1517785 | 1686955 | 1686955 | 928050 | 0.90 | 602 | 627 | 627 | 578 |
| fusion_window_sum | 534155 | 466350 | 467190 | 1314768 | 1.15 | 358 | 327 | 325 | 369 |
| gcd | 3067013 | 4255738 | 5148130 | 4243840 | 0.72 | 51 | 55 | 57 | 56 |
| in_interval | 404116 | 248046 | 248046 | 302012 | 1.63 | 50 | 50 | 50 | 40 |
| memset_fill | 3588390 | 3686325 | 3701625 | 14807040 | 0.97 | 297 | 340 | 337 | 294 |
| mul8x8 | 815104 | 1804544 | 1742592 | 1012992 | 0.45 | 42 | 74 | 72 | 42 |
| popcount8 | 56598726 | 58682220 | 100142735 | 93942400 | 0.96 | 313 | 358 | 326 | 304 |
| reverse_loop11 | 10616370 | 25538820 | 25538820 | 14021760 | 0.42 | 555 | 585 | 585 | 552 |
| strlen_bytes | 3574400 | 6641920 | 6641920 | 9083520 | 0.54 | 283 | 279 | 279 | 278 |
| struct_roundtrip | 344064 | 380928 | 380928 | 2023424 | 0.90 | 45 | 94 | 94 | 167 |
| sum_array | 6141696 | 5661924 | 5661932 | 7433216 | 1.08 | 305 | 320 | 320 | 316 |

llvm -O2 needs fewer T-states than MinZ default on 8 of 26 cases: checksum16, crc8, fat12_decode, fusion_map_filter_sum, fusion_map_store, fusion_window_sum, in_interval, sum_array

llvm -O2 is smaller than MinZ default on 4 of 26 cases: fat12_decode, fusion_dot_product, fusion_window_sum, strlen_bytes
