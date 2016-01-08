import sys
import gzip
import re

xopen = lambda f: (gzip.open if f.endswith(".gz") else open)(f)
vcf = xopen(sys.argv[1])
key = sys.argv[2]

patt = re.compile("ID=AN_(...),")
fields = []
for line in vcf:
    if line[0] != "#": break

    m = patt.search(line)
    if not m: continue
    group = m.groups()[0]
    fields.extend(["AC_{group}".format(**locals()),
                   "AN_{group}".format(**locals())])
    if group != "Adj":
        fields.extend([
                   "Hom_{group}".format(**locals()),
                   "Het_{group}".format(**locals())])

    # create a postannotation block for each population
    print """\
[[postannotation]]
fields=["{key}_AC_{group}", "{key}_AN_{group}"]
name="{key}_af_{group}"
op="div2"
type="Float"
""".format(**locals())

# create a single annotation that pulls all fields.
print """\
[[annotation]]
file="{vcf}"
fields=[{fields}]
ops=[{ops}]
names=[{names}]
""".format(vcf=sys.argv[1],
                          fields=",".join('"%s"' % f for f in fields),
                          ops=",".join(['"self"'] * len(fields)),
                          names=",".join('"%s_%s"' % (key, f) for f in fields))
