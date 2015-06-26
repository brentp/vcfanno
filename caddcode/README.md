caddencode
----------

CADD is a useful annotation for genomic variants. It provides a score for every
base-change in the genome (totalling 8.6 billion bases). The download from the
[cadd website](http://cadd.gs.washington.edu/) is 79GB and moderately difficult
to use to annotate a VCF. It is free for **academic** use, please contact the
CADD creators for commercial use.

`caddencode` encodes the CADD phred scores into an 11GB binary file with O(1)
access (and provides a means to annotate a VCF). It does this by encoding the
reference base using 2 bits (0: A, 1:C, 2:G, 3:T) and the CADD phred score of a
change to each of the 3 alternate alleles using 10 bits each (2^10 == 1024) for
a total of **32 bits per site**.

We use a memory-mapped view of the binary file to provide very fast access. This
takes advantage of the OS cache especially when querying variants in genome order.

Since the max phred score is 99 and we can store up to 1024 for each base, **the
loss of precision is bound to under 0.05** (because we multiply phred * 10.23 on input
and divide on output). So, if the real phred-score is 12.21, the recovered phred-score
is guaranteed to be between 12.16 and 12.26.

download
--------

The index and the binary file are here:

 - https://s3.amazonaws.com/vcfanno/cadd_v1.2.idx
 - https://s3.amazonaws.com/vcfanno/cadd_v1.2.bin

The `vcfanno` conf file should point to the .idx file as in the [example](https://github.com/brentp/vcfanno/blob/master/example/conf.toml)
The conf file should be updated as:

```
[caddidx]
file="/path/to/cadd_v1.2.idx"
names=["cadd_phred_score"]
ops=["concat"]
```

annotation
----------

The user can define a reduction function (mean, max, concat, etc) to
apply to the resulting values...

- For single-base changes, we report the single cadd score. 

- For deletions where the alt is a single base, we report the list of changes from ref[i] to alt.
  e.g.  ref=CCGCCGTTGCAAAGGCGCGCCG, alt=C, the first 2 changes will be C->C and will have score 0.
  Each score will be calculated by it's position in the reference.

- For insertions, and other MNP's that don't have an alt of length 1. We report the 2 scores of the
  positions flanking the event.

We are open to suggestions on how to better handle MNPs.

testing
-------

We have tested this extensively. In generating 20 million random
phred quartets (actually triplets), the maximum difference seen
between the real an decoded(encoded(data)) was 0.0488758515995.
This matches well with the theoretical maximum:
1 / (2 * 10.23) == 0.04887585532746823

There are 3 positions on chromosome 3 that use an ambiguous reference and so store
4 base changes. In the file, we handle these by storing a change to C, T, G. In the
code, we handle this by storing the values in the actual code so that the guarantee
of a precision to within 0.05 is maintained.

standalone use
--------------

It's possible to get the cadd scores for arbitrary positions in the genome independent from
vcfanno. 

From this directory:

```Shell
$ go build main/cadd.go
```


Then use E.g:


```Shell
IDX=$CADD_PATH/cadd_v1.2.idx

$ ./cadd $IDX 1 10618 C
1.075268817204301 <nil>

$ ./cadd $IDX 22 100020618 T
2015/06/25 10:47:37 22 100020618 50944546
0 requested position out of range


#check:
$ tabix -s 1 -b 2 whole_genome_SNVs.tsv.gz 21:9411195-9411195
21	9411195	A	C	-0.006942	3.231
21	9411195	A	G	-0.005108	3.257
21	9411195	A	T	0.045036	3.997

$ ./cadd $IDX 21 9411195 C
3.225806451612903 <nil>
$ ./cadd $IDX 21 9411195 G
3.225806451612903 <nil>
$ ./cadd $IDX 21 9411195 T
4.007820136852395 <nil>
$ ./cadd $IDX 21 9411195 A
0 <nil>

```
Note that an error is return when a position is requested beyond the end of the chromosome.
The \<nil\> indicates that the query was successful.
A query for a change the the reference results in a score of 0 with no error.


This mode is very fast. When running:

```Shell
./cadd test $IDX > /dev/null
```

We see something like: `tested 5585322 sites (641787/second)`
