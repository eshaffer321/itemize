#!/usr/bin/env bash
set -euo pipefail

if [[ "${ITEMIZE_SKIP_PRECOMMIT:-}" == "1" ]]; then
  echo "ITEMIZE_SKIP_PRECOMMIT=1 set; skipping local checks."
  exit 0
fi

echo "Running itemize pre-commit checks..."
make pre-commit-local
