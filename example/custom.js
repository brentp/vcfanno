function mean(vals) {
    sum=0
    for(i=0;i<vals.length;i++){
        sum += vals[i]
    }
    return sum / vals.length
}

function loc(chrom, start, end){
    return chrom + ":" + start + "-" + end
}
