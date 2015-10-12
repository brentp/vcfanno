package api

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/biogo/hts/sam"
	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/irelate/parsers"
	"github.com/brentp/vcfgo"
	"github.com/robertkrimen/otto"
)

const LEFT = "left_"
const RIGHT = "right_"
const BOTH = "both_"
const INTERVAL = ""

type HeaderUpdater interface {
	AddInfoToHeader(id string, itype string, number string, description string)
}

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
	mu    sync.RWMutex
	Js    *otto.Script
	Vm    *otto.Otto
}

// IsNumber indicates that we expect the Source to return a number given the op
func (s *Source) IsNumber() bool {
	return s.Op == "mean" || s.Op == "max" || s.Op == "min" || s.Op == "count" || s.Op == "median"
}

// Annotator holds the information to annotate a file.
type Annotator struct {
	Sources []*Source
	Strict  bool // require a variant to have same ref and share at least 1 alt
	Ends    bool // annotate the ends of the variant in addition to the interval itself.
}

// JsOp uses Otto to run a javascript snippet on a list of values and return a single value.
// It makes the chrom, start, end, and values available to the js interpreter.
func (s *Source) JsOp(v interfaces.IVariant, js *otto.Script, vals []interface{}) string {
	s.mu.Lock()
	s.Vm.Set("chrom", v.Chrom())
	s.Vm.Set("start", v.Start())
	s.Vm.Set("end", v.End())
	s.Vm.Set("vals", vals)
	//s.Vm.Set("info", v.Info.String())
	value, err := s.Vm.Run(js)
	if err != nil {
		return fmt.Sprintf("js-error: %s", err)
	}
	val, err := value.ToString()
	s.mu.Unlock()
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
func NewAnnotator(sources []*Source, js string, ends bool, strict bool) *Annotator {
	for _, s := range sources {
		if e := checkSource(s); e != nil {
			log.Fatal(e)
		}
	}
	a := Annotator{
		Sources: sources,
		Strict:  strict,
		Ends:    ends,
	}
	for _, src := range a.Sources {
		if strings.HasPrefix(src.Op, "js:") {
			src.Vm = otto.New() // create a new vm for each source and lock in the source
			var err error
			src.Js, err = src.Vm.Compile(src.Op, src.Op[3:])
			if err != nil {
				log.Fatalf("error parsing op: %s for file %s", src.Op, src.File)
			}
			if js != "" {
				_, err := src.Vm.Run(js)
				if err != nil {
					log.Fatalf("error parsing customjs:%s", err)
				}
			}
		}
	}
	return &a
}

func unsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func checkSource(s *Source) error {
	if s.Name == "" {
		return fmt.Errorf("no name specified for %v", s)
	}
	return nil
}

// partition separates the relateds for a relatable so it reduces running over the data multiple times for each file.
func (a *Annotator) partition(r interfaces.Relatable) [][]interfaces.Relatable {
	parted := make([][]interfaces.Relatable, 0)
	for _, o := range r.Related() {
		s := int(o.Source()) - 1
		for len(parted) <= s {
			parted = append(parted, make([]interfaces.Relatable, 0))
		}
		parted[s] = append(parted[s], o)
	}
	return parted
}

// collect applies the reduction (op) specified in src on the rels.
func collect(v interfaces.IVariant, rels []interfaces.Relatable, src *Source, strict bool) []interface{} {
	coll := make([]interface{}, 0, len(rels))
	var val interface{}
	for _, other := range rels {
		if int(other.Source())-1 != src.Index {
			log.Fatalf("got source %d with related %d", src.Index, other.Source())
		}
		// need this check for the ends stuff.
		if !overlap(v.(interfaces.IPosition), other) {
			continue
		}
		if o, ok := other.(interfaces.IVariant); ok {
			if strict && !interfaces.Same(v, o, strict) {
				continue
			}
			// special case pulling the rsid
			if src.Field == "ID" {
				val = o.Id()
				if val == "." || val == "" {
					continue
				}
			} else {
				var err error
				val, err = o.Info().Get(src.Field)
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
		} else if o, ok := other.(*parsers.Interval); ok {
			sval := o.Fields[src.Column-1]
			if src.IsNumber() {

				v, e := strconv.ParseFloat(unsafeString(sval), 32)
				if e != nil {
					log.Println(e)
				}
				coll = append(coll, v)
			} else {
				coll = append(coll, sval)
			}
		} else if bam, ok := other.(*parsers.Bam); ok {
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
			msg := fmt.Sprintf("not supported for: %T", other)
			log.Println(msg)
			coll = []interface{}{msg}
		}
	}
	return coll
}

// vFromB makes a variant from an interval. this helps avoid code duplication.
func vFromB(b *parsers.Interval) *parsers.Variant {
	h := vcfgo.NewHeader()
	h.Infos["SVLEN"] = &vcfgo.Info{Id: "SVLEN", Type: "Integer", Description: "", Number: "1"}
	m := vcfgo.NewInfoByte([]byte(fmt.Sprintf("SVLEN=%d", int(b.End()-b.Start())-1)), h)
	v := parsers.NewVariant(&vcfgo.Variant{Chromosome: b.Chrom(), Pos: uint64(b.Start() + 1),
		Reference: "A", Alternate: []string{"<DEL>"}, Info_: m}, 0, b.Related())
	return v
}

// AnnotateOne annotates a relatable with the Sources in an Annotator.
// In most cases, no need to specify end (it should always be a single
// arugment indicting LEFT, RIGHT, or INTERVAL, used from AnnotateEnds
func (a *Annotator) AnnotateOne(r interfaces.Relatable, strict bool, end ...string) error {
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
	var v interfaces.IVariant
	v, ok := r.(interfaces.IVariant)
	if !ok {
		v = r.(interfaces.IVariant)
	}

	var src *Source
	for i := range a.Sources {
		src = a.Sources[i]
		if len(parted) <= src.Index {
			continue
		}

		related := parted[src.Index]
		if len(related) == 0 {
			continue
		}
		vals := collect(v, related, src, strict)
		src.AnnotateOne(v, vals, prefix)
	}
	return nil
}

func (src *Source) AnnotateOne(v interfaces.IVariant, vals []interface{}, prefix string) {
	if len(vals) == 0 {
		return
	}
	if src.Js != nil {
		jsval := src.JsOp(v, src.Js, vals)
		if jsval == "true" || jsval == "false" && strings.Contains(src.Op, "_flag(") {
			if jsval == "true" {
				v.Info().Set(prefix+src.Name, true)
			}
		} else {
			v.Info().Set(prefix+src.Name, jsval)
		}
	} else {
		val := Reducers[src.Op](vals)
		v.Info().Set(prefix+src.Name, val)
	}
}

// UpdateHeader adds to the Infos in the vcf Header so that the annotations will be reported in the header.
//func (a *Annotator) UpdateHeader(h HeaderUpdater) {
func (a *Annotator) UpdateHeader(r HeaderUpdater) {
	for _, src := range a.Sources {
		src.UpdateHeader(r, a.Ends)
	}
}

//func (src *Source) UpdateHeader(h HeaderUpdater, ends bool) {
func (src *Source) UpdateHeader(r HeaderUpdater, ends bool) {
	ntype, number := "Character", "1"
	var desc string

	if strings.HasSuffix(src.Field, "_float") {
		ntype, number = "Float", "1"
	} else if strings.HasSuffix(src.Field, "_int") {
		ntype, number = "Integer", "1"
	} else if strings.HasSuffix(src.Field, "_flag") || strings.Contains(src.Field, "flag(") {
		ntype, number = "Integer", "1"

	} else {
		if src.Op == "flag" {
			ntype, number = "Flag", "0"
		}
		if (strings.HasSuffix(src.File, ".bam") && src.Field == "") || src.IsNumber() {
			ntype = "Float"
		} else if src.Js != nil {
			if strings.Contains(src.Op, "_flag(") {
				ntype, number = "Flag", "0"
			} else {
				ntype = "Character"
			}
		}
	}

	if strings.HasSuffix(src.File, ".bam") && src.Field == "" {
		desc = fmt.Sprintf("calculated by coverage from %s", src.File)
	} else if src.Field != "" {
		desc = fmt.Sprintf("calculated by %s of overlapping values in field %s from %s", src.Op, src.Field, src.File)
	} else {
		desc = fmt.Sprintf("calculated by %s of overlapping values in column %d from %s", src.Op, src.Column, src.File)
	}
	r.AddInfoToHeader(src.Name, number, ntype, desc)
	if ends {
		for _, end := range []string{LEFT, RIGHT} {
			d := fmt.Sprintf("%s at end %s", desc, strings.TrimSuffix(end, "_"))
			r.AddInfoToHeader(end+src.Name, number, ntype, d)
		}
	}
}

// SetupStreams takes the query stream and sets everything up for annotation.
func (a *Annotator) SetupStreams() ([]string, error) {

	files := make([]string, 0, 4)

	seen := make(map[int]bool)
	for _, src := range a.Sources {
		// have expanded so there are many sources per file.
		// use seen to just grab the file the first time it is seen and start a stream
		if _, ok := seen[src.Index]; ok {
			continue
		}
		seen[src.Index] = true
		files = append(files, src.File)
	}
	return files, nil
}

// AnnotatedEnds makes a new 1-base interval for the left and one for the right end
// so that it can use the same machinery to annotate the ends and the entire interval.
// Output into the info field is prefixed with "left_" or "right_".
func (a *Annotator) AnnotateEnds(r interfaces.Relatable, ends string) error {
	v, ok := r.(interfaces.VarWrap)
	if !ok {
		return fmt.Errorf("unable to convert %v to Variant", r)
	}

	var err error
	// if Both, call the interval, left, and right version to annotate.
	if ends == BOTH {
		// dont want strict for BED.
		if e := a.AnnotateOne(v, a.Strict && ok); e != nil {
			log.Println(e)
			return e
		}
		if e := a.AnnotateEnds(v, LEFT); e != nil {
			log.Println(e)
			return e
		}
		if e := a.AnnotateEnds(v, RIGHT); e != nil {
			log.Println(e)
			return e
		}
		// it was a Bed, we add the info to its fields
		if !ok {
			b := r.(*parsers.Interval)
			v.Info().Delete("SVLEN")
			b.Fields = append(b.Fields, v.Info().Bytes())
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
		// the end is determined by the SVLEN, so we have to make sure it has length 1.
		var l, r uint32
		var ok bool
		if ends == LEFT {
			l, r, ok = v.IVariant.(interfaces.CIFace).CIPos()
		} else {
			l, r, ok = v.IVariant.(interfaces.CIFace).CIEnd()
		}
		// dont reannotate same interval
		if !ok && (l == v.Start() && r == v.End()) {
			return nil
		}

		m := vcfgo.NewInfoByte([]byte(fmt.Sprintf("SVLEN=%d;END=%d", r-l-1, r)), v.IVariant.(*vcfgo.Variant).Header)
		v2 := parsers.NewVariant(&vcfgo.Variant{Chromosome: v.Chrom(), Pos: uint64(l + 1),
			Reference: "A", Alternate: []string{"<DEL>"}, Info_: m}, v.Source(), v.Related())

		err = a.AnnotateOne(v2, false, ends)
		if err != nil {
			log.Println(err)
		}
		var val interface{}
		for _, key := range v2.Info().Keys() {
			if key == "SVLEN" || key == "END" {
				continue
			}
			val, err = v2.Info().Get(key)
			v.Info().Set(key, val)
		}
	}
	return err
}
