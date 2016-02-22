set -e
DATA=/data2/gemini_install/data/gemini/data/
echo -e "method\tprocs\ttime\tquery" > par-times.txt
for Q in /data2/gemini_install/data/gemini/data/ExAC.r0.3.sites.vep.tidy.vcf.gz /data2/gemini_install/data/gemini/data/ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites.tidy.vcf.gz; do
	f=$(basename $Q .tidy.vcf.gz)
	echo $f
	export GOGC=500
	for p in $(seq 20 -1 1); do
		e=paper.go1.4.err.fix$GOGC.$p
		vcfanno -p $p -base-path $DATA -lua custom.lua gem.conf $Q > /dev/null 2> $e
		runtime=$(grep -oP "[^ ]+ seconds" $e)
		echo -e "fixed\t$p\t$runtime\t$f" >> par-times.txt
	done
done

