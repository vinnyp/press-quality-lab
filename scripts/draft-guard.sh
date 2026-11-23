#!/usr/bin/env bash
set -euo pipefail
[[ ! -e "$1/KIT.sha256" && ! -L "$1/KIT.sha256" ]] || { echo "draft collection kit must not contain KIT.sha256 (seal is a later unit)" >&2; exit 1; }
