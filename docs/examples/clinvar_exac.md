Annotate Clinvar With ExAC
==========================

The [ExAC paper](http://biorxiv.org/content/early/2015/10/30/030338) notes that
some of the variants in [ClinVar](http://www.ncbi.nlm.nih.gov/clinvar/intro/) that 
are classified as pathogenic (or likely pathogenic) are actually in high enough (>1%)
allele frequency in ExAC to indicate that it is unlikely that these are really pathogenic.

Here, we will use `vcfanno` to annotate the clinvar VCF with the allele frequencies
for ExAC so that we can find variants that are indicated as pathogenic **and** rare in ExAC.

The ExAC reports the alternate counts and the total number of chromosomes (`AN*`) and the
alternate allele counts (`AC*`) so, to we will annotate with those and then use `postannotation`
in `vcfanno` to get the `AF` as `AC/AN`. We will use
an already [decomposed and normalized](http://www.ncbi.nlm.nih.gov/pubmed/25701572) version of
ExAC (but vcfanno will match on any of the alternate alleles if multiple are present for a given
variant). The `[[annotation]]` section in the config file will look like this:

ExAC Config
-----------

```
[[annotation]]
file="ExAC.r0.3.sites.vep.tidy.vcf.gz"
fields = ["AC_Adj", "AN_Adj", "AC_AFR", "AN_AFR", "AC_AMR", "AN_AMR", "AC_EAS", "AN_EAS", "AC_FIN", "AN_FIN", "AC_NFE", "AN_NFE", "AC_OTH", "AN_OTH", "AC_SAS", "AN_SAS"]
names = ["ac_exac_all", "an_exac_all", "ac_exac_afr", "an_exac_afr", "ac_exac_amr", "an_exac_amr", "ac_exac_eas", "an_exac_eas", "ac_exac_fin", "an_exac_fin", "ac_exac_nfe", "an_exac_nfe", "ac_exac_oth", "an_exac_oth", "ac_exac_sas", "an_exac_sas"]
ops=["self", "self", "self", "self", "self", "self", "self", "self", "self", "self", "self", "self", "self", "self", "self", "self"]
```


Note that we can have as many of these sections as we want with vcfanno, but here we are only
interested in annotating clinvar with the single ExAC file. The `fields` section indicates which
fields to pull from the `ExAC` VCF. The `names` section indicates how those fields will be named
as they are added to the clinvar VCF. Since we intend to match on REF and ALT, there will only
be 1 match so the `op` is just "self" for all fields.

Because we want to know the allele frequency, we will need to divide `AC` by `AN`. This is done in a `[[postannotation]]`
section that looks like this:

PostAnnotation
--------------

```
[[postannotation]]
fields=["ac_exac_all", "an_exac_all"]
name="af_exac_all"
op="div2"
type="Float"
```

We need one of these section for each population, which is onerous, but simple enough to generate with
a small script. Note that the op `div` is provided by `vcfanno`, but we could have written this as a
custom op in lua as:

```lua
function div(a, b)
    if(a == 0){ return 0.0; }
    return string.format("%.9f", a / b)
}
```
and then use:
```
op="lua:div(ac_exac_all, an_exac_all)"
```

in the `[[postannotation]]`.

These `postannotation` sections are executed in the order they are specified so we can specify a final section that
takes the maximum of all of the allele frequencies. This is informative as a truly pathogenic variant should have a
low allele frequency in all populations. Here is the section to take the maximum AF of all the populations which
we've already calculated:

```
[[postannotation]]
fields=["af_exac_all", "af_exac_afr", "af_exac_amr", "af_exac_eas", "af_exac_nfe", "af_exac_oth", "af_exac_sas"]
op="max"
name="max_aaf_all"
type="Float"
```

Flag Common Pathogenic
----------------------

Finally, we can flag variants that have a `max_aaf_all` above some cutoff and are labelled as pathogenic.
```
[[postannotation]]
fields=["clinvar_sig", "max_aaf_all"]
op="lua:check_clinvar_aaf(clinvar_sig, max_aaf_all, 0.005)"
name="common_pathogenic"
type="Flag"
```

Note that we use 0.005 as the allele frequency cutoff. For any variant that was not present in ExAC, the `max_aaf_all` field
will be absent from the INFO field and so this will not be called.

If we've saved this in a file called `exac-af.conf` then the vcfanno command looks like:

```
vcfanno -lua clinvar_exac.lua -p 4 -base-path $EXAC_DIR clinvar_exac.conf $CLINVAR_VCF > $CLINVAR_ANNOTATED_VCF
```

This command finishes in about 2 minutes on a good laptop with a core i7 processor.

An example INFO field from the clinvar file after annotation looks like this:
```
RS=17855739;RSPOS=5831840;RV;dbSNPBuildID=123;SSR=0;SAO=1;VP=0x050060000a05150136110100;GENEINFO=FUT6:2528;WGT=1;VC=SNV;PM;NSM;REF;ASP;VLD;G5;GNO;KGPhase1;KGPhase3;LSD;OM;CLNALLE=1;CLNHGVS=NC_000019.9:g.5831840C>T;CLNSRC=OMIM_Allelic_Variant;CLNORIGIN=1;CLNSRCID=136836.0001;CLNSIG=5;CLNDSDB=MedGen:OMIM;CLNDSDBID=C3151219:613852;CLNDBN=Fucosyltransferase_6_deficiency;CLNREVSTAT=single;CLNACC=RCV000017626.26;CAF=0.8393,0.1607;COMMON=1;ac_exac_all=10114;an_exac_all=121354;ac_exac_afr=3093;an_exac_afr=10402;ac_exac_amr=449;an_exac_amr=11572;ac_exac_eas=867;an_exac_eas=8638;ac_exac_fin=210;an_exac_fin=6612;ac_exac_nfe=2836;an_exac_nfe=66712;ac_exac_oth=62;an_exac_oth=906;ac_exac_sas=2597;an_exac_sas=16512;af_exac_all=0.0833;af_exac_afr=0.2973;af_exac_amr=0.0388;af_exac_eas=0.1004;af_exac_nfe=0.0425;af_exac_oth=0.0684;af_exac_sas=0.1573;max_aaf_all=0.2973;clinvar_sig=pathogenic;common_pathogenic
```
(NOTE that clinvar has applied the `COMMON=1` tag here indicating a high AF in 1kg.)

With our ExAC fields appearing at the end:

```
ac_exac_all=10114;an_exac_all=121354;ac_exac_afr=3093;an_exac_afr=10402;ac_exac_amr=449;an_exac_amr=11572;ac_exac_eas=867;an_exac_eas=8638;ac_exac_fin=210;an_exac_fin=6612;ac_exac_nfe=2836;an_exac_nfe=66712;ac_exac_oth=62;an_exac_oth=906;ac_exac_sas=2597;an_exac_sas=16512;af_exac_all=0.0833;af_exac_afr=0.2973;af_exac_amr=0.0388;af_exac_eas=0.1004;af_exac_nfe=0.0425;af_exac_oth=0.0684;af_exac_sas=0.1573;max_aaf_all=0.2973;clinvar_sig=pathogenic;common_pathogenic
```
So this variant was classified as pathogenic, but has a `max_aaf_all` of 0.2973 and so it received the 'common_pathogenic' flag as did 566 other clinvar variants.

While the config file used to generate this final dataset was fairly involved, each step is very simple and it shows the power in vcfanno.
However, note that for most analyses, it will be sufficient to specify a config file that pulls the fields of
interest.

Supporting Files
----------------
The full config and lua files used to run this analysis are available [here](https://github.com/brentp/vcfanno/tree/master/docs/examples/).
