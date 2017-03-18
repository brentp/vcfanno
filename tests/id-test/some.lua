
function setid(...)
    local t = {...}
    local res = {}
    for i, v in ipairs(t) do
        if v ~= "." and v ~= nil and v ~= "" then
            res[#res+1] = string.gsub(v, ",", ";")
        end
    end
    return table.concat(res, ";")
end
