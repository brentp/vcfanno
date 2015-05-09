package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/brentp/irelate"
	"github.com/brentp/vcfgo"
	"github.com/robertkrimen/otto"
)

type Source struct {
	File   string
	Op     string
	Name   string
	Column int
	Field  string
	Index  int
	IsJs   bool
}

func (s *Source) IsNumber() bool {
	return s.Op == "mean" || s.Op == "max" || s.Op == "min" || s.Op == "count" || s.Op == "median"
}

type Annotator struct {
	vm      *otto.Otto
	Sources []Source
}

func (a *Annotator) JsOp(v *vcfgo.Variant, js string, vals []interface{}) interface{} {
	a.vm.Set("chrom", v.Chrom())
	a.vm.Set("start", v.Start())
	a.vm.Set("end", v.End())
	a.vm.Set("vals", vals)
	value, err := a.vm.Run(js[3:])
	if err != nil {
		return fmt.Sprintf("js-error: %s", err)
	}
	val, err := value.ToString()
	if err != nil {
		log.Println("js-error:", err)
		val = fmt.Sprintf("error:%s", err)
	}
	return val
}

/*
func NewAnnotator(annos []anno, js string) *Annotator {
	// TODO: figure out how to get annos from toml into here. flattend out to 1 op per source.
	sources := make([]Source, 0)
	a := Annotator{
		vm:      otto.New(),
		Sources: sources,
	}
	return &a
}
*/

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
		if !overlap(v, other) {
			continue
		}
		if o, ok := other.(*irelate.Variant); ok {
			if strict && !v.Is(o.Variant) {
				continue
			}

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
		} else if bam, ok := other.(*irelate.Bam); ok {
			if bam.MapQ() < 1 || bam.Flags&(0x4) != 0 {
				continue
			}
			coll = append(coll, 1)
		} else {
			panic(fmt.Sprintf("not supported for: %v", other))
		}
	}
	return coll
}

// Annotate a relatable with the Sources in an Annotator.
func (a *Annotator) Annotate(r irelate.Relatable, strict bool) {

	parted := a.partition(r)
	var b *irelate.Interval
	var v *irelate.Variant
	var isBed, isVariant bool
	if v, isVariant = r.(*irelate.Variant); !isVariant {
		if b, isBed = r.(*irelate.Interval); !isBed {
			panic("can only annotate Bed or VCF at this time")
		}
		// create a Variant, annotate it, then pull out the info and put back
		// into bed
		m := make(vcfgo.InfoMap)
		m["__order"] = []string{}
		m["SVLEN"] = int(b.End()-b.Start()) - 1
		var rels []irelate.Relatable
		v = irelate.NewVariant(&vcfgo.Variant{Chromosome: b.Chrom(), Pos: uint64(b.Start() + 1),
			Ref: "A", Alt: []string{"<DEL>"}, Info: m}, 0, rels)
	}

	for _, src := range a.Sources {
		related := parted[src.Index]
		vals := collect(v, related, &src, strict)
		if src.IsJs {
			v.Info.Add(src.Name, a.JsOp(v.Variant, src.Op, vals))
		} else {
			v.Info.Add(src.Name, Reducers[src.Op](vals))
		}
	}
	if isBed {
		delete(v.Info, "SVLEN")
		b.Fields = append(b.Fields, v.Info.String())
	}
}
