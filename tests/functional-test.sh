#!/bin/bash

test -e ssshtest || wget -q https://raw.githubusercontent.com/ryanlayer/ssshtest/master/ssshtest

. ssshtest

set -o nounset


go build -race -a
export PATH=.:$PATH
echo using $(which vcfanno)


run check_self_number vcfanno -base-path tests/data/ -lua example/custom.lua tests/data/number.conf tests/data/number-input.vcf
assert_equal 0 $(grep -c "lua error in postannotation" $STDERR_FILE)
cat $STDERR_FILE

run check_empty_args_gives_non_zero_exit vcfanno
assert_exit_code 2

run check_example vcfanno -lua example/custom.lua example/conf.toml example/query.vcf.gz
assert_equal $(zgrep -cv ^# example/query.vcf.gz) $(grep -cv ^# $STDOUT_FILE)
assert_equal 6 $(grep ^# $STDOUT_FILE | grep -c lua)
# 2 are for chromsome 2 not found in header
# 1 is for 2:98688 (bedtools intersect -v -a example/query.vcf.gz -b example/fitcons.bed.gz)
# so lua_start doesn't exist.
assert_equal 3 $(grep -c "not found in" $STDERR_FILE)

run check_samples vcfanno -base-path tests/citest/ tests/citest/conf.toml  tests/citest/test.vcf
n=$(grep -c ^#CHROM $STDOUT_FILE)
assert_equal $n 1

run check_ends vcfanno -ends -lua example/custom.lua example/conf.toml example/query.vcf.gz
n=$(grep -v ^# $STDOUT_FILE | grep -c right_)
assert_equal $(( $n > 0 )) 1

n=$(grep -v ^# $STDOUT_FILE | grep -c left_)
assert_equal $(( $n > 0 )) 1

n=$(grep  ^"#CHROM" $STDOUT_FILE | cut -f 10- | wc -w) 
assert_equal $n 3

# test bam stuff
ret=$(grep -w 10712 $STDOUT_FILE)
n=$(echo $ret | grep -c "coverage=9;")
assert_equal $n 1
n=$(echo $ret | grep -c "xdp2=0,9;")
assert_equal $n 1


n=$(grep  -v "^##" $STDOUT_FILE | awk 'BEGIN{FS="\t"}{ print NF}' | uniq)
assert_equal $n 12

run check_lua_required vcfanno example/conf.toml example/query.vcf.gz
assert_exit_code 1
assert_in_stderr ERROR
assert_in_stderr lua
assert_no_stdout

cat << EOF > __t.conf
[[annotation]]
file="example/exac.vcf.gz"
ops=["lua:non_existent_field"]
names=["nef"]
fields=["xxx"]
EOF

run check_not_bgz vcfanno -p 4 tests/notbgz/a.conf tests/notbgz/notbgz.vcf.gz
assert exit_code 0

run check_field_warning vcfanno -lua example/custom.lua __t.conf example/exac.vcf.gz
assert_exit_code 0
assert_in_stderr "xxx not found in header"


cat << EOF > __t.conf
[[postannotation]]
file="example/exac.vcf.gz"
op="lua:non_existent_field"
name="nef"
fields=["xxx"]
EOF

run check_postannotation_field_warning vcfanno -lua example/custom.lua __t.conf example/exac.vcf.gz
assert_exit_code 1
assert_in_stderr "must specify a type"

echo "type='String'" >> __t.conf
run check_postannotation_field_warning vcfanno -lua example/custom.lua __t.conf example/exac.vcf.gz
assert_exit_code 0
assert_in_stderr "xxx not found in header"



run check_cipos vcfanno -base-path tests/citest/ tests/citest/conf.toml  tests/citest/test.vcf  | grep -v ^#
assert_exit_code 0
assert_equal 1 $(grep -c "END=97;ExonGene=GeneY" $STDOUT_FILE)

run check_cipos_ends vcfanno -ends -base-path tests/citest/ tests/citest/conf.toml  tests/citest/test.vcf  | grep -v ^#
assert_exit_code 0
assert_equal 2 $(grep -c "left_ExonGene=GeneY" $STDOUT_FILE)
assert_equal 0 $(grep -c "GeneY,GeneY" $STDOUT_FILE)


run check_multiple_alts vcfanno tests/data/multiple-alts.conf tests/data/multiple-alts.vcf.gz
assert_exit_code 0
# there should be 0 non-header lines without 'max_maf' since we are annotating self.
assert_equal 0 $(grep -v max_maf $STDOUT_FILE | grep -cv ^#)


run check_overwrite_a vcfanno tests/overwrite-multiple-alts/a/conf.toml tests/overwrite-multiple-alts/a/input.vcf
assert_exit_code 0
assert_in_stderr "using op 'self' when with Number='1' for 'raw' from 'tests/overwrite-multiple-alts/a/whole.vcf.gz' can"

run check_overwrite_b vcfanno tests/overwrite-multiple-alts/b/conf.toml tests/overwrite-multiple-alts/b/input.vcf
assert_exit_code 0
assert_in_stdout "CADD_SCALED=1.9,0.6;CADD=-0.1,-0.3"

touch e.lua
run check_ends_overlap vcfanno -lua e.lua -base-path tests/citest/at/ -ends tests/citest/at/conf.toml tests/citest/at/test.vcf | grep -v ^#
assert_exit_code 0
assert_equal 2 $(grep -c ";left_ExonTranscript=" $STDOUT_FILE)
assert_equal 3 $(grep -c ";right_ExonTranscript=" $STDOUT_FILE)
assert_equal 3 $(grep -c ";right_ref_alt=A" $STDOUT_FILE)
rm e.lua

irefalt() {
    vcfanno -lua <(echo "") -permissive-overlap -base-path tests/dbnsfp/ tests/dbnsfp/conf.toml tests/dbnsfp/Calls_for_dbNSFP_example.vcf.gz | grep -v ^#
}

run check_iref_alt irefalt
assert_in_stdout "nsalt=A,G,T"
assert_exit_code 0

irefalt_strict() {
    vcfanno -lua <(echo "") -base-path tests/dbnsfp/ tests/dbnsfp/conf.toml tests/dbnsfp/Calls_for_dbNSFP_example.vcf.gz | grep -v ^#
}
run check_iref_alt_strict irefalt_strict
assert_in_stdout $'nsalt=T\t'
# check that ID was set.
assert_in_stdout $'\tReadPosRankSum;ORIGID\t'
assert_exit_code 0


refaltend() {
    vcfanno -base-path tests/ref-alt-test/ tests/ref-alt-test/tmp_annotations.toml tests/ref-alt-test/tmp_calls.vcf.gz
}
run check_ref_alt_posns refaltend
assert_exit_code 0
assert_equal 3 $(grep -c ALT_60 $STDOUT_FILE)
assert_equal 3 $(grep -c HET_60 $STDOUT_FILE)
assert_equal 3 $(grep -c ALT_90 $STDOUT_FILE)
assert_equal 3 $(grep -c HET_90 $STDOUT_FILE)
cat $STDERR_FILE

astar() {
vcfanno tests/astar/astar.conf tests/astar/astar.vcf
}

run check_astar astar
assert_exit_code 0
assert_in_stdout "ExAC_AF=0.021771,0"
assert_in_stdout "ExAC_AN=17546;ExAC_Hom=7,.;"


multiallelics() {
    vcfanno tests/multiple-alts/ma.conf tests/multiple-alts/ma-query.vcf
}
run check_multiallelics multiallelics
assert_exit_code 0
assert_in_stdout "STR_by=hello,goodbye|goodbye-again"
assert_in_stdout "AF_by=0.00059904|0.99,0.0029952"

idtest() {
    vcfanno -lua tests/id-test/some.lua tests/id-test/small.toml tests/id-test/small.vcf.gz
}

run check_ids idtest
assert_exit_code 0
assert_equal $(grep -v ^# $STDOUT_FILE | awk '$3 != "."' | wc -l) 2
assert_in_stdout "rs9996;COSM4590035;COSM4590034"
assert_in_stdout "cosmic_filter=QQ,ZZ"
# test delete
assert_equal $(grep -c "NS=" $STDOUT_FILE) 0
# test that Description is transferred from CAF
assert_in_stdout "comma delimited list of allele frequencies based on 1000Genomes"
# make sure that ID didn't get added to the header as we're just updating the ID column.
assert_equal $(grep -c "ID=ID" $STDOUT_FILE) 0
assert_equal $(grep -c "ID=FILTER" $STDOUT_FILE) 0

fn_self_test_with_A_and_many_overlaps() {
    vcfanno tests/testVA/vcfanno.config tests/testVA/test-input.vcf
}

run self_test_with_A_and_many_overlaps fn_self_test_with_A_and_many_overlaps
assert_exit_code 0
assert_equal $(tail -1 $STDOUT_FILE | cut -f 8) "AC=35,65;GN_AF=0.3717,0.6122"
assert_in_stdout "ID=GN_AF,Number=A"


run check_by_alts_float vcfanno -base-path tests/by_alt_float/ tests/by_alt_float/conf.toml tests/by_alt_float/q.vcf
assert_exit_code 0
assert_equal $(tail -1 $STDOUT_FILE | cut -f 8) "AF=1.56250e-02;AQ=1328;CADD_RAW=0.591814"
go install -a


run check_missing_postanno vcfanno -lua tests/missing_postanno/o.lua tests/missing_postanno/o.conf tests/missing_postanno/o.vcf
assert_exit_code 0
assert_in_stdout "exome_af=-1"

