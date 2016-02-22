function af_conf_int(n_alts, n_total)
	-- eqn 4 from: http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.408.7107&rep=rep1&type=pdf
	p = n_alts / n_total
	n = n_total
    z = 1.96
    q = 1 - p

    a = 2*n*p + z^2
    denom = 2 * (n + z^2)

	bL = z * math.sqrt(z^2 - 2 - 1.0 / n + 4*p*(n*q+1))
	L = (a - 1 - bL) / denom

	bU = z * math.sqrt(z^2 + 2 - 1.0 / n + 4*p*(n*q-1))
	U = (a + 1 + bU) / denom
    return math.max(L, 0.0)
end
