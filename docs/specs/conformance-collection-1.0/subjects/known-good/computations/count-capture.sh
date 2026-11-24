#!/bin/sh
tr ',' '\n' < "$1" | grep -c '"name"'
