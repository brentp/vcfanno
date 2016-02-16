import sys
import re
import numpy as np
from collections import defaultdict

groups = defaultdict(list)
for line in open(sys.argv[1]):
    gap, chunk, procs, info = re.split("\s+", line, 3)

    seconds = re.search("in (.+) seconds", info).groups(0)[0]
    if gap == '100' or chunk == '100': continue

    groups[(int(gap), int(chunk), int(procs))].append(float(seconds))

bychunk = defaultdict(list)
bygap = defaultdict(list)
bycpu = defaultdict(list)
for gap, chunk, cpu in groups:
    if cpu != 4: continue
    #if chunk != 5000: continue
    m = np.mean(groups[(gap, chunk, cpu)])
    groups[(gap, chunk, cpu)] = m
    bychunk[chunk].append((gap, m))
    bygap[gap].append((chunk, m))
    bycpu[cpu].append((gap, m))

from matplotlib import pyplot as plt
import seaborn as sns
sns.set_palette('Set1', len(groups))

for chunk, vals in sorted(bychunk.items()):
    vals.sort()
    xs, ys = zip(*vals)
    plt.plot(xs, ys, label="chunk-size: %d" % chunk)
    print chunk, vals

"""
for gap, vals in sorted(bygap.items()):
    vals.sort()
    xs, ys = zip(*vals)
    plt.plot(xs, ys, label="gap-size: %d" % gap)

for cpu, vals in sorted(bycpu.items()):
    vals.sort()
    xs, ys = zip(*vals)
    plt.plot(xs, ys, label="cpus: %d" % cpu)

"""

plt.xlabel("gap size")
#plt.xlabel("chunk size")
plt.ylabel("time (seconds)")
#plt.yscale('log', basey=2)
#plt.xscale('log', basex=10)
plt.legend()
plt.show()
1/0
for g, c in sorted(groups):
    if g == '100' or c == '100': continue
    print g, c, groups[(g, c)]

