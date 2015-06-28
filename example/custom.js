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

CLINVAR_LOOKUP = {'0': 'unknown',
                  '1': 'germline',
                  '2': 'somatic',
                  '4': 'inherited',
                  '8': 'paternal',
                  '16': 'maternal',
                  '32': 'de-novo',
                  '64': 'biparental',
                  '128': 'uniparental',
                  '256': 'not-tested',
                  '512': 'tested-inconclusive',
                  '1073741824': 'other'}
CLINVAR_SOURCE = {'0': 'unknown',
                  '1': 'germline',
                  '2': 'somatic',
                  '4': 'inherited',
                  '8': 'paternal',
                  '16': 'maternal',
                  '32': 'de-novo',
                  '64': 'biparental',
                  '128': 'uniparental',
                  '256': 'not-tested',
                  '512': 'tested-inconclusive',
                  '1073741824': 'other'}


