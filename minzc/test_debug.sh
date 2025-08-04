#!/bin/bash
cd /Users/alice/dev/minz-ts/minzc
DEBUG=1 ./minzc test_print_literal.minz -o test_print_literal.a80 2>&1 | grep -C3 "unsupported expression type"