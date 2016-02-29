# Timing table (downloads data for other plots too)
python timing.py # writes timing.txt which is the table.

# Figure 4. (number of cores vs speedup for ExAC and 1KG)
bash parallelization-run.sh
python parallelization-figure.py

# Figure 5A. (gap-size vs chunk-size)
python chunk-gap-plot.py  1kg.times-tails.fmt.txt exac.times-tails.txt
