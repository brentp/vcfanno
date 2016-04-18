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

function intotbl(ud)
    local tbl = {}
    for i=1,#ud do
        tbl[i] = ud[i]
    end
    return tbl
end


function clinvar_sig(vals)

    local t = type(vals)
    -- just a single-value
    if(t == "string" or t == "number") and not string.find(vals, "|") ~= nil then
        return CLINVAR_SIG[vals]
    else
        if not contains(t, "userdata") then
            vals = {vals}
        else
            vals = intotbl(vals)
        end
    end
    local ret = {}
    for i=1,#vals do
        if not contains(vals[i], "|") then
            ret[#ret+1] = CLINVAR_SIG[vals[i]]
        else
            local invals = split(vals[i], "|")
            local inret = {}
            for j=1,#invals do
                inret[#inret+1] = CLINVAR_SIG[invals[j]]
            end
            ret[#ret+1] = join(inret, "|")
        end
    end
    return ret.concat(",")
end

