package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brentp/irelate"
)

type Reducer func([]interface{}) interface{}

func mean(vals []interface{}) interface{} {
	s := float32(0.0)
	for _, v := range vals {
		s += asfloat32(v)
	}
	return s / float32(len(vals))
}

func max(vals []interface{}) interface{} {
	imax := float32(-999999999999999.0)
	for _, v := range vals {
		vv := asfloat32(v)
		if vv > imax {
			imax = vv
		}
	}
	return imax
}

func asfloat32(i interface{}) float32 {
	switch i.(type) {
	case uint32:
		return float32(i.(uint32))
	case uint64:
		return float32(i.(uint64))
	case int:
		return float32(i.(int))
	case float32:
		return i.(float32)
	case float64:
		return float32(i.(float64))
	}
	return i.(float32)
}

func min(vals []interface{}) interface{} {
	imin := float32(999999999999999.0)
	for _, v := range vals {
		vv := asfloat32(v)
		if vv < imin {
			imin = vv
		}
	}
	return imin
}

func concat(vals []interface{}) interface{} {
	var s []string
	for _, v := range vals {
		if v == nil {
			continue
		}
		if str, ok := v.(string); ok {
			s = append(s, str)
		} else {
			s = append(s, fmt.Sprintf("%d", v))
		}
	}
	return strings.Join(s, ",")
}

func count(vals []interface{}) interface{} {
	return len(vals)
}

func uniq(vals []interface{}) interface{} {
	m := make(map[interface{}]bool)
	var uvals []interface{}
	for _, v := range vals {
		if _, ok := m[v]; !ok {
			m[v] = true
			uvals = append(uvals, v)
		}
	}
	return concat(uvals)
}

func first(vals []interface{}) interface{} {
	if len(vals) < 1 {
		return nil
	}
	return vals[0]
}

// named vflag because of conflict with builtin.
func vflag(vals []interface{}) interface{} {
	return len(vals) > 0
}

// don't need to use chrom since we are only testing things
// returned from irelate.IRelate.
func overlap(a, b irelate.Relatable) bool {
	return b.Start() < a.End() && a.Start() < b.End()
}

// Collect the fields associated with a variant into a single slice.
func Collect(iv *irelate.Variant, rels []irelate.Relatable, cfg anno, strict bool) [][]interface{} {
	annos := make([][]interface{}, len(cfg.Names))
	v := iv.Variant
	for _, b := range rels {
		// VCF
		if o, ok := b.(*irelate.Variant); ok {
			// Is checks for same allele. but if it's not strict, we just check for overlap.
			if (strict && v.Is(o.Variant)) || (!strict && overlap(iv, b)) {
				for i := range cfg.Names {
					val := o.Info[cfg.Fields[i]]
					if arr, ok := val.([]interface{}); ok {
						annos[i] = append(annos[i], arr...)
					} else if val == nil {
						continue
					} else {
						annos[i] = append(annos[i], val)
					}
				}
			}
			// BED
		} else if iv, ok := b.(*irelate.Interval); ok {
			for i := range cfg.Names {
				val := iv.Fields[cfg.Columns[i]-1]
				if cfg.isNumber(i) {
					v, e := strconv.ParseFloat(val, 32)
					if e != nil {
						panic(e)
					}
					annos[i] = append(annos[i], float32(v))
				} else {
					annos[i] = append(annos[i], val)
				}
			}
			// BAM
		} else if bam, ok := b.(*irelate.Bam); ok {
			//if bam.MapQ() < 0 || bam.Flags&(0x4|0x8|0x100|0x200|0x400|0x800) != 0 {
			if bam.MapQ() < 1 || bam.Flags&(0x4) != 0 {
				continue
			}
			// currently, only valid op for bam is count
			for i := range cfg.Names {
				annos[i] = append(annos[i], 1)
			}
		} else {
			panic(fmt.Sprintf("type not supported for %v", b))
		}
	}
	if len(annos) > 0 {
		return annos
	}
	return nil
}

var Reducers = map[string]Reducer{
	"mean":   Reducer(mean),
	"max":    Reducer(max),
	"min":    Reducer(min),
	"concat": Reducer(concat),
	"count":  Reducer(count),
	"uniq":   Reducer(uniq),
	"first":  Reducer(first),
	"flag":   Reducer(vflag),
}

// Partition separates the Related() elements by source.
func Partition(a irelate.Relatable, nSources int) [][]irelate.Relatable {
	sep := make([][]irelate.Relatable, nSources)
	for _, r := range a.Related() {
		s := r.Source() - 1 // use -1 because we never include query
		if sep[s] == nil {
			sep[s] = make([]irelate.Relatable, 1)
			sep[s][0] = r
		} else {
			sep[s] = append(sep[s], r)
		}
	}
	return sep
}
