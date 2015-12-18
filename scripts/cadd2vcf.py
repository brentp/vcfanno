import toolshed as ts
def main(precision, path):

    tmpl = "{Chrom}\t{Pos}\t.\t{Ref}\t{Alt}\t1\tPASS\traw={RawScore:.%if};phred={PHRED:.%if}" % (precision, precision)

    hdr = """\
##fileformat=VCFv4.1
##INFO=<ID=raw,Number=1,Type=Float,Description="raw cadd score">
##INFO=<ID=phred,Number=1,Type=Float,Description="phred-scaled cadd score">
##CADDCOMMENT=<ID=comment,comment="{comment}">
#CHROM	POS	ID	REF	ALT	QUAL	FILTER INFO"""

    for i, line in enumerate(ts.nopen(path)):
        if i == 0:
            print hdr.format(comment=line.strip("# ").strip())
            continue
        if line.startswith("#Chrom"):
            header = line[1:].rstrip().split("\t")
            continue
        d = dict(zip(header, line.rstrip().split("\t")))
        d['PHRED'] = float(d['PHRED'])
        d['RawScore'] = float(d['RawScore'])
        print tmpl.format(**d)




if __name__ == "__main__":

    import argparse
    p = argparse.ArgumentParser()
    p.add_argument("--precision", type=int, default=1, help="amount of precision to keep")
    p.add_argument("path")
    a = p.parse_args()

    main(a.precision, a.path)

