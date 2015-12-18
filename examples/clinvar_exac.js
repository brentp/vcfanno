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

function check_clinvar_aaf(clinvar_sig, max_aaf_all, aaf_cutoff){
	if (typeof clinvar_sig == "string") {
		return clinvar_sig.indexOf("pathogenic") != -1 && max_aaf_all > aaf_cutoff
    }
    clinvar_sig = clinvar_sig.join(",")
	return check_clinvar_aaf(clinvar_sig, max_aaf_all, aaf_cutoff)
}

function div(a, b) {
	if(a == 0){ return 0.0; }
	return (a / b).toFixed(9)
}
