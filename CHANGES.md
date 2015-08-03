v0.0.8
------
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
