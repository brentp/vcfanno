set -e o nounset
DATA=/data2/gemini_install/data/gemini_data/
Q=/data2/u6000294/ALL.phase3.autosome.vcf.gz
export GOGC=500
vcfanno -p 16 -base-path $DATA -lua custom.lua gem.conf $Q > /dev/null 2> 1kg.genotypes.err

