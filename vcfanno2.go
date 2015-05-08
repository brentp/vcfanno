package main

import (
	"strconv"

	"github.com/brentp/irelate"
	"github.com/robertkrimen/otto"
)

type Source struct {
	File   string
	Op     string
	Name   string
	Column int
	Field  string
	Index  int
}

func (s *Source) IsNumber() bool {
	return s.Op == "mean" || s.Op == "max" || s.Op == "min" || s.Op == "count" || s.Op == "median"
}

type Annotator struct {
	vm      *otto.Otto
	Sources []Source
}

func NewAnnotator(annos []anno, js string) *Annotator {
	// TODO: figure out how to get annos from toml into here. flattend out to 1 op per source.
	sources := make([]Source, 0)
	a := Annotator{
		vm:      otto.New(),
		Sources: sources,
	}
	return &a
}

func (a *Annotator) partition(r irelate.Relatable) [][]irelate.Relatable {
	parted := make([][]irelate.Relatable, 0)
	for _, o := range r.Related() {
		s := int(o.Source()) - 1
		for len(parted) < s {
			parted = append(parted, make([]irelate.Relatable, 0))
		}
		parted[s] = append(parted[s], o)
	}
	return parted
}

func collect(v *irelate.Variant, rels []irelate.Relatable, src *Source, strict bool) []interface{} {
	coll := make([]interface{}, 0)
	for _, other := range v.Related() {
		if o, ok := other.(*irelate.Variant); ok {

			val := o.Info[src.Field]
			if arr, ok := val.([]interface{}); ok {
				coll = append(coll, arr...)
			} else if val == nil {
				continue
			} else {
				coll = append(coll, val)
			}
		} else if o, ok := other.(*irelate.Interval); ok {
			val := o.Fields[src.Column-1]
			if src.IsNumber() {
				v, e := strconv.ParseFloat(val, 32)
				if e != nil {
					panic(e)
				}
				coll = append(coll, v)
			} else {
				coll = append(coll, val)
			}
		} // TODO: bam
	}
	return coll
}

// Annotate a relatable with the Sources in an Annotator.
func (a *Annotator) Annotate(r irelate.Relatable, strict bool) {

	parted := a.partition(r)
	v := r.(*irelate.Variant)

	for _, src := range a.Sources {
		related := parted[src.Index]
		vals := collect(v, related, &src, strict)
		v.Info.Add(src.Name, Reducers[src.Op](vals))
	}
}
