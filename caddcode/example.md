**NOTE** that you can annotate a VCF directly with CADD using [vcfanno](https://github.com/brentp/vcfanno)
but `caddencode` can also be used directly.

In most cases, users will want to download the full encoded data set for the [index](https://s3.amazonaws.com/vcfanno/cadd_v1.2a.idx) and
[bin file](https://s3.amazonaws.com/vcfanno/cadd_v1.2a.bin) (11GB)

# build the index

```Shell
$ python scripts/cadd-encode.py cadd.sub < cadd.sub.txt
```

# run a test to pull out values in order order (uses OS cache via mmmap) many times:

```Shell
$ go run cadd.go test cadd.sub.idx
2015/06/19 09:25:56 tested 4615 sites (96348/second)
```

so it can pull values for nearly 100K sites per second

We can pull a single value:

```Shell
$ go run cadd.go cadd.sub.idx 1 18050 C
11.241446725317692
```

And verify it matches the original from the text file:
```Shell
$ grep -P $'1\t18050' cadd.sub.txt | grep -w C
1	18050	A	C	0.745955	11.27
```

