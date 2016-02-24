import toolshed as ts

lookup = {'ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites': '1000G',
          'ExAC.r0.3.sites.vep': 'ExAC'}

data = {'1000G': [], 'ExAC': []}

"""
method	procs	time	query
var	20	888.29 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
var	19	897.02 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
var	18	895.35 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
var	17	909.24 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
var	16	916.43 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
var	15	945.61 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
var	14	981.14 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
var	13	1051.26 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
var	12	1126.58 seconds	ALL.wgs.phase3_shapeit2_mvncall_integrated_v5a.20130502.sites
"""
for d in ts.reader(1):
    time = float(d['time'].split()[0])
    key = lookup[d['query']]
    data[key].append(time)


for k in data:
    data[k] = data[k][::-1]

from matplotlib import pyplot as plt
import seaborn as sns
sns.set_style('white')

from matplotlib import rcParams
rcParams['font.family'] = 'Arial'
rcParams['font.size'] = 18

N = len(data.values()[0])
N = 16

markers = 'os'
for j, k in enumerate(data):
    values = data[k]
    plt.plot(range(1, N + 1), [values[0] / values[i] for i in range(N)],
             markers[j] + "-", label=k)

plt.ylabel("Speed-up relative to 1 process")
plt.xlabel("Number of processes")
plt.plot(range(1, N + 1), [i for i in range(1, N + 1)], '--',
         c="0.78", lw=2)
plt.tight_layout()


plt.legend(loc="upper left")

ax = plt.gca()
for item in ([ax.title, ax.xaxis.label, ax.yaxis.label] +
              ax.get_xticklabels() + ax.get_yticklabels()):
        item.set_fontsize(16)
for item in ax.get_legend().get_texts():
    item.set_fontsize(13)

plt.xlim(xmin=1, xmax=N+0.16)
plt.ylim(ymin=1, ymax=8)
sns.despine(left=True, bottom=True)
plt.savefig('vcfanno-par.png')
plt.show()

