#!/bin/bash

# long-running tests run before a release to make sure everything is copacetic

test -e ssshtest || wget -q https://raw.githubusercontent.com/ryanlayer/ssshtest/master/ssshtest

. ssshtest

set -o nounset

BASE=/usr/local/src/gemini_install//data/gemini_data/

run clinvar_common_pathogenic vcfanno -lua docs/examples/clinvar_exac.lua -p 4 -base-path $BASE docs/examples/clinvar_exac.conf $BASE/clinvar_20150305.tidy.vcf.gz
assert_equal 567 $(zgrep -wc common_pathogenic $STDOUT_FILE)
assert_equal $(zgrep -cv ^# $STDOUT_FILE) $(zgrep -cv ^# $BASE/clinvar_20150305.tidy.vcf.gz)

#run exac_combine vcfanno -base-path $BASE $BASE/ExAC.r0.3.sites.vep.tidy.vcf.gz docs/examples/exac_combine/exac_combine.conf
