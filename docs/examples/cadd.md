Annotate with CADD
==================

This documents how to use `vcfanno` to annotate a VCF with CADD scores.

*"CADD is a tool for scoring the deleteriousness of single nucleotide variants as well as insertion/deletions variants in the human genome."*
Users of CADD should refer to the [web page](http://cadd.gs.washington.edu/info) for citation and use requirements.

Setup
-----

In order to use CADD with `vcfanno`, we must convert the tsv format provided by CADD to VCF. We can do this with [a
python script](https://github.com/brentp/vcfanno/blob/master/scripts/cadd2vcf.py). After downloading the *All possible SNVs of GRCh37/hg19* filre from the [CADD website](http://cadd.gs.washington.edu/download) we can run as:

```
python cadd2vcf.py whole_genome_SNVs.tsv.gz | bgzip -c > cadd_v1.3.vcf.gz
```

This will create a 50GB vcf.gz from the 80GB tsv.gz. 
The entirety of the INFO field for a given record will look like: "raw=0.34;phred=6.05"

Config
------

From here, we can specify a conf file:

```
[[annotation]]
file="cadd_v1.3.vcf.gz"
names=["cadd_phred", "cadd_raw"]
ops=["mean", "mean"]
fields=["phred", "raw"]
```

Annotation
----------

And we can run `vcfanno` as:

```
vcfanno -p 12 cadd.conf query.vcf > query.anno.vcf
```

As an extreme case, we can run this on the ExAC VCF:

```Shell
vcfanno -p 18 cadd.conf ExAC.r0.3.sites.vep.tidy.vcf.gz | bgzip -c > /tmp/exac-cadd.vcf.gz
```

This takes about 88 minutes on a good server. This time will improve in future versions but it
is due to the large number of lines that must be parsed from the CADD VCF, even with the algorithm
that allows it to avoid parsing annotation intervals that fall in large gaps in the query. By
comparison, `bedtools intersect -sorted` takes 92 minutes for this same overlap.

Note
----

This will only work for single-nucleotide variants since the default for VCF is to match on REF and ALT.

