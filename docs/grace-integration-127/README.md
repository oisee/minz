# Original plasma K11 and integrated live-predicate correction

Original full plasma source frozen before transformation. Generic multiplication
lost a live comparison predicate: x0,y1 expected1, got0. c75fe869 materializes
pending flags before all multiplication paths, preserving A. Corrected baseline
plasma remains byte-identical with its original793332T/1555B.

00bab964 adds byte K11 sequence using one scratch and existing saves. Root
1024 original plasma_attr raw/final VM/native calls PASS0.097s:
793332→331508T, full module1555B unchanged because other consumers retain
runtime. Root18432 live-predicate native cases PASS0.177s; exhausted scratch
PASS0.339s. Fullscreen/render/runtime is not claimed; exact full module retained.

Full CLI checkpoint708e77c5 terminalPASS365.129s, predating subsequent
preflight/typed-address/multiply fixes. Full pipelinea7954042 still live;
it predates multiply integration. Exact running executables are retained.

Sites v66 private deployment succeeded at20:57:16UTC, sourcef2f551ae,
compiler1dd58076. It includes51-path catalog/CY/fusion but predates plasma
integration. https://minz-grace-hir-mir-report.grittysnipe3.chatgpt.site/current-state.html
