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

CLINVAR_SIG = {'0': 'uncertain',
               '1': 'not-provided',
               '2': 'benign',
               '3': 'likely-benign',
               '4': 'likely-pathogenic',
               '5': 'pathogenic',
               '6': 'drug-response',
               '7': 'histocompatibility',
               '255': 'other'}

function clinvar_pathogenic(vals){
	for(i=0;i<vals.length;i++){
		if(vals[i] == 5){
			return true
		}
	}
	console.log("checked")
	return false
}
