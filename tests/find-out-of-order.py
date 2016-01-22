import sys

last_chrom = "XXX"
last = 0

for i, line in enumerate(sys.stdin):
    if line[0] == "#": continue
    toks = line.split("\t", 3)
    pos = int(toks[1])
    if pos < last and last_chrom == toks[0]:
        print "error at line: %d at %s:%d" % (i, toks[0], pos)
        raise Exception("OUT OF ORDER")
    last = pos
    last_chrom = toks[0]
