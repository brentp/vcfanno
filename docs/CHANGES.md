v0.2.7 (dev)
------------
+ restore multiple threads per annotation file after fix in biogo/hts (#64)

v0.2.6
------
+ fix race condition from 0.2.4 by reverting some speed improvements from that release.

v0.2.5
------
+ better error messages

v0.2.4
------
+ 30-40% speed improvement when annotating with few, large files by parallelizing decompression.
+ handle multiple values to div2 (mostly for multi-allelics)
+ if `_af` or `af_` is in the `name` then make Number=A for a `postannotation`.

v0.2.3
------
+ allow pulling the 'FILTER' field from a VCF by specifing 'FILTER' in the `fields`
+ add new op for postannotation only 'delete' which can be used to remove fields from the INFO.
+ use Description from annotation as Description in annotated file (thanks @xuyangy for the implementation #43)

v0.2.2
------
+ wholely fix order of error checking from postannotation (affects users using ID annotation) (thanks @vladsaveliev)

v0.2.1
------
+ correct order of error-checking on return from postannotation (affects users using ID annotation)

v0.2
----
+ proper support for multi-allelics in query and in annotation with `op="self"`
+ allow post-annotation to set ID column

v0.1.1
------
+ set number=1 when op=min see #40
+ fix when we get -0 converted to just '-' (#42)
+ when an annotation bed file has a header with 'ref' or 'reference' and 'alt' or 'alternate',
  then match on the ref and alt values defined in those columns (not just on position as for bed).
+ (vcfgo) allow double-digit number in header thanks @chapmanb
+ correct END position when using ref and alt from a file with only a position (infer end as start + len(ref)) [thanks Tomas].

v0.1.0
------
+ drastically reduce the warning message output.
+ add field=['DP2'] for bam files which reports number of forward and reverse reads overlapping a variant.
  value returned can be replicated with: `samtools mpileup -A -x -Q 1 -q 1 -r chrom:pos-pos $bam`
+ add op=sum
+ expose ref and alt to lua.


v0.0.11
-------
+ when op=self, pull Number from the annotation file (previously Number was always 1)
+ when op=concat or op=uniq user Number=. 
+ when name ends with '\_float' '\_int' or '\_flag' that is used to determine the Type in the output and that is then stripped from the name. So, e.g. with
``
names=["af_esp_aa_float"]
``
The resulting header will be:
```
##INFO=<ID=af_esp_aa,Number=1,Type=Float,Description=...
```
+ fix regression with CIPOS/CIEND when using `-ends` with structural variants. (Thanks to Liron for reporting)
+ fix bug with uniq not uniq-ing (Thanks Liron for reporting)
+ improve error messages from lua (Thanks Komal Rathi for the test-case provoking this change).
+ improve handling of multiple values (usually due to multiple alts) for `max` and `min`. Now, if we have, e.g. var1:SAS=0.22,0.33
  and var2:SAS=0.25 then `max` will give 0.33. This change makes max and min recursive so they'll call themselves on nested slices.
  Thanks @snashraf for reporting. 

v0.0.10
-------
+ allow using postannotation even if not all requested fields were found for a given variant.
+ restore ability to have bams as annotation files. Can pull mapq and coverage. See `examples/conf.toml`
+ fix regression where output was not in sorted order.
+ fix regression that resulted in "too many open files" error.
+ expand test-suite.
+ fix bug found when using max() op

v0.0.9
------
+ restore ability to take query file from STDIN (no tabix required).
+ fix memory leak. memory use now scales with number of procs (-p).
+ added new op 'self' which should be used for most cases when matching on ref and alt as it
  determines the type from the annotation header and uses that to update the annotated header
  with the correct type.
+ new [documentation site](http://brentp.github.io/vcfanno/)
+ [[postannotation]] allows modifying stuff in the query VCF after annotation (or instead).
  See examples on the documentation site.
+ convert scripting engine to lua from javascript
+ add CADD conversion script and example


v0.0.8
------
+ parallel chrom-sweep (removes problems with chromosome sort order).
  - as a result, files are required to be tabix'ed.
  - the chromosome sort order is no longer important.
+ fix bug in SV support of CIPOS, CIENDS
+ huge speed improvement (can annotate ~30K variants/second with 10 cpus).
+ remove server and cadd support (will return soon).
+ fix bug where header is not updated.
+ respect strict when -ends is used.


v0.0.7
------
+ better support for flags. e.g. can specify a flag from js by ending the function name with \_flag
+ [irelate] error if intervals are out of order within a file.
+ -base-path argument replaces basepath in .toml file
+ [vcfgo] report all headers in original file.
+ integrated server to host annotations
+ -ends argument will now use CIPOS and CIEND to annotate the left and right interval of an SV. If CIPOS
   and CIEND are undefined for a given interval, the ends will not be annotated.
+ for MNPs, cadd score is reported as a list of max values (of the 3 possible changes) for each reference base
  covered by the event.
+ fix bug in CADD annotation and provide CADD v1.3 download
+ ~25-30% speed improvement. from a modest laptop:  *annotated 10195872 variants in 28.97 minutes (351984.0 / minute)*

v0.0.6
------
+ [support for CADD](https://github.com/brentp/vcfanno/tree/master/caddcode)
+ concat defaults to | separator
+ speed improvements (vcfgo info field)
+ natural sort is default. use -lexographical to

v0.0.5
------
+ allow natural sort (1, 2, ... 9, 10, 11 instead of 1, 10, 11 ..., 19, 2, 20) via flag
+ vcfgo: handle lines longer than 65KB **major**
+ vcfgo: fix error reporting
+ irelate: report warning when chroms out of order

v0.0.4
------
+ performance improvements for Javascript ops with pre-compilation.
+ bam: annotate with `mapq` and `seq` for mapping-quality and sequence respectively.
+ api now returns a channel on which to recieve annotated Relatables
+ vcfgo: fix printing of INFO fields with multiple values (thanks to Liron for reporting).
+ vcfgo: fix writing of ##SAMPLE and ##PEDIGREE headers. (thanks to Liron)

v0.0.3
------
+ custom ops with javascript.
+ proper support for <CNV>, <INV>
+ option to annotate BED files.
+ vcfanno has an [api](https://godoc.org/github.com/brentp/vcfanno/tree/api) so it can be
  used from other progs. 
