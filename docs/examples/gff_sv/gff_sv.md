Annotating with GFF
===================

GFF often contains the location of genes and features.
It has a column that contains key=value pairs like the INFO
field in a VCF. `vcfanno` does not support these natively,
but one can access particular fields using lua. This is an example of
how to do this.

Given a variant in `sample.vcf` like:
```
22	1000	DEL000SUR	N	<DEL>	.	PASS	END=1051060
```

And genomic features annotated in `genes.gff3.gz`:

```
22	ensembl_havana	gene	69091	70008	.	+	.	ID=ENST00000335137;geneID=ENSG00000186092;gene_name=OR4F5
22	ensembl	mRNA	182393	184158	.	+	.	ID=ENST00000624431;geneID=ENSG00000279928;gene_name=FO538757.3
22	ensembl	mRNA	184923	195411	.	-	.	ID=ENST00000623834;geneID=ENSG00000279457;gene_name=FO538757.2
22	ensembl	mRNA	184925	195411	.	-	.	ID=ENST00000623083;geneID=ENSG00000279457;gene_name=FO538757.2
22	ensembl	mRNA	184927	200322	.	-	.	ID=ENST00000624735;geneID=ENSG00000279457;gene_name=FO538757.2
22	ensembl_havana	mRNA	450740	451678	.	-	.	ID=ENST00000426406;geneID=ENSG00000278566;gene_name=OR4F29
22	ensembl_havana	mRNA	685716	686654	.	-	.	ID=ENST00000332831;geneID=ENSG00000273547;gene_name=OR4F16
22	havana	mRNA	924880	939291	.	+	.	ID=ENST00000420190;geneID=ENSG00000187634;gene_name=SAMD11
22	havana	mRNA	925150	935793	.	+	.	ID=ENST00000437963;geneID=ENSG00000187634;gene_name=SAMD11
...
```

We can write a conf file [gff_sv.conf](https://github.com/brentp/vcfanno/blob/master/docs/examples/gff_sv/gff_sv.conf)
```
[[annotation]]
file="genes.gff3.gz"
columns=[9]
names=["gene"]
ops=["lua:gff_to_gene_name(vals, 'gene_name')"]
```

This will extract the full text of the 9th column. Then we can use some lua to get just the `gene_name` field.
The lua uses a table so each observed gene is reported only once.
This is in [gff_sv.lua](https://github.com/brentp/vcfanno/blob/master/docs/examples/gff_sv/gff_sv.lua)


Then we can run `vcfanno` as:
```
vcfanno -lua gff_sv.lua gff_sv.conf sample.vcf
```

and `gene=FO538757.3,OR4F29,FO538757.2,OR4F16,OR4F5` is added to the INFO. The lua allows the user to do more complex things like only reporting genes, or taking uniq values.
