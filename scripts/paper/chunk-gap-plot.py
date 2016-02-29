import sys
import re
import numpy as np
from collections import defaultdict

from matplotlib import pyplot as plt
import seaborn as sns
sns.set_style("white")
colors = sns.set_palette('Set1', 8)
colors = sns.color_palette('Set1', 3)

f, axes = plt.subplots(1, figsize=(4, 2))
axes = (axes,)

# run as  python chunk-gap-plot.py  1kg.times-tails.fmt.txt exac.times-tails.txt

for i, f in enumerate(sys.argv[1:3]):
    if i == 0:
        assert "1kg" in f.lower()
    else:
        assert "exac" in f.lower()

    groups = defaultdict(list)
    for line in open(f):
        gap, chunk, procs, info = re.split("\s+", line, 3)

        if not int(chunk) in (1000, 10000, 100000): continue

        seconds = re.search("in (.+) seconds", info).groups(0)[0]
        if gap == '100' or chunk == '100': continue
        if int(procs) != 4: continue

        groups[(int(gap), int(chunk))].append(float(seconds))

    bychunk = defaultdict(list)
    for gap, chunk in groups:
        #if chunk != 5000: continue
        m = np.mean(groups[(gap, chunk)])
        bychunk[chunk].append((gap, m))

    label = "ExAC" if i == 1 else "1KG"
    marker = "o" if label == "ExAC" else "s"

    for j, (chunk, vals) in enumerate(sorted(bychunk.items())):
        vals.sort()
        xs, ys = zip(*vals)
        plabel = "%d : %s" % (chunk, label)
        if i == 1:
            plabel = label
        axes[0].plot(xs, ys, color=colors[j], ls="--" if label == "ExAC" else
                "-", label=plabel) #, marker=marker)

    if i == 0:
        axes[0].set_xlabel("Gap size")
    axes[0].set_ylabel("Time (seconds)")

sns.despine()
plt.legend(ncol=2, markerfirst=False, title="Chunk size",
        loc=(axes[0].get_position().x1-0.45, axes[0].get_position().y1 - 0.085))

ax = plt.gca()
for item in ([ax.title, ax.xaxis.label, ax.yaxis.label] +
              ax.get_xticklabels() + ax.get_yticklabels()):
    item.set_fontsize(7)
for item in ax.get_legend().get_texts():
    item.set_fontsize(5)


plt.savefig('figure-5.pdf')
plt.show()
