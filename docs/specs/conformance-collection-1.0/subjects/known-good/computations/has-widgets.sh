#!/bin/sh
if grep -q '"name"' "$1"; then printf 'true\n'; else printf 'false\n'; fi
