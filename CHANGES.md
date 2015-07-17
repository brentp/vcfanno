v0.0.7 (development)
--------------------
+ better support for flags. e.g. can specify a flag from js by ending the function name with \_flag
+ [irelate] error if intervals are out of order within a file.
+ -base-path argument replaces basepath in .toml file

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
