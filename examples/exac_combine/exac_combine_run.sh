vt decompose -s ExAC.r0.3.nonpsych.sites.vcf.gz | vt normalize -r ../human_g1k_v37_decoy_phix.fasta - | bgzip -c > ExAC.r0.3.nonpsych.sites.tidy.vcf.gz &
vt decompose -s ExAC.r0.3.nonTCGA.sites.vep.vcf.gz | perl -pe "s/;CSQ=.*//g" | vt normalize -r ../human_g1k_v37_decoy_phix.fasta - | bgzip -c > ExAC.r0.3.nonTCGA.sites.tidy.vcf.gz &
wait
tabix ExAC.r0.3.nonpsych.sites.tidy.vcf.gz
tabix ExAC.r0.3.nonTCGA.sites.tidy.vcf.gz

python exac_combine_mkconf.py ExAC.r0.3.nonpsych.sites.tidy.vcf.gz nonpsych > exac_combine.conf
python exac_combine_mkconf.py ExAC.r0.3.nonTCGA.sites.tidy.vcf.gz nonTCGA >> exac_combine.conf

# hack to calculate the AF for the orignal ExAC population as well.
grep -B2 -A2 nonTCGA_af exac_combine.conf | perl -pe "s/--//g" | perl -pe "s/nonTCGA_//g" >> exac_combine.conf

cat << EOF >> exac_combine.conf
[[postannotation]]
fields=["af_AFR", "af_AMR", "af_EAS", "af_FIN", "af_NFE", "af_OTH", "af_SAS", "nonTCGA_af_AFR", "nonTCGA_af_AMR", "nonTCGA_af_EAS", "nonTCGA_af_FIN", "nonTCGA_af_NFE", "nonTCGA_af_OTH", "nonTCGA_af_SAS", "nonpysch_af_AFR", "nonpysch_af_AMR", "nonpysch_af_EAS", "nonpysch_af_FIN", "nonpysch_af_NFE", "nonpysch_af_OTH", "nonpysch_af_SAS"]
op="max"
name="max_af_ALL"
type="Float"
EOF
