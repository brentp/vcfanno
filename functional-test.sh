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

vcfanno -js example/custom.js example/conf.toml example/query.vcf.gz > obs
show "annotated vcf"
check $(zgrep -cv ^# example/query.vcf.gz) $(grep -cv ^# obs)

show "checking that header is updated"
check "6" $(grep ^# obs | grep -c otto)
