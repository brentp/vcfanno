package api

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/biogo/hts/sam"
	"github.com/brentp/irelate"
	"github.com/brentp/vcfgo"
	"github.com/robertkrimen/otto"
)

const LEFT = "left_"
const RIGHT = "right_"
const BOTH = "both_"
const INTERVAL = ""

// Source holds the information for a single annotation to be added to a query.
// Many sources can come from the same file, but each must have their own Source.
type Source struct {
	File string
	Op   string
	Name string
	// column number in bed file or ...
	Column int
	// info name in VCF. (can also be ID).
	Field string
	// 0-based index of the file order this source is from.
	Index int
	Js    *otto.Script
}

// IsNumber indicates that we expect the Source to return a number given the op
func (s *Source) IsNumber() bool {
	return s.Op == "mean" || s.Op == "max" || s.Op == "min" || s.Op == "count" || s.Op == "median"
}

// Annotator holds the information to annotate a file.
type Annotator struct {
	vm      *otto.Otto
	Sources []*Source
	Strict  bool // require a variant to have same ref and share at least 1 alt
	Ends    bool // annotate the ends of the variant in addition to the interval itself.
	Less    func(a, b irelate.Relatable) bool
}

// JsOp uses Otto to run a javascript snippet on a list of values and return a single value.
// It makes the chrom, start, end, and values available to the js interpreter.
func (a *Annotator) JsOp(v *vcfgo.Variant, js *otto.Script, vals []interface{}) interface{} {
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

// NewAnnotator returns an Annotator with the sources, seeded with some javascript.
// If ends is true, it will annotate the 1 base ends of the interval as well as the
// interval itself. If strict is true, when overlapping variants, they must share
// the ref allele and at least 1 alt allele.
func NewAnnotator(sources []*Source, js string, ends bool, strict bool, natsort bool) *Annotator {
	for _, s := range sources {
		if e := checkSource(s); e != nil {
			log.Fatal(e)
		}
	}
	var less func(a, b irelate.Relatable) bool
	if natsort {
		less = irelate.NaturalLessPrefix
	} else {
		less = irelate.LessPrefix
	}
	a := Annotator{
		vm:      otto.New(),
		Sources: sources,
		Strict:  strict,
		Ends:    ends,
		Less:    less,
	}
	if js != "" {
		_, err := a.vm.Run(js)
		if err != nil {
			log.Fatalf("error parsing customjs:%s", err)
		}
	}
	for _, src := range a.Sources {
		if strings.HasPrefix(src.Op, "js:") {
			var err error
			src.Js, err = a.vm.Compile(src.Op, src.Op[3:])
			if err != nil {
				log.Fatalf("error parsing op: %s for file %s", src.Op, src.File)
			}
		}
	}
	return &a
}

func checkSource(s *Source) error {
	if s.Name == "" {
		return fmt.Errorf("no name specified for %v", s)
	}
	return nil
}

// partition separates the relateds for a relatable so it reduces running over the data multiple times for each file.
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

// collect applies the reduction (op) specified in src on the rels.
func collect(v *irelate.Variant, rels []irelate.Relatable, src *Source, strict bool) []interface{} {
	coll := make([]interface{}, 0)
	var val interface{}
	for _, other := range rels {
		// need this check for the ends stuff.
		if int(other.Source())-1 != src.Index {
			log.Fatalf("got source %d with related %d", src.Index, other.Source())
		}
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
				var err error
				val, err = o.Info.Get(src.Field)
				if err != nil {
					log.Println(err)
				}
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
			if bam.MapQ() < 1 || (bam.Flags&sam.Unmapped != 0) {
				continue
			}
			if src.Field == "" {
				coll = append(coll, 1)
			} else {
				switch src.Field {
				case "mapq":
					coll = append(coll, bam.MapQ())
				case "seq":
					coll = append(coll, string(bam.Seq.Expand()))
				default:
					if src.Op != "count" {
						log.Fatalf("unknown field %s specifed for bam: %s\n", src.Field, src.File)
					}
					coll = append(coll, 1)
				}
			}
		} else {
			panic(fmt.Sprintf("not supported for: %v", other))
		}
	}
	return coll
}

// vFromB makes a variant from an interval. this helps avoid code duplication.
func vFromB(b *irelate.Interval) *irelate.Variant {
	h := vcfgo.NewHeader()
	h.Infos["SVLEN"] = &vcfgo.Info{Id: "SVLEN", Type: "Integer", Description: "", Number: "1"}
	m := vcfgo.NewInfoByte(fmt.Sprintf("SVLEN=%d", int(b.End()-b.Start())-1), h)
	v := irelate.NewVariant(&vcfgo.Variant{Chromosome: b.Chrom(), Pos: uint64(b.Start() + 1),
		Ref: "A", Alt: []string{"<DEL>"}, Info: m}, 0, b.Related())
	return v
}

// AnnotatedEnds makes a new 1-base interval for the left and one for the right end
// so that it can use the same machinery to annotate the ends and the entire interval.
// Output into the info field is prefixed with "left_" or "right_".
func (a *Annotator) AnnotateEnds(r irelate.Relatable, ends string) error {
	var v *irelate.Variant
	var ok bool
	if v, ok = r.(*irelate.Variant); !ok {
		v = vFromB(r.(*irelate.Interval))
	}
	// if Both, call the interval, left, and right version to annotate.
	if ends == BOTH {
		if e := a.AnnotateOne(v, false); e != nil {
			return e
		}
		if e := a.AnnotateEnds(v, LEFT); e != nil {
			return e
		}
		if e := a.AnnotateEnds(v, RIGHT); e != nil {
			return e
		}
		// it was a Bed, we add the info to its fields
		if !ok {
			b := r.(*irelate.Interval)
			v.Info.Delete("SVLEN")
			b.Fields = append(b.Fields, v.Info.String())
		}
		return nil
	}
	if ends == INTERVAL {
		return a.AnnotateOne(r, a.Strict)
	}
	// hack:
	// modify the variant in-place to create a 1-base variant at the end of
	// the interval. annotate that end and then change the position back to what it was.
	if ends == LEFT || ends == RIGHT {
		// save end here to get the right end.
		pos, ref, alt, end := v.Pos, v.Ref, v.Alt, v.End()
		svlen, err := v.Info.Get("SVLEN")
		v.Info.Set("SVLEN", 1)
		// the end is determined by the alt, so we have to make sure it has length 1.
		v.Ref, v.Alt = "A", []string{"T"}
		if ends == RIGHT {
			v.Pos = uint64(end)
		}

		a.AnnotateOne(v, false, ends)
		v.Pos, v.Ref, v.Alt = pos, ref, alt
		if err == nil && ok {
			v.Info.Set("SVLEN", svlen)
		} else {
			v.Info.Delete("SVLEN")
		}
	}
	return nil
}

// AnnotateOne annotates a relatable with the Sources in an Annotator.
// In most cases, no need to specify end (it should always be a single
// arugment indicting LEFT, RIGHT, or INTERVAL, used from AnnotateEnds
func (a *Annotator) AnnotateOne(r irelate.Relatable, strict bool, end ...string) error {
	if len(r.Related()) == 0 {
		return nil
	}
	prefix := ""
	if len(end) > 0 {
		prefix = end[0]
		if len(end) > 1 {
			log.Fatalf("too many ends in AnnotateOne")
		}
	}

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
		if len(parted) <= src.Index {
			continue
		}

		related := parted[src.Index]
		if len(related) == 0 {
			continue
		}
		vals := collect(v, related, src, strict)
		AnnotateOne(v, src, vals, prefix, a)
	}
	if isBed {
		v.Info.Delete("SVLEN")
		b.Fields = append(b.Fields, v.Info.String())
	}
	return nil
}

func AnnotateOne(v *irelate.Variant, src *Source, vals []interface{}, prefix string, a *Annotator) {
	if len(vals) == 0 {
		return
	}
	if src.Js != nil {
		v.Info.Add(prefix+src.Name, a.JsOp(v.Variant, src.Js, vals))
	} else {
		v.Info.Add(prefix+src.Name, Reducers[src.Op](vals))
	}
}

// UpdateHeader adds to the Infos in the vcf Header so that the annotations will be reported in the header.
func (a *Annotator) UpdateHeader(h *vcfgo.Header) {
	for _, src := range a.Sources {
		UpdateHeader(h, src, a.Ends)
	}
}

func UpdateHeader(h *vcfgo.Header, src *Source, ends bool) {
	ntype := "Character"
	var desc string
	if (strings.HasSuffix(src.File, ".bam") && src.Field == "") || src.IsNumber() {
		ntype = "Float"
	} else if src.Js != nil {
		ntype = "."
	}

	if strings.HasSuffix(src.File, ".bam") && src.Field == "" {
		desc = fmt.Sprintf("calculated by coverage from %s", src.File)
	} else if src.Field != "" {
		desc = fmt.Sprintf("calculated by %s of overlapping values in field %s from %s", src.Op, src.Field, src.File)
	} else {
		desc = fmt.Sprintf("calculated by %s of overlapping values in column %d from %s", src.Op, src.Column, src.File)
	}
	h.Infos[src.Name] = &vcfgo.Info{Id: src.Name, Number: "1", Type: ntype, Description: desc}
	if ends {
		for _, end := range []string{LEFT, RIGHT} {
			h.Infos[end+src.Name] = &vcfgo.Info{Id: end + src.Name, Number: "1", Type: ntype,
				Description: fmt.Sprintf("%s at end %s", desc, strings.TrimSuffix(end, "_"))}
		}
	}
}

// SetupStreams takes the query file and sets everything up for annotation. If the input was
// a vcf, it returns the header with the new annotation fields added.
func (a *Annotator) SetupStreams(queryFile string) ([]irelate.RelatableChannel, *vcfgo.Reader) {

	streams := make([]irelate.RelatableChannel, 1)

	var query *vcfgo.Reader // need this to print header

	if strings.HasSuffix(queryFile, ".bed") || strings.HasSuffix(queryFile, ".bed.gz") {
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
	if nil != query {
		a.UpdateHeader(query.Header)
		return streams, query
	}
	return streams, nil
}

// Annotate annotates a file with the sources in the Annotator.
// It accepts RelatableChannels, and returns a RelatableChannel on which it will send
// annotated variants.
func (a *Annotator) Annotate(streams ...irelate.RelatableChannel) irelate.RelatableChannel {
	ch := make(irelate.RelatableChannel, 48)
	ends := INTERVAL
	if a.Ends {
		ends = BOTH
	}

	go func(irelate.RelatableChannel, *Annotator, string) {
		for interval := range irelate.IRelate(irelate.CheckOverlapPrefix, 0, a.Less, streams...) {
			a.AnnotateEnds(interval, ends)
			ch <- interval
		}
		close(ch)
	}(ch, a, ends)
	return ch
}
