go build main/cadd.go
IDX=$CADD_PATH/cadd_v1.2a.idx

$ ./cadd $IDX 1 10618 C
1.075268817204301 <nil>

$ ./cadd $IDX 22 100020618 T
2015/06/25 10:47:37 22 100020618 50944546
0 requested position out of range

$ ./cadd $IDX 21 9411195 C
3.225806451612903 <nil>
$ ./cadd $IDX 21 9411195 G
3.225806451612903 <nil>
$ ./cadd $IDX 21 9411195 T
3.225806451612903 <nil>
$ ./cadd $IDX 21 9411195 A
0.0 <nil>

check:
$ tabix -s 1 -b 2 whole_genome_SNVs.tsv.gz 21:9411195-9411195
21	9411195	A	C	-0.006942	3.231
21	9411195	A	G	-0.005108	3.257
21	9411195	A	T	0.045036	3.997

