package main

import (
	"fmt"
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
		} else if f, ok := v.(float64); ok {
			s = append(s, fmt.Sprintf("%f", f))
		} else if anint, ok := v.(int); ok {
			s = append(s, fmt.Sprintf("%d", anint))
		} else {
			s = append(s, fmt.Sprintf("%v", v))
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
