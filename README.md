vcfanno
=======

[![Build Status](https://travis-ci.org/brentp/vcfanno.svg)](https://travis-ci.org/brentp/vcfanno)
[![Coverage Status](https://coveralls.io/repos/brentp/vcfanno/badge.svg)](https://coveralls.io/r/brentp/vcfanno)



vcfanno annotates a VCF with any number of *sorted* input BED, BAM, and VCF files.
It does this by finding overlaps as it streams over the data and applying
user-defined operations on the overlapping fields.

For VCF, values are pulled by name from the INFO field.
For BED, values are pulled from (1-based) column number.
For BAM, only depth (`count`) is currently supported.


`vcfanno` is written in [go](http://golang.org)
It can annotate ~ 5,000 variants per second with 5 annotations from 3 files on a modest laptop.

We are actively developing `vcfanno` and appreciate feedback and bug reports.

Usage
=====

After downloading the [binary for your system](https://github.com/brentp/vcfanno/releases/tag/v0.0.2) (see section below) usage looks like:

```Shell
  ./vcfanno example/conf.toml example/query.vcf
```

Where config.toml looks like:

```
[[annotation]]
file="ExAC.vcf"
fields = ["AC_AFR", "AC_AMR", "AC_EAS"]
ops=["first", "first", "min"]

[[annotation]]
file="fitcons.bed"
columns = [4, 4]
names=["fitcons_mean", "js_sum"]
# note the 2nd op here is javascript that has access to `vals`
ops=["mean", "js:sum=0;for(i=0;i<vals.length;i++){sum+=vals[i]}; vals"]

[[annotation]]
file="example/ex.bam"
names=["ex_bam_depth"]
#count is currently the only valid option for a bam

```

So from `ExAC.vcf` we will pull the fields from the info field and apply the corresponding
`operation` from the `ops` array. Users can add as many `[[annotation]]` blocks to the
conf file as desired. Files can be local as above, or available via http/https.

Also see the additional usage section at the bottom for additional details.

Example
-------

the example directory contains the data and conf for a full example. To run, either download
the [appropriate binary](https://github.com/brentp/vcfanno/releases/tag/v0.0.2) for your system build with:

```Shell
go get
go build -o vcfanno
```

from this directory.
Then, you can annotate with:

```Shell
GOMAXPROCS=4 ./vcfanno example/conf.toml example/query.vcf > annotated.vcf
```

An example INFO field row before annotation (pos 98683):
```
AB=0.282443;ABP=56.8661;AC=11;AF=0.34375;AN=32;AO=45;CIGAR=1X;TYPE=snp
```

and after:
```
AB=0.2824;ABP=56.8661;AC=11;AF=0.3438;AN=32;AO=45;CIGAR=1X;TYPE=snp;AC_AFR=0;AC_AMR=0;AC_EAS=0;fitcons_mean=0.061;js_sum=0.061
```

Operations
==========

In most cases, we will have a single annotation entry for each entry (variant)
in the query VCF. However, it is possible that there will be multiple annotations
from a single annotation file--in this case, the op determines how the many values
are `reduced`. Valid operations are:

 + js:$javascript // see below for more details
 + mean
 + max
 + min
 + concat // comma delimited list of output
 + count  // count the number of overlaps
 + uniq
 + first 
 + flag   // presense/absence via vcf flag

Please open an issue, or use the javascript if your desired operation is not supported.

custom ops (javascript)
-----------------------

we embed the javascript engine [otto](https://github.com/robertkrimen/otto) so that it's 
possible to create a custom op if it is not provided. For example if the users wants to
output the sum of the values, then the op would be:

    "js:sum=0;for(i=0;i<vals.length;i++){sum+=vals[i]};sum"

where the last value (in this case sum) is returned as the annotation value. It is encouraged
to instead define javascript functions in a `js` tag in the config file, e.g. the above sum
would be:

	js="""
	function sum(vals) {
		isum=0
		for(i=0;i<vals.length;i++){
			isum += vals[i]
		}
		return isum
	}
	"""

(note the triple quotes). Then it would be used in an op as:

	"js:sum(vals)"

See [examples/conf.toml](https://github.com/brentp/vcfanno/blob/master/example/conf.toml)
for more examples.

Binaries
========

binary executables are available [here](https://github.com/brentp/vcfanno/releases/tag/v0.0.2)
for *linux*, *mac* (darwin), and *windows* for *32* and *64* bit platforms.

Preprocessing
=============

Annotations will be the most accurate if your query and annotation variants are split (no multiple ALTs) and normalize (left-aligned and
trimmed). At some point, this will be done internally, but for now, you can get a split and normalized VCF using [vt](https://github.com/atks/vt)
with:

```Shell
vt decompose -s $VCF | vt normalize -r $REF - > $NORM_VCF
```

Development
===========

This, and the associated go libraries ([vcfgo](https://github.com/brentp/vcfgo),
[irelate](https://github.com/brentp/irelate), [xopen](https://github.com/brentp/xopen)) are
under active development. The following are on our radar:

- [ ] decompose, normalize, and get allelic primitives for variants on the fly
      (we have code to do this, it just needs to be integrated)
- [x] improve test coverage for vcfanno
- [x] embed otto js engine to allow custom ops.
- [x] support for annotating BED files.


Additional Usage
================

-ends
-----

For annotating large variants, such as CNVs or structural variants (SVs), it can be useful to
annotate the *ends* of the variant in addition to the region itself. To do this, specify the `-ends`
flag to `vcfanno`. e.g.:
```Shell
vcfanno -ends example/conf.toml example/query.vcf
```
In this case, the names field in the *conf* file contains, "fitcons\_mean". The output will contain
`fitcons\_mean` as before along with `left\_fitcons\_mean` and `right\_fitcons\_mean` for any variants
that are longer than 1 base. The *left* end will be for the single-base at the lowest base of the variant
and the *right* end will be for the single base at the higher numbered base of the variant.

-permissive-overlap
-------------------

By default, when annotating with a variant, in addition to the overlap requirement, the variants must share
the same position, the same reference allele and at least one alternate allele (this is only used for
variants, not for BED/BAM annotations). If this flag is specified, only overlap testing is used and shared
REF/ALT are not required.

```Shell
vcfanno -permissive-overlap example/conf.toml example/query.vcf
```

<!--
 goxc -include example/,README.md -d /tmp/vcfanno/ -pv=0.0.1 -bc='linux,darwin,windows,!arm'
-->
