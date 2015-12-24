CLINVAR_SIG = {}
CLINVAR_SIG["0"] = 'uncertain'
CLINVAR_SIG["1"] = 'not-provided'
CLINVAR_SIG["2"] = 'benign'
CLINVAR_SIG["3"] = 'likely-benign'
CLINVAR_SIG["4"] = 'likely-pathogenic'
CLINVAR_SIG["5"] = 'pathogenic'
CLINVAR_SIG["6"] = 'drug-response'
CLINVAR_SIG["7"] = 'histocompatibility'
CLINVAR_SIG["255"] = 'other'
CLINVAR_SIG["."] = '.'

function contains(str, tok)
	return string.find(str, tok) ~= nil
end

function intotbl(ud)
	local tbl = {}
	for i=1,#ud do
		tbl[i] = ud[i]
	end
	return tbl
end

-- from lua-users wiki
--[[
function split(str, sep)
        local sep, fields = sep or ":", {}
        local pattern = string.format("([^%s]+)", sep)
        str:gsub(pattern, function(c) fields[#fields+1] = c end)
        return fields
end
--]]
split = gosplit

function clinvar_sig(vals)
    local t = type(vals)
    -- just a single-value
    if(t == "string" or t == "number") and not contains(vals, "|") then
        return CLINVAR_SIG[vals]
    elseif t ~= "table" then
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
    return join(ret, ",")
end

join = table.concat

function check_clinvar_aaf(clinvar_sig, max_aaf_all, aaf_cutoff)
	if type(clinvar_sig) ~= "string" then
    	clinvar_sig = join(clinvar_sig, ",")
    end
	return contains(clinvar_sig, "pathogenic") and max_aaf_all > aaf_cutoff
end

function div(a, b)
	if(a == 0) then return "0.0" end
	return string.format("%.9f", a / b)
end
