#!/bin/bash

# long-running tests run before a release to make sure everything is copacetic

test -e ssshtest || wget -q https://raw.githubusercontent.com/ryanlayer/ssshtest/master/ssshtest

. ssshtest

set -o nounset

BASE=/usr/local/src/gemini_install//data/gemini_data/

V=vcfanno

IRELATE_VERBOSE=TRUE IRELATE_MAX_GAP=500 run clinvar_common_pathogenic $V -lua docs/examples/clinvar_exac.lua -p 4 -base-path $BASE docs/examples/clinvar_exac.conf $BASE/clinvar_20160203.tidy.vcf.gz
assert_equal 594 $(zgrep -wc common_pathogenic $STDOUT_FILE)
assert_equal $(zgrep -cv ^# $STDOUT_FILE) $(zgrep -cv ^# $BASE/clinvar_20160203.tidy.vcf.gz)

tail -1 $STDERR_FILE

run exac_combine vcfanno -base-path $BASE $BASE/ExAC.r0.3.sites.vep.tidy.vcf.gz docs/examples/exac_combine/exac_combine.conf

orun() {
$V -lua example/custom.lua -p 4 -base-path $BASE example/gem.conf $BASE/ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites.tidy.vcf.gz | head -100000 | python tests/find-out-of-order.py

}

IRELATE_MAX_CHUNK=2 IRELATE_MAX_GAP=10 run filehandletest orun
assert_exit_code 0
assert_equal $(grep -c "too many open files" $STDERR_FILE) "0"
