set -e
check() {
	a=$1
	b=$2
	if [[ "$a" -ne "$b" ]]; then
		echo " <ERROR!>" "$a != $b"
	else
		echo " <OK!>"
	fi
	echo ""
}
  
go install -race

_N=0
show() {
	_N=$(($_N+1))
	echo "===================================================================="
	echo -n "<## TEST.$_N ##>" $1
}

vcfanno -js example/custom.js example/conf.toml example/query.vcf.gz > obs 2>err
show "annotated vcf"
check $(zgrep -cv ^# example/query.vcf.gz) $(grep -cv ^# obs)

show "checking that header is updated"
check "6" $(grep ^# obs | grep -c otto)


vcfanno -ends -js example/custom.js example/conf.toml example/query.vcf.gz > obs 2>err
show "checking that ends works"
n=$(grep -v ^# obs | grep -c right_)
check $( (( $n > 0 )) ) true
n=$(grep -v ^# obs | grep -c left_)
check $( (( $n > 0 )) ) true
