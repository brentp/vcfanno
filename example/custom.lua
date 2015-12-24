function mean(vals)
    local sum=0
	for i=1,#vals do
		sum = sum + vals[i]
	end
	return sum / #vals
end

function loc(chrom, start, stop)
    return chrom .. ":" .. start .. "-" .. stop
end

CLINVAR_LOOKUP = {}
CLINVAR_LOOKUP['0'] = 'unknown'
CLINVAR_LOOKUP['1'] = 'germline'
CLINVAR_LOOKUP['2'] = 'somatic'
CLINVAR_LOOKUP['4'] = 'inherited'
CLINVAR_LOOKUP['8'] = 'paternal'
CLINVAR_LOOKUP['16'] = 'maternal'
CLINVAR_LOOKUP['32'] = 'de-novo'
CLINVAR_LOOKUP['64'] = 'biparental'
CLINVAR_LOOKUP['128'] = 'uniparental'
CLINVAR_LOOKUP['256'] = 'not-tested'
CLINVAR_LOOKUP['512'] = 'tested-inconclusive'
CLINVAR_LOOKUP['1073741824'] = 'other'

CLINVAR_SIG = {}
CLINVAR_SIG['0'] = 'uncertain'
CLINVAR_SIG['1'] = 'not-provided'
CLINVAR_SIG['2'] = 'benign'
CLINVAR_SIG['3'] = 'likely-benign'
CLINVAR_SIG['4'] = 'likely-pathogenic'
CLINVAR_SIG['5'] = 'pathogenic'
CLINVAR_SIG['6'] = 'drug-response'
CLINVAR_SIG['7'] = 'histocompatibility'
CLINVAR_SIG['255'] = 'other'
CLINVAR_SIG['.'] = '.'

function clinvar_sig(vals) 
    local t = type(vals)
	-- just a single-value
    if(t == "string" or t == "number") then
		return CLINVAR_SIG[vals]
	else
        vals = {vals}
	end
    local ret = {}
    for i=1,#vals do
        if(index(vals[i], "|") == -1) then
            ret[#ret+1] = CLINVAR_SIG[vals[i]]
        else
            local invals = split(vals[i], "|")
            local inret = {}
            for j=1,#invals do
                inret[#inret+1] = CLINVAR_SIG[invals[j]]
			end
			ret[#ret+1] = table.concat(inret, "|")
		end
    end
    return table.concat(ret, ",")
end


function div(a, b)
	if(a == 0) then return 0.0 end
	return (a / b).toFixed(9)
end
