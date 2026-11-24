#!/bin/sh
n=$(grep -c '"name"' "$1")
if grep -q "\"name\":\"$2\"" "$1"; then echo "$n"; else echo $((n + 1)); fi
