function gff_to_gene_name(infos, name_field)
    if infos == nil or #infos == 0 then
        return nil
    end
    local result = {}

    for i=1,#infos do
        info = infos[i]
        local s, e = string.find(info, name_field .. "=")
        if s ~= nil then
            name = info:sub(e + 1)
            s, e = string.find(name, ";")
            if e == nil then
                e = name:len()
            else
                e = e - 1
            end

            result[name:sub(0, e)] = 1
        end
    end
    local keys = {}
    for k, v in pairs(result) do
        keys[#keys+1] = k
    end
    return table.concat(keys,",")
end
