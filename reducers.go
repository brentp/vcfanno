package main

import (
	"fmt"
	"github.com/brentp/irelate"
	"strconv"
	"strings"
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
		s = append(s, v.(string))
	}
	return strings.Join(s, ",")
}

func count(vals []interface{}) interface{} {
	return len(vals)
}

func uniq(vals []interface{}) interface{} {
	var m map[interface{}]bool
	var uvals []interface{}
	for _, v := range vals {
		if _, ok := m[v]; !ok {
			m[v] = true
			uvals = append(uvals, v)
		}
	}
	return uvals
}

func first(vals []interface{}) interface{} {
	if len(vals) < 1 {
		return nil
	}
	return vals[0]
}

// Collect the fields associated with a variant into a single slice.
func Collect(v *irelate.Variant, cfg anno, src uint32) [][]interface{} {
	annos := make([][]interface{}, len(cfg.Names))
	for _, b := range v.Related() {
		if b.Source()-uint32(1) != src {
			continue
		}
		// VCF
		if o, ok := b.(*irelate.Variant); ok {
			// Is checks for same allele.
			if v.Is(o.Variant) {
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
		} else if iv, ok := b.(*irelate.Interval); ok {
			for i := range cfg.Names {
				op := cfg.Ops[i]
				val := iv.Fields[cfg.Columns[i]-1]
				if op != "concat" && op != "uniq" && op != "count" && op != "first" {
					v, e := strconv.ParseFloat(val, 32)
					if e != nil {
						panic(e)
					}
					annos[i] = append(annos[i], float32(v))
				} else {
					annos[i] = append(annos[i], val)
				}
			}
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
	"min":    Reducer(max),
	"concat": Reducer(concat),
	"count":  Reducer(count),
	"uniq":   Reducer(uniq),
	"first":  Reducer(first),
}
