package api

import (
	"fmt"
	"log"
	"math"
	"reflect"
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

func dp2(vals []interface{}) interface{} {
	ret := make([]int, 2)
	for _, v := range vals {
		if v.(bool) {
			ret[1]++
		} else {
			ret[0]++
		}
	}
	return ret
}

func sum(vals []interface{}) interface{} {
	s := float32(0.0)
	for _, v := range vals {
		s += asfloat32(v, sum)
	}
	return s
}

func div2(vals []interface{}) interface{} {
	if vals[0] == 0 {
		return 0
	}
	return asfloat32(vals[0]) / asfloat32(vals[1])
}

func max(vals []interface{}) interface{} {
	imax := float32(-math.MaxFloat32)
	for _, v := range vals {
		vv := asfloat32(v, max)
		if vv > imax {
			imax = vv
		}
	}
	return imax
}

// asfloat32 returns a float32 from a single value. If there is
// more than 1 value then it calls reducer_func(vals).
func asfloat32(i interface{}, reducer_func ...Reducer) float32 {
	l_arr := -1
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
		l_arr = len(v)
	case []string:
		v := i.([]string)
		if len(v) == 1 {
			return asfloat32(v[0])
		}
		l_arr = len(v)
	case []float32:
		v := i.([]float32)
		if len(v) == 1 {
			return v[0]
		}
		l_arr = len(v)
		if l_arr == 2 && len(reducer_func) == 1 {
			fn := reducer_func[0]
			return fn([]interface{}{v[0], v[1]}).(float32)
		}

	}

	if l_arr > 0 && len(reducer_func) == 1 {
		fn := reducer_func[0]
		return fn(to_interface_slice(i)).(float32)
	}

	log.Fatalf("FATAL: tried to call asfloat32('%+v'). This usually means you have multiple alts and need to decompose or call max() or min() ", i)
	return i.(float32)
}

func to_interface_slice(a interface{}) []interface{} {
	s := reflect.ValueOf(a)
	if s.Kind() != reflect.Slice {
		panic("InterfaceSlice() given a non-slice type")
	}

	b := make([]interface{}, s.Len())
	for i := range b {
		b[i] = s.Index(i).Interface()
	}
	return b
}

func min(vals []interface{}) interface{} {
	imin := float32(math.MaxFloat32)
	for _, v := range vals {
		vv := asfloat32(v, min)
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
				m[str] = true
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
				m[str] = true
				s = append(s, str)
			}
		} else if arr, ok := v.([]string); ok {
			str := strings.Join(arr, ",")
			if !uniq {
				s = append(s, str)
			} else if _, ok := m[str]; !ok {
				m[str] = true
				s = append(s, str)
			}

		} else {
			str := vcfgo.ItoS("", v)
			if !uniq {
				s = append(s, str)
			} else if _, ok := m[str]; !ok {
				m[str] = true
				s = append(s, str)
			}
		}
	}
	return s
}

// delete is not used but we need it for a place-holder.
func delete(vals []interface{}) interface{} {
	panic("do not use")
	return nil
}

func uniq(vals []interface{}) interface{} {
	return strings.Join(_strings(vals, true), ",")
}

func concat(vals []interface{}) interface{} {
	return strings.Join(_strings(vals, false), ",")
}

func first(vals []interface{}) interface{} {
	if len(vals) < 1 {
		return nil
	}
	return vals[0]
}

func self(vals []interface{}) interface{} {
	if len(vals) == 0 {
		return nil
	}
	if len(vals) > 1 {
		//log.Println("found > 1 value in self()", vals)
		return _strings(vals, false)
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
	"self":   Reducer(self),
	"concat": Reducer(concat),
	"count":  Reducer(count),
	"delete": Reducer(delete),
	"mean":   Reducer(mean),
	"sum":    Reducer(sum),
	"max":    Reducer(max),
	"min":    Reducer(min),
	"uniq":   Reducer(uniq),
	"first":  Reducer(first),
	"flag":   Reducer(vflag),
	"div2":   Reducer(div2),
	"DP2":    Reducer(dp2),
}
