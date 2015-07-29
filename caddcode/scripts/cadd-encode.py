import sys
import itertools as it

# max value in cadd phred is 100. We have 10 bits per number (1023 max)
# 1023 / 100.
MULT3 = 10.23

def encode3(A, C, G, T, mult=MULT3):
    missing = [A, C, G, T].index(None)
    x, y, z = [int(l * mult + 0.5) for l in (A, C, G, T) if l is not None]
    # when decoding, we use the first 2 bits to determine which base is missing
    # (reference base).
    return missing | ((x << 2) | ((y << 12) | (z << 22)))

def decode3(num, mult=MULT3):
    """
    >>> decode3(encode3(0.17, None, 0.222, 0.991))
    [0.19550342130987292, None, 0.19550342130987292, 0.9775171065493645]

    >>> decode3(encode3(None, 17, 22, 9))
    [None, 17.008797653958943, 21.994134897360702, 8.993157380254154]

    >>> decode3(encode3(1.7, 2.2, None, 9.0))
    [1.6617790811339197, 2.2482893450635384, None, 8.993157380254154]

    >>> decode3(encode3(99., 99., 99.0, None))
    [99.02248289345063, 99.02248289345063, 99.02248289345063, None]
    """
    missing = num % 4
    off = (2**10)-1
    vals = [((num >> 2) & off) / mult,
            ((num >> 12) & off) / mult,
            ((num >> 22) & off) / mult]
    vals.insert(missing, None)
    return vals

def test():
    import random
    def phreds():
        phreds = [random.random() * 99.0 for _ in range(3)]
        missing = random.randint(0, 3)
        phreds.insert(missing, None)
        return phreds
    def check_diff(actual, decoded):
        diffs = []
        for i, a in enumerate(actual):
            if a is None:
                assert decoded[i] is None
            else:
                diffs.append(a - decoded[i])
        return diffs

    diffs = []
    for i in range(20000000):
        actual = phreds()
        decoded = decode3(encode3(*actual))
        diffs.extend(check_diff(actual, decoded))

    actual = [99.0, 99.9, 99.1, None]
    decoded = decode3(encode3(*actual))
    diffs.extend(check_diff(actual, decoded))

    print max(diffs)

if __name__ == "__main__":
    import doctest
    doctest.testmod()
    if sys.argv[1] == "test":
        sys.exit(test())

    import array
    import operator as op

    sin = (x.rstrip().split("\t", 5) for x in sys.stdin if not x[0] == "#")
    pairs = []
    last_chrom = None
    last_pos = None
    zero = encode3(None, 0, 0, 0)

    prefix = sys.argv[1]
    fh_bin = open(prefix + ".bin", "w")
    fh_idx = open(prefix + ".idx", "w")

    cache = []
    for pos, grp in it.groupby(sin, op.itemgetter(0, 1, 2)):
        grp = list(grp)
        if len(grp) != 3:
            # http://www.boekhoff.info/?pid=data&dat=fasta-codes
            assert len(grp) == 4
            print >>sys.stderr, grp
            sys.stderr.flush()

            if grp[0][2] in ("R", "M"):  # R: A, C, G: A, G
                # just drop the A
                grp = grp[1:]

            else:
                1/0

        a, b, c = grp
        pos = int(a[1])

        if a[0] != last_chrom:
            print a[0]
            if len(cache) > 0:
                array.array('I', cache).write(fh_bin)
                fh_idx.write("%s\t%d\n" % (last_chrom, len(cache)))
                fh_idx.flush()
                cache = []

            cache.extend([zero] * pos)
            last_pos = pos - 1
            last_chrom = a[0]

        # skipped regions.
        if pos - 1 != last_pos:
            sys.stderr.write("writing %d empties at %s:%d-%d\n" % (pos - 1 - last_pos, a[0], last_pos, pos))
            cache.extend([zero] * (pos - 1 - last_pos))

        assert pos == len(cache)
        phred = {a[3]: float(a[5]),
                 b[3]: float(b[5]),
                 c[3]: float(c[5])}

        phred_encoded = encode3(phred.get('A'), phred.get('C'),
                                phred.get('G'), phred.get('T'))
        cache.append(phred_encoded)
        last_pos = pos

    array.array('I', cache).write(fh_bin)
    fh_idx.write("%s\t%d\n" % (last_chrom, len(cache)))
