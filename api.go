package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

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
	Strict  bool // require a variant to have same ref and share at least 1 alt
	Ends    bool // annotate the ends of the variant in addition to the interval itself.
}

func (a *Annotator) JsOp(v *vcfgo.Variant, js string, vals []interface{}) interface{} {
	a.vm.Set("chrom", v.Chrom())
	a.vm.Set("start", v.Start())
	a.vm.Set("end", v.End())
	a.vm.Set("vals", vals)
	value, err := a.vm.Run(js)
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

func NewAnnotator(sources []Source, js string, ends bool, strict bool) *Annotator {
	a := Annotator{
		vm:      otto.New(),
		Sources: sources,
		Strict:  strict,
		Ends:    ends,
	}
	if js != "" {
		_, err := a.vm.Run(js)
		if err != nil {
			log.Fatalf("error parsing customjs:%s", err)
		}
	}
	return &a
}

func (a *Annotator) partition(r irelate.Relatable) [][]irelate.Relatable {
	parted := make([][]irelate.Relatable, 0)
	for _, o := range r.Related() {
		s := int(o.Source()) - 1
		for len(parted) <= s {
			parted = append(parted, make([]irelate.Relatable, 0))
		}
		parted[s] = append(parted[s], o)
	}
	return parted
}

func collect(v *irelate.Variant, rels []irelate.Relatable, src *Source, strict bool) []interface{} {
	coll := make([]interface{}, 0)
	var val interface{}
	for _, other := range rels {
		// need this check for the ends stuff.
		if !overlap(v, other) {
			continue
		}
		if o, ok := other.(*irelate.Variant); ok {
			if strict && !v.Is(o.Variant) {
				continue
			}
			// special case pulling the rsid
			if src.Field == "ID" {
				if o.Id == "." {
					continue
				}
				val = o.Id
			} else {
				val = o.Info[src.Field]
			}
			if arr, ok := val.([]interface{}); ok {
				coll = append(coll, arr...)
			} else if val == nil {
				continue
			} else {
				coll = append(coll, val)
			}
		} else if o, ok := other.(*irelate.Interval); ok {
			sval := o.Fields[src.Column-1]
			if src.IsNumber() {
				v, e := strconv.ParseFloat(sval, 32)
				if e != nil {
					panic(e)
				}
				coll = append(coll, v)
			} else {
				coll = append(coll, sval)
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

// make a variant from an interval. this helps avoid code duplication.
func vFromB(b *irelate.Interval) *irelate.Variant {
	m := make(vcfgo.InfoMap)
	m["__order"] = []string{}
	m["SVLEN"] = int(b.End()-b.Start()) - 1
	var rels []irelate.Relatable
	v := irelate.NewVariant(&vcfgo.Variant{Chromosome: b.Chrom(), Pos: uint64(b.Start() + 1),
		Ref: "A", Alt: []string{"<DEL>"}, Info: m}, 0, rels)
	return v
}

// Annotate a relatable with the Sources in an Annotator.
func (a *Annotator) AnnotateOne(r irelate.Relatable) {
	// TODO: could pass isBed to here to avoid cast check.

	parted := a.partition(r)
	var b *irelate.Interval
	var v *irelate.Variant
	var isBed, isVariant bool
	if v, isVariant = r.(*irelate.Variant); !isVariant {
		if b, isBed = r.(*irelate.Interval); !isBed {
			panic("can only annotate Bed or VCF at this time")
		}
		// make a Variant, annotate it, pull out the info, put back in bed
		v = vFromB(b)
	}

	for _, src := range a.Sources {
		related := parted[src.Index]
		if len(related) == 0 {
			continue
		}
		vals := collect(v, related, &src, a.Strict)
		if len(vals) == 0 {
			continue
		}
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

// a horrible function to set up everything for starting annotation.
func (a *Annotator) setupStreams(queryFile string) ([]irelate.RelatableChannel, bool, io.Writer) {

	streams := make([]irelate.RelatableChannel, 1)
	var isBed bool

	var query *vcfgo.Reader // need this to print header
	if strings.HasSuffix(queryFile, ".bed") || strings.HasSuffix(queryFile, ".bed.gz") {
		isBed = true
		streams[0] = irelate.Streamer(queryFile)
	} else {
		query = irelate.Vopen(queryFile)
		streams[0] = irelate.StreamVCF(query)
	}

	seen := make(map[int]bool)
	for _, src := range a.Sources {
		// have expanded so there are many sources per file.
		// use seen to just grab the file the first time it is seen and start a stream
		if _, ok := seen[src.Index]; ok {
			continue
		}
		seen[src.Index] = true
		streams = append(streams, irelate.Streamer(src.File))
	}
	var out io.Writer
	var err error
	if !isBed {
		out, err = vcfgo.NewWriter(os.Stdout, query.Header)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		out = bufio.NewWriter(os.Stdout)
	}
	return streams, isBed, out
}

func (a *Annotator) Annotate(queryFile string) {

	streams, isBed, out := a.setupStreams(queryFile)
	_ = isBed
	_ = out

	for interval := range irelate.IRelate(irelate.CheckOverlapPrefix, 0, irelate.LessPrefix, streams...) {
		a.AnnotateOne(interval)
		fmt.Fprintln(out, interval)
	}
}
