test -e ssshtest || wget -q https://raw.githubusercontent.com/ryanlayer/ssshtest/master/ssshtest

. ssshtest

go install -race -a

run check_example vcfanno -lua example/custom.lua example/conf.toml example/query.vcf.gz
assert_equal $(zgrep -cv ^# example/query.vcf.gz) $(grep -cv ^# obs)
assert_equal 6 $(grep ^# $STDOUT_FILE | grep -c lua)
assert_equal 2 $(grep -c "not found in" $STDERR_FILE)

run check_ends vcfanno -ends -lua example/custom.lua example/conf.toml example/query.vcf.gz
n=$(grep -v ^# $STDOUT_FILE | grep -c right_)
assert_equal $(( $n > 0 )) 1

n=$(grep -v ^# $STDOUT_FILE | grep -c left_)
assert_equal $(( $n > 0 )) 1

n=$(grep  ^"#CHROM" obs | cut -f 10- | wc -w) 
assert_equal $n 3


# checking that all non-header lines have same number of columns
n=$(grep  -v "^##" obs | awk 'BEGIN{FS="\t"}{ print NF}' | uniq)
assert_equal $n 12
