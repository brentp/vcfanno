"""
If we annotate ExAC with ExAC, we can test that all variants are annotated with
themselves
"""

import toolshed as ts
from hts import Tbx
import sys

tbx = Tbx(sys.argv[1])
fh = ts.nopen(1)

fields = ["AC_Adj", "AN_Adj", "AC_AFR", "AN_AFR", "AC_AMR", "AN_AMR", "AN_EAS",
        "AC_EAS", "AN_FIN", "AN_NFE", "AN_OTH", "AN_SAS", "AC_FIN", "AC_NFE",
        "AC_OTH", "AC_SAS"]
for toks in ts.reader(fh, header=False):
    if toks[0] == "#CHROM": break
toks[0] = 'CHROM'

for d in ts.reader(fh, header=toks):

    info = {x[0]: x[1] for x in (v.split("=") for v in d['INFO'].split(";") if '=' in v)}

    for f in fields:
        # if it's not the same, it overlapped many sites.
        if info[f] != info["exac" + f]:
            assert info[f] in info["exac" + f].split("|"), (d, f, info[f], info["exac" + f])
            r = list(tbx("%s:%d-%d" % (d['CHROM'], int(d['POS']) - 1, int(d['POS']) + len(d['REF']) - 1)))
            if len(r) < 2:
                print (d['CHROM'], d['POS'], d['REF'], d['ALT'], f, info[f], info["exac" + f])
                break

