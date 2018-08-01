Annotate with CADD
==================

This documents how to use `vcfanno` to annotate a VCF with CADD scores.

*"CADD is a tool for scoring the deleteriousness of single nucleotide variants as well as insertion/deletions variants in the human genome."*
Users of CADD should refer to the [web page](http://cadd.gs.washington.edu/info) for citation and use requirements.

Config
------

From here, we can specify a conf file:

```
[[annotation]]
file="whole_genome_SNVs.tsv.gz"
names=["cadd_raw", "cadd_phred"]
ops=["mean", "mean"]
columns=[4, 5]
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

