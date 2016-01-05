set -e
check() {
	a=$1
	b=$2
	if [[ "$a" -ne "$b" ]]; then
		echo " <ERROR!>" "$a != $b"
		exit 4
	else
		echo " <OK!>"
	fi
	echo ""
}
  
go install -race -a

_N=0
show() {
	_N=$(($_N+1))
	echo "===================================================================="
	echo -n "<## TEST.$_N ##>" $1
}

vcfanno -lua example/custom.lua example/conf.toml example/query.vcf.gz > obs 2>err
show "annotated vcf"
check $(zgrep -cv ^# example/query.vcf.gz) $(grep -cv ^# obs)

show "checking that header is updated"
check "6" $(grep ^# obs | grep -c lua)

show "check warning message for missing chrom 2 in annotation dbs"
check "2" $(grep -c "not found in" err)

vcfanno -ends -lua example/custom.lua example/conf.toml example/query.vcf.gz > obs 2>err
show "checking that ends works"
n=$(grep -v ^# obs | grep -c right_)
check $( (( $n > 0 )) ) true
n=$(grep -v ^# obs | grep -c left_)
check $( (( $n > 0 )) ) true

show "checking that samples are retained"
n=$(grep  ^"#CHROM" obs | cut -f 10- | wc -w)
check $n 3

show "checking that all non-header lines have same number of columns"
n=$(grep  -v "^##" obs | awk 'BEGIN{FS="\t"}{ print NF}' | uniq)
check $n 12

echo "PASSED ALL TESTS"
exit 0
