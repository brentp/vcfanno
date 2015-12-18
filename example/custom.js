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
               '255': 'other',
               '.': '.'}

function clinvar_sig(vals) {
	var t = typeof vals
	// just a single-value
	if(t == "string" || t == "number") {
		return CLINVAR_SIG[vals]
	}
	var ret = []
    // handle '|' and ','
	for(i=0;i<vals.length;i++){
		if(vals[i].indexOf("|") == -1) {
			ret.push(CLINVAR_SIG[vals[i]])
		} else {
			invals = vals[i].split("|")
			inret = []
			for(j=0;j<invals.length;j++){
				inret.push(CLINVAR_SIG[invals[j]])
			}
			ret.push(inret.join("|"))
		}
	}
	return ret.join(",")
}

function clinvar_pathogenic_flag(vals){
	for(i=0;i<vals.length;i++){
		if(vals[i] == 5){
			return true
		}
	}
	return false
}

function check_clinvar_aaf(clinvar_sig, max_aaf_all, aaf_cutoff){
	if (typeof clinvar_sig == "string") {
		return clinvar_sig.indexOf("pathogenic") != -1 && max_aaf_all > aaf_cutoff
    }
    clinvar_sig = clinvar_sig.join(",")
	return check_clinvar_aaf(clinvar_sig, max_aaf_all, aaf_cutoff)
}


function clinvar_likely_pathogenic_flag(vals){
	for(i=0;i<vals.length;i++){
		if(vals[i] == 4){
			return true
		}
	}
	return false
}

function clinvar_histocompatibility_flag(vals){
	for(i=0;i<vals.length;i++){
		if(vals[i] == 7){
			return true
		}
	}
	return false
}

function div2(vals){
	if(vals.length != 2){
		return "BAD"
	}
	denom = 0 + vals[0] + vals[1]
	if(denom == 0){
		return 0
	}
	return vals[1] / denom
}

function clinvar_drug_response_flag(vals){
	for(i=0;i<vals.length;i++){
		if(vals[i] == 6){
			return true
		}
	}
	return false
}

function div(a, b) {
	if(a == 0){ return 0.0; }
	return (a / b).toFixed(9)
}
