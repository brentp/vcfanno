import sys

import numpy as np
import pandas as pd
from matplotlib import pyplot as plt
import seaborn as sns

sns.set_style('white')

colors = sns.color_palette('Set2', 3)

df = pd.read_table(sys.argv[1])

df = df[df.i == 9]
print df

fig, ax = plt.subplots(figsize=(12, 6))

#bt_rects = ax.bar([0], df.seconds[df.method == "bedtools"], color=colors[0])
#bt_rects = ax.bar([1], df.seconds[df.method == "bcftools"], color=colors[1])
bt_rects = ax.bar(range(4), df.seconds[(df.method == "vcfanno")],
        color=colors[2])

ax.set_xticks(0.5 + np.arange(4))
ax.set_xticklabels(df.procs[df.method == "vcfanno"])

ax.axhline(y=list(df.seconds[df.method == "bedtools"])[0], color=colors[0], label="bedtools")
ax.axhline(y=list(df.seconds[df.method == "bcftools"])[0], color=colors[1], label="bcftools")

ax.set_ylabel('time in seconds')
ax.set_xlabel('cores used by vcfanno')
plt.legend()
plt.show()
