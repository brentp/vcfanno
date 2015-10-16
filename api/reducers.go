package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/vcfgo"
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
	case string:
		f, err := strconv.ParseFloat(i.(string), 32)
		if err != nil {
			return float32(0)
		}
		return float32(f)
	case []int:
		v := i.([]int)
		if len(v) == 1 {
			return asfloat32(v[0])
		}
	case []float32:
		v := i.([]float32)
		if len(v) == 1 {
			return v[0]
		}

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

func count(vals []interface{}) interface{} {
	return len(vals)
}

func _strings(vals []interface{}, uniq bool) []string {
	s := make([]string, 0)
	var m map[string]bool
	if uniq {
		m = make(map[string]bool)
	}
	for _, v := range vals {
		if v == nil {
			continue
		}
		if str, ok := v.(string); ok {
			if !uniq {
				s = append(s, str)
			} else if _, ok := m[str]; !ok {
				s = append(s, str)
			}

		} else if arr, ok := v.([]interface{}); ok {
			sub := make([]string, len(arr))
			for i, a := range arr {
				sub[i] = fmt.Sprintf("%s", a)
			}
			str := strings.Join(sub, ",")
			if !uniq {
				s = append(s, str)
			} else if _, ok := m[str]; !ok {
				s = append(s, str)
			}
		} else if arr, ok := v.([]string); ok {
			str := strings.Join(arr, ",")
			if !uniq {
				s = append(s, str)
			} else if _, ok := m[str]; !ok {
				s = append(s, str)
			}

		} else {
			str := vcfgo.ItoS("", v)
			if !uniq {
				s = append(s, str)
			} else if _, ok := m[str]; !ok {
				s = append(s, str)
			}
		}
	}
	return s
}

func uniq(vals []interface{}) interface{} {
	return strings.Join(_strings(vals, true), "|")
}

func concat(vals []interface{}) interface{} {
	return strings.Join(_strings(vals, false), "|")
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
func overlap(a, b interfaces.IPosition) bool {
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
