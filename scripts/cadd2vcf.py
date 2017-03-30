from __future__ import print_function
import locale

def _to_str(s, enc=locale.getpreferredencoding()):
    if isinstance(s, bytes):
        return s.decode(enc)
    return s

import toolshed as ts
def main(precision, path):
    header = None

    tmpl = "{chrom}\t{pos}\t.\t{ref}\t{alt}\t1\tPASS\traw={rawscore:.%if};phred={phred:.%if}" % (precision, precision)

    hdr = """\
##fileformat=VCFv4.1
##INFO=<ID=raw,Number=1,Type=Float,Description="raw cadd score">
##INFO=<ID=phred,Number=1,Type=Float,Description="phred-scaled cadd score">
##CADDCOMMENT=<ID=comment,comment="{comment}">
#CHROM	POS	ID	REF	ALT	QUAL	FILTER INFO"""

    for i, line in enumerate(ts.nopen(path)):
        line = _to_str(line)
        if i == 0:
            print(hdr.format(comment=line.strip("# ").strip()))
            continue
        if header is None and line.startswith("#Chrom"):
            header = line[1:].lower().rstrip().split("\t")
            continue
        d = dict(zip(header, line.rstrip().split("\t")))
        d['phred'] = float(d['phred'])
        d['rawscore'] = float(d['rawscore'])
        print(tmpl.format(**d))




if __name__ == "__main__":

    import argparse
    p = argparse.ArgumentParser()
    p.add_argument("--precision", type=int, default=1, help="amount of precision to keep")
    p.add_argument("path")
    a = p.parse_args()

    main(a.precision, a.path)

