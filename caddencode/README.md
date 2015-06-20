caddencode
----------

CADD is a useful annotation for genomic variants. It provides a score for every
base-change in the genome (totalling 8.6 billion bases). The download from the
[cadd website](http://cadd.gs.washington.edu/) is 79GB and moderately difficult
to use to annotate a VCF.

`caddencode` encodes the CADD phred scores into an 11GB binary file with O(1)
access (and provides a means to annotate a VCF). It does this by encoding the
reference base using 2 bits (0: A, 1:C, 2:G, 3:T) a each of the 3 alternate alleles
using 10 bits (2^10 == 1024) for a total of **32 bits per site**.

Since the max phred score is 99 and we can store up to 1024 for each base, **the
loss of precision is bound to under 0.05** (because we multiply phred * 10.23 on input
and divide on output). So, if the real phred-score is 12.21, the recovered phred-score
is guaranteed to be between 12.16 and 12.26.

testing
-------

We have tested this extensively. In generating 20 million random
phred quartets (actually triplets), the maximum difference seen
between the real an decoded(encoded(data)) was 0.0488758515995.
This matches well with the theoretical maximum:
1 / (2 * 10.23) == 0.04887585532746823
