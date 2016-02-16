BASE=/data2/gemini_install/data/gemini/data/
QUERY_VCF=$BASE/ExAC.r0.3.sites.vep.tidy.20.vcf.gz
QUERY_VCF=a57.vcf.gz
QUERY_VCF=$BASE/ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites.tidy.20.vcf.gz
NAME=exac
NAME=1kg
PROCS=8
for procs in 1 4 8 12; do
for try in 1 2; do
for GAP in 1000 5000 10000 20000 50000 100000; do
	for CHUNK in 1000 5000 10000 20000 50000 100000; do
		IRELATE_MAX_GAP=$GAP IRELATE_MAX_CHUNK=$CHUNK vcfanno -base-path $BASE -p $procs -lua custom.lua gem.conf $QUERY_VCF > /dev/null 2> time.$procs.$NAME.$GAP.$CHUNK.try-$try.txt
		echo -n $GAP $CHUNK $procs $try" "; tail -1 time.$procs.$NAME.$GAP.$CHUNK.try-$try.txt
	done
done
done
done
