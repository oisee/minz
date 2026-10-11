#!/usr/bin/env bash
set -eu
cd /tmp/minz-grace-pgo-resume/minzc
GOCACHE=/tmp/minz-grace-gocache MINZ_PGO_SCALAR_ARTIFACT_DIR=/tmp/minz-grace-pgo-resume/docs/grace-pgo-scalar-artifacts /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go test ./pkg/pipeline -run '^TestMeasuredScalarChecksumProgram$' -count=1 -v
