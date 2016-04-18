
BASE=/media/brentp/transcend/gemini_install/data/gemini_data/
VCF=/usr/local/src/gocode/src/github.com/brentp/vcfanno/a57.20k.vcf.gz
#go run ../../vcfanno.go -p 4 -lua example.lua -base-path $BASE example.conf $VCF > /dev/null
~/Downloads/vcfanno_0.0.10_linux_amd64/vcfanno -p 4 -lua example.lua -base-path $BASE example.conf $VCF

