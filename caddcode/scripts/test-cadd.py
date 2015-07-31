"""
compare what we get from tabix with what we get from the encoded CADD stuff
and make sure it is < 0.05 difference
"""
import sys
import toolshed as ts
import os.path as op

cadd_prefix = sys.argv[1]
tsv = sys.argv[2]

assert op.exists("%s.idx" % cadd_prefix)
assert op.exists("%s.bin" % cadd_prefix)

slen = {x[0]: int(x[1]) for x in (l.rstrip().split() for l in open(cadd_prefix + ".idx"))}


def read_cadd(path, chrom, pos):
    cmd = "cadd {path} {chrom} {pos} {letter}"
    cmds = []
    d = locals()
    for letter in "ACGT":
        d['letter'] = letter
        cmds.append(cmd.format(**d))
    cmd = " && ".join(cmds)
    vals = [x.strip().split() for x in ts.nopen('|' + cmd)]
    for i in range(len(vals)):
        vals[i][0] = float(vals[i][0])
    return dict(zip("ACGT", vals))

def read_tsv(path, chrom, pos):
    cmd = "tabix {path} {chrom}:{pos}-{pos}"
    res = [x.strip().split() for x in ts.nopen("|%s" % cmd.format(**locals()))]
    v = {r[3]: float(r[5]) for r in res}
    for k in "ACGT":
        if not k in v:
            v[k] = 0.0
    return v

def runner(chrom, length):
    step = max(2, length/10000)
    for i, pos in enumerate(range(10000, length, step)):

        cadd = read_cadd(cadd_prefix + ".idx", chrom, pos)
        tabx = read_tsv(tsv, chrom, pos)
        for letter in "ACGT":

            if abs(cadd[letter][0] - tabx[letter]) > 0.05:
                print "BAD:", chrom, pos, cadd, tabx
                1/0

        else:
            if i % 100 == 0:
                print "%s %d/%d ok" % (chrom, pos, length)

list(ts.pmap(runner, ((chrom, length) for chrom, length in slen.items() if chrom.isdigit() or chrom in "XY"), n=12))
