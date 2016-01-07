Performance Tips
================

Following the [golang](https://golang.org) philosophy, there are few knobs
to turn for tweaking performance.

Processes
---------

The simplest performance tweak is to use `-p PROCS` to specify the number
of processes to use when running `vcfanno`. It scales well to ~15 processes.

Garbage Collection
------------------

On machines with even modest amounts of memory, it can be good to allow
`go` to use more memory for the benefit of spending less time in garbage
collection. Users can do this by preceding their `vcfanno` command with
`GOGC=1000`. Where higher values allow `go` to use more memory and the
default value is 100. For example:

```
GOGC=2000 vcfanno -p 12 a.conf a.vcf
```

Max Gap Size
------------

The parallel chrom-sweep algorithm has a gap size parameter that determines
when a chunk of records from the the query file is sent to be annotated. If
a gap of a certain
size is encountered, a new chunk is sent off. Given a (number of) dense
annotation file(s), it might be good to reduce the gap size so that `vcfanno`
will need to parse fewer unneeded records. However, given sparse annotation
sets, it is best to have this value be large so that each annotation worker
gets enough work to keep it busy.

The default gap size is `20000` bases. Users can alter this using the
environment variable `IRELATE_MAX_GAP` e.g.:

```
IRELATE_MAX_GAP=5000 vcfanno -p 12 a.conf a.vcf
```

