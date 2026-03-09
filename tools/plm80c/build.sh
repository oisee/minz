#!/bin/bash
# Build plm80c (Intel PL/M-80 V4.0 compiler, C port by Mark Ogden)
# Source: https://github.com/ogdenpm/c-ports
#
# Tested on macOS arm64. Requires gcc.
# Three macOS compatibility patches are applied in-place to the cloned source.
#
# Usage:
#   ./build.sh            # clones, patches, builds → /tmp/plm80c
#   ./build.sh /my/path   # clone to custom path
#
set -e

ROOT="${1:-/tmp/c-ports}"
BUILD=/tmp/plm_build

if [ ! -d "$ROOT" ]; then
    echo "=== cloning ogdenpm/c-ports ==="
    git clone https://github.com/ogdenpm/c-ports "$ROOT"
fi

# ── macOS patch 1: CHAR_BIT is in <limits.h>, not implicitly available ──────
if ! grep -q 'limits.h' "$ROOT/utility/option.c"; then
    echo "=== patching option.c (CHAR_BIT / limits.h) ==="
    sed -i.bak '1s/^/#include <limits.h>\n/' "$ROOT/utility/option.c"
fi

# ── macOS patch 2: fcloseall() doesn't exist on macOS ───────────────────────
if ! grep -q '__APPLE__' "$ROOT/shared/os.c"; then
    echo "=== patching os.c (fcloseall stub) ==="
    # Insert stub before the first line that references fcloseall
    sed -i.bak 's/extern int fcloseall(void);/#ifdef __APPLE__\nstatic int fcloseall(void) { fflush(NULL); return 0; }\n#else\nextern int fcloseall(void);\n#endif/' "$ROOT/shared/os.c"
fi

# ── stub _version.h (getVersion utility needs a working git repo to generate it) ─
echo '#define GIT_VERSION "4.0"' > "$ROOT/plm80c/_version.h"

# ── build ────────────────────────────────────────────────────────────────────
mkdir -p "$BUILD"
cd "$BUILD"

INC="-I$ROOT/plm80c -I$ROOT/utility -I$ROOT/shared"

echo "=== building utility ==="
# Note: exit.o is compiled but NOT linked — it duplicates _Exit from os.o on macOS
gcc -O2 $INC -c "$ROOT/utility/file.c"    -o file.o
gcc -O2 $INC -c "$ROOT/utility/getch.c"   -o getch.o
gcc -O2 $INC -c "$ROOT/utility/memory.c"  -o memory.o
gcc -O2 $INC -c "$ROOT/utility/message.c" -o message.o
gcc -O2 $INC -c "$ROOT/utility/option.c"  -o option.o

echo "=== building shared ==="
gcc -O2 $INC -c "$ROOT/shared/cmdline.c" -o cmdline.o
gcc -O2 $INC -c "$ROOT/shared/os.c"      -o os.o
gcc -O2 $INC -DAPP_NAME='"plm80c"' -DCPORT -c "$ROOT/shared/_appinfo.c" -o _appinfo.o

echo "=== building plm80c sources ==="
for f in "$ROOT/plm80c/"*.c; do
    name=$(basename "$f" .c)
    gcc -O2 $INC -c "$f" -o "${name}.o"
done

echo "=== linking (exit.o excluded — macOS _Exit conflict) ==="
gcc -O2 -o /tmp/plm80c \
    file.o getch.o memory.o message.o option.o \
    cmdline.o os.o _appinfo.o \
    *.o 2>/dev/null || \
gcc -O2 -o /tmp/plm80c $(ls *.o | grep -v '^exit\.o$')

echo "Done: $(/tmp/plm80c 2>&1 | head -1)"
echo "Binary: /tmp/plm80c"
