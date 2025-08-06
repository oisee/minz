#!/bin/bash
cd /Users/alice/dev/minz-ts/minzc
echo "Testing REPL build..."
go build -o mzr cmd/mzr/main.go
if [ $? -eq 0 ]; then
    echo "✅ REPL built successfully!"
    ls -la mzr
else
    echo "❌ Build failed"
fi