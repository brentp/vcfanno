#!/bin/bash
LC_ALL=C zcat $1 | awk '($0 ~ /^#/) { print } ( $1 !~ /^#/) { exit; }'; zgrep -Pv "^#|^M" $1 | sed 's/^chr//'  | sort -k1,1V -k2,2n
