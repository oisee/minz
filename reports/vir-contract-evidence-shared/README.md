# Consolidated late VIR evidence mirror

Eleven explicit late VIR report packages,85 files, were audited against all actual package files, Git tracking, manifest entries and SHA256, then copied and rechecked at /mnt/safe/minz/grace/2026-10-12/vir-contract-evidence-7f3c254c/. Zero failures. audit.json lists every payload with its repository path, permitting a single mirror audit. This preservation result is not native execution proof and does not include future files or the currently running full-gate log.

Use scripts/audit_report_packages.py with the package list and --shared-root from audit.json. Unlike the corpus reference audit, this check detects files present in a package but missing from its manifest. Negative tests verify missing/unlisted/shared-missing cases and unsafe path rejection.
