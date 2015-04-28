vcfanno
=======

vcfanno annotates a VCF with any number of *sorted* input BED, BAM, and VCF files.
It does this by finding overlaps as it streams over the sorted files and applying
user-defined operations on the overlapping fields.

For VCF annotations, values are pulled by name from the INFO field. A variant from
the query VCF will only be annotated with a variant from an annotation file if they
have the same position and REF and share at least 1 ALT.

For BED files, values are pulled from (1-based) column number.

For BAM files, only depth (`count`) is currently supported.


`vcfanno` is written in [go](http://golang.org) and will make use of multiple CPU's
if your environment variable `GOMAXPROCS` is set to a number greater than 1. It can
annotate ~ 5,000 variants per second with 5 annotations from 3 files on a modest laptop.

We are actively developing `vcfanno` and appreciate your feedback as we navigate the
[fruit salad](https://www.biostars.org/p/7126/#7136) of the VCF format.

Usage
=====

Usage looks like:

    vcfanno config.toml $input.vcf > $annotated.vcf

Where config.toml contains the information on any number of annotation files.
Example entries look like

```
[[annotation]]
file="ExAC.vcf"
fields = ["AC_AFR", "AC_AMR", "AC_EAS"]
ops=["first", "first", "min"]

[[annotation]]
file="fitcons.bed"
columns = [4]
names=["fitcons_mean"]
ops=["mean"]

[[annotation]]
file="example/ex.bam"
names=["ex_bam_depth"]
#count is currently the only valid option for a bam

```

So from `ExAC.vcf` we will pull the fields from the info field and apply the corresponding
`operation` from the `ops` array. Users can add as many `[[annotation]]` blocks to the
conf file as desired.

Example
-------

the example directory contains the data and conf for a full example. To run, either download
the appropriate binary for your system from **TODO** or build with:

```Shell
go build -o vcfanno
```

from this directory.
Then, you can annotate with:

```Shell
./vcfanno example/conf.toml example/query.vcf > annotated.vcf
```
Or, to get the result a bit sooner:

```Shell
GOMAXPROCS=4 ./vcfanno example/conf.toml example/query.vcf > annotated.vcf
```

An example INFO field row before annotation (pos 98683):
```
AB=0.282443;ABP=56.8661;AC=11;AF=0.34375;AN=32;AO=45;CIGAR=1X;TYPE=snp
```

and after:
```
AB=0.2824;ABP=56.8661;AC=11;AF=0.3438;AN=32;AO=45;CIGAR=1X;TYPE=snp;AC_AFR=0;AC_AMR=0;AC_EAS=0;fitcons_mean=0.061
```

Operations
==========

In most cases, we will have a single annotation entry for each entry (variant)
in the query VCF. However, it is possible that there will be multiple annotations
from a single annotation file--in this case, the op determines how the many values
are `reduced`. Valid operations are:

 + mean
 + max
 + min
 + concat
 + count
 + uniq
 + first

Please open an issue if your desired operation is not supported.

Binaries
========

binary executables are available for *linux*, *mac* (darwin), and *windows* for *32* and *64* bit
platforms.

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

Again, this, along with the associated go libraries ([vcfgo](https://github.com/brentp/vcfgo),
[irelate](https://github.com/brentp/irelate), [xopen](https://github.com/brentp/xopen)) are
under active development. A number of things are not yet supported and a number of features
will be added soon.

- [ ] add flag op. just check for presence/overlap with annotation.
- [x] strip 'chr' prefix from chroms to prevent lack of overlap due to different names.
- [x] handle structural variants correctly. (SVLEN <DEL/DUP> / <INS> [len=0])
- [ ] decompose, normalize, and get allelic primitives for variants on the fly
      (we have code to do this, it just needs to be integrated)
- [ ] improve test coverage for vcfanno (started, but needs more)
- [x] correct order of contigs from vcf writer.
- [ ] embed v8 to allow custom ops.

<!--
 goxc -include example/,README.md -d /tmp/vcfanno/ -pv=0.0.1 -bc='linux,darwin,windows,!arm'
-->
