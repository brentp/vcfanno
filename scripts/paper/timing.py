import sys
import os
import time

import toolshed as ts

files = [
        dict(file="ESP6500SI.all.snps_indels.tidy.v2.vcf.gz",
             fields=["EA_AC", "AA_AC"],
             names=["esp_ea", "esp_aa"],
             ops=["first", "first"]),

        dict(file="ExAC.r0.3.sites.vep.tidy.vcf.gz",
             fields=["AC_Adj", "AC_Het", "AC_Hom", "AC_NFE"],
             names=["exac_AC_Adj", "exac_AC_Het", "exac_AC_Hom", "exac_AC_NFE"],
             ops=["first", "first", "first", "first"]),

        dict(file="hg19_fitcons_fc-i6-0_V1-01.bed.gz",
             columns=[4],
             names=["fitcons_mean"],
             ops=["mean"],
             h="fitcons.hdr",
             c="CHROM,FROM,TO,FITCONS"),

        dict(file="ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites.tidy.vcf.gz",
             fields=["EAS_AF", "AMR_AF"],
             names=["1kg_eas_af", "1kg_amr_af"],
             ops=["first", "first"]),

        dict(file="GRCh37-gms-mappability.vcf.gz",
             fields=["GMS_illumina"],
             names=["gms_mappapility"],
             ops=["first"]),

        dict(file="clinvar_20150305.tidy.vcf.gz",
             fields=["CLNHGVS"],
             names=["clinvar_hgvs"],
             ops=["first"]),

        dict(file="cosmic-v68-GRCh37.tidy.vcf.gz",
             fields=["ID"],
             names=["cosmic_id"],
             ops=["concat"]),

        dict(file="dbsnp.b141.20140813.hg19.tidy.vcf.gz",
             fields=["RS"],
             names=["dbsnp_id"],
             ops=["concat"]),

        dict(file="hg19.gerp.elements.bed.gz",
             columns=[4],
             names=["gerp"],
             ops=["mean"],
             h="gerp.hdr",
             c="CHROM,FROM,TO,GERP"),
        ]


def asanno(d):
    c = "fields={fields}" if "fields" in d else "columns={columns}"
    tmpl = """
[[annotation]]
file="{file}"
names={names}
ops={ops}
""" + c + "\n"
    return tmpl.format(**d)

def asbcf(d):
    d['cols'] = "+" + ",+".join(d['fields']) if 'fields' in d else d["c"]
    d['hdr'] = "-h " + d['h'] if 'h' in d else ""
    return 'bcftools annotate -a {DATA}/{file} -c "{cols}" {hdr}'.format(**d)

with open('fitcons.hdr', 'w') as fh:
    fh.write('##INFO=<ID=FITCONS,Number=1,Type=Float,Description="FITCONS VALUE">\n')

with open('gerp.hdr', 'w') as fh:
    fh.write('##INFO=<ID=GERP,Number=1,Type=Float,Description="GERP VALUE">\n')


toml = "compare.toml"
with open(toml, "w") as fh:
    for d in files:
        fh.write(asanno(d))

DATA = '/usr/local/src/gemini_install/data/gemini_data/'
DATA= '/data2/gemini_install/data/gemini/data/'

QUERY = "/usr/local/src/gocode/src/github.com/brentp/vcfanno/t.vcf.gz"
QUERY = "{DATA}/ExAC.r0.3.sites.vep.tidy.vcf.gz".format(DATA=DATA)
DATA_SORTED = "/data2/brentp/gemini_sorted/" # TODO: bedtools requires all to be sorted in same way and

for f in files:
    f['DATA'] = DATA
    f['DATA_SORTED'] = DATA_SORTED
    f['QUERY'] = QUERY

commands = [asbcf(f) for f in files]

query = QUERY.format(DATA=DATA)
fnames = [f['DATA_SORTED'] + "/" + f['file'] for f in files]
bedtools_cmd = ("bedtools intersect -sorted -sortout -wao -a {query} -b " + " -b ".join(fnames)).format(query=query)

# TODO: send to file to match bcftools and vcfanno
print "method\ti\tseconds\tprocs"
t = time.time()
list(ts.nopen("|%s | bgzip -c > /tmp/trash.gz" % bedtools_cmd))
print "bedtools\t%d\t%.2f\t1" % (len(commands), time.time() - t)

for procs in (1, 4, 8, 12):

    if procs == 1:
        tottime = 0
        for i in range(len(commands)):
            out = "tmp%d.vcf.gz" % i
            try:
                os.unlink("tmp%d.vcf.gz" % (i - 2))
            except OSError:
                pass
            query = QUERY.format(DATA=DATA) if i == 0 else ("tmp%d.vcf.gz" % (i - 1))
            cmd = commands[i] + " {query} | bgzip -c > {out}; tabix {out} ".format(DATA=DATA, query=query, out=out)
            print >>sys.stderr, cmd

            t = time.time()

            res = list(ts.nopen("|%s" % cmd))

            t1 = time.time()
            tottime += t1 - t
        print "bcftools\t%d\t%.2f\t1" % (i+1, tottime)
        sys.stdout.flush()

    vcmd = "vcfanno -p {procs} -base-path {DATA} {toml} {QUERY} | bgzip -c > /dev/null".format(
            DATA=DATA, procs=procs, QUERY=QUERY, toml=toml)
    print >>sys.stderr, vcmd
        #print vcmd
    t = time.time()

    res = list(ts.nopen("|%s" % vcmd))
    t1 = time.time()
    print "vcfanno\t%d\t%.2f\t%d" % (i+1, t1 - t, procs)


