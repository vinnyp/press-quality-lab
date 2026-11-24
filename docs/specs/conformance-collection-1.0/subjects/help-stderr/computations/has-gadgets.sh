#!/bin/sh
if grep -q '"gadget"' "$1"; then printf 'true\n'; else printf 'false\n'; fi
