Combine ExAC with non-psych and nonTCGA
=======================================

ExAC originally released a VCF that contained the aggregate data for all samples.
It has INFO fields for `AFR`, `AMR`, `EAS`, `FIN`, `NFE`, `OTH`, and `SAS` populations
along with the combined `Adj` count.
Recently, 2 additional VCF's that contain only non pyschiatric and only non TCGA samples
have been released.

Here, we will use `vcfanno` to decorate the original VCF with the alternate counts, 
chromosome counts, number of hets, number of hom alts, and allele frequencies of all populations
from each of the 2 additional VCFs. Since the original VCF contains only allele counts, we will
also use `vcfanno` to calculate the allele frequency for the original, full population. Finally,
we'll calculate the maximum allele frequency from all populations [as that is very useful for
filtering](http://quinlanlab.org/blog/2015/12/17/geminisummary.html).

Creating a `conf` file can be cumbersome, but in this case, it's simple, if repetitive. As such
we write a [**script**](https://github.com/brentp/vcfanno/tree/master/docs/examples/exac_combine/exac_combine_mkconf.py) to create a block like this:

```
[[annotation]]
file="ExAC.r0.3.nonpsych.sites.tidy.vcf.gz"
fields=["AC_AFR","AN_AFR","Hom_AFR","Het_AFR","AC_AMR","AN_AMR","Hom_AMR","Het_AMR","AC_Adj","AN_Adj","AC_EAS","AN_EAS","Hom_EAS","Het_EAS","AC_FIN","AN_FIN","Hom_FIN","Het_FIN","AC_NFE","AN_NFE","Hom_NFE","Het_NFE","AC_OTH","AN_OTH","Hom_OTH","Het_OTH","AC_SAS","AN_SAS","Hom_SAS","Het_SAS"]
ops=["self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self","self"]
names=["nonpsych_AC_AFR","nonpsych_AN_AFR","nonpsych_Hom_AFR","nonpsych_Het_AFR","nonpsych_AC_AMR","nonpsych_AN_AMR","nonpsych_Hom_AMR","nonpsych_Het_AMR","nonpsych_AC_Adj","nonpsych_AN_Adj","nonpsych_AC_EAS","nonpsych_AN_EAS","nonpsych_Hom_EAS","nonpsych_Het_EAS","nonpsych_AC_FIN","nonpsych_AN_FIN","nonpsych_Hom_FIN","nonpsych_Het_FIN","nonpsych_AC_NFE","nonpsych_AN_NFE","nonpsych_Hom_NFE","nonpsych_Het_NFE","nonpsych_AC_OTH","nonpsych_AN_OTH","nonpsych_Hom_OTH","nonpsych_Het_OTH","nonpsych_AC_SAS","nonpsych_AN_SAS","nonpsych_Hom_SAS","nonpsych_Het_SAS"]
```

And a similar one for `nonTCGA`. Then, we create a section like this:

```
[[postannotation]]
fields=["nonpsych_AC_NFE", "nonpsych_AN_NFE"]
name="nonpsych_af_NFE"
op="div2"
type="Float"
```

for each population and for the original superset as well as for nonpsych, nonTCGA. The script to create all this automatically is [here](https://github.com/brentp/vcfanno/tree/master/docs/examples/exac_combine/exac_combine_mkconf.py)

Then we have a final section that calculates the maximum allele frequency among all populations:

```
[[postannotation]]
fields=["af_AFR", "af_AMR", "af_EAS", "af_FIN", "af_NFE", "af_OTH", "af_SAS", "nonTCGA_af_AFR", "nonTCGA_af_AMR", "nonTCGA_af_EAS", "nonTCGA_af_FIN", "nonTCGA_af_NFE", "nonTCGA_af_OTH", "nonTCGA_af_SAS", "nonpysch_af_AFR", "nonpysch_af_AMR", "nonpysch_af_EAS", "nonpysch_af_FIN", "nonpysch_af_NFE", "nonpysch_af_OTH", "nonpysch_af_SAS"]
op="max"
name="max_af_ALL"
type="Float"
```

The entire conf file is [here](https://github.com/brentp/vcfanno/tree/master/docs/examples/exac_combine/exac_combine.conf)

Then, the `vcfanno` command is very simple:

```
vcfanno -p 8 exac_combine.conf ExAC.r0.3.sites.vep.tidy.vcf.gz | bgzip -c > ExAC.r0.3.all.tidy.vcf.gz
```

This runs in about 15 minutes (10K variants / second) on good server with fast disk.
