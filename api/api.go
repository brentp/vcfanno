package api

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/biogo/hts/sam"
	"github.com/brentp/bix"
	"github.com/brentp/irelate"
	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/irelate/parsers"
	"github.com/brentp/vcfgo"
	"github.com/robertkrimen/otto"
)

const LEFT = "left_"
const RIGHT = "right_"
const BOTH = "both_"
const INTERVAL = ""

type CIFace interface {
	CIPos() (uint32, uint32, bool)
	CIEnd() (uint32, uint32, bool)
}

type HeaderUpdater interface {
	Update(id string, itype string, number string, description string)
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
	Js    *otto.Script
	Vm    *otto.Otto

	Sweep bool
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
	Less    func(a, b interfaces.Relatable) bool
	Region  string // restrict annotation to this (chrom:start-end) region.
	// if sweep is true, use chrom sweep, otherwise, use tabix
	Sweep bool
}

// JsOp uses Otto to run a javascript snippet on a list of values and return a single value.
// It makes the chrom, start, end, and values available to the js interpreter.
func (s *Source) JsOp(v *parsers.Variant, js *otto.Script, vals []interface{}) string {
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
func NewAnnotator(sources []*Source, js string, ends bool, strict bool, natsort bool, region string) *Annotator {
	for _, s := range sources {
		if e := checkSource(s); e != nil {
			log.Fatal(e)
		}
	}
	var less func(a, b interfaces.Relatable) bool
	if natsort {
		less = irelate.NaturalLessPrefix
	} else {
		less = irelate.LessPrefix
	}
	a := Annotator{
		Sources: sources,
		Strict:  strict,
		Ends:    ends,
		Less:    less,
		Region:  region,
	}
	vm := otto.New()
	for _, s := range a.Sources {
		s.Vm = vm
	}
	if js != "" {
		_, err := vm.Run(js)
		if err != nil {
			log.Fatalf("error parsing customjs:%s", err)
		}
	}
	for _, src := range a.Sources {
		if strings.HasPrefix(src.Op, "js:") {
			var err error
			src.Js, err = vm.Compile(src.Op, src.Op[3:])
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
	coll := make([]interface{}, 0)
	var val interface{}
	for _, other := range rels {
		if int(other.Source())-1 != src.Index {
			log.Fatalf("got source %d with related %d", src.Index, other.Source())
		}
		// need this check for the ends stuff.
		if !overlap(v.(interfaces.Relatable), other) {
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
			} else if strings.ContainsRune(src.Field, '/') {
				fields := strings.Split(src.Field, "/")
				vals := make([]interface{}, len(fields))
				var err error
				for i, f := range fields {
					vals[i], err = o.Info().Get(f)
					if err != nil {
						log.Println(err)
					}
				}
				val = float64(vals[0].(int)) / float64(vals[1].(int))
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
				v, e := strconv.ParseFloat(sval, 32)
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
	m := vcfgo.NewInfoByte(fmt.Sprintf("SVLEN=%d", int(b.End()-b.Start())-1), h)
	v := parsers.NewVariant(&vcfgo.Variant{Chromosome: b.Chrom(), Pos: uint64(b.Start() + 1),
		Reference: "A", Alternate: []string{"<DEL>"}, Info_: m}, 0, b.Related())
	return v
}

// AnnotatedEnds makes a new 1-base interval for the left and one for the right end
// so that it can use the same machinery to annotate the ends and the entire interval.
// Output into the info field is prefixed with "left_" or "right_".
func (a *Annotator) AnnotateEnds(r interfaces.Relatable, ends string) error {
	var v *parsers.Variant
	var ok bool
	var err error
	if v, ok = r.(*parsers.Variant); !ok {
		v = vFromB(r.(*parsers.Interval))
	}
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
			b.Fields = append(b.Fields, v.Info().String())
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
			l, r, ok = v.IVariant.(CIFace).CIPos()
		} else {
			l, r, ok = v.IVariant.(CIFace).CIEnd()
		}
		// dont reannotate same interval
		if !ok && (l == v.Start() && r == v.End()) {
			return nil
		}

		m := vcfgo.NewInfoByte(fmt.Sprintf("SVLEN=%d", r-l-1), v.IVariant.(*vcfgo.Variant).Header)
		v2 := parsers.NewVariant(&vcfgo.Variant{Chromosome: v.Chrom(), Pos: uint64(l + 1),
			Reference: "A", Alternate: []string{"<DEL>"}, Info_: m}, v.Source(), v.Related())

		err = a.AnnotateOne(v2, false, ends)
		if err != nil {
			log.Println(err)
		}
		var val interface{}
		for _, key := range v2.Info().Keys() {
			if key == "SVLEN" {
				continue
			}
			val, err = v2.Info().Get(key)
			v.Info().Set(key, val)
		}
	}
	return err
}

// TODO: cleanup how ends are handled. Thats the most error-prone part fo the code.
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
	var b *parsers.Interval
	var v *parsers.Variant
	var isBed, isVariant bool
	if v, isVariant = r.(*parsers.Variant); !isVariant {
		if b, isBed = r.(*parsers.Interval); !isBed {
			panic("can only annotate Bed or VCF at this time")
		}
		// make a Variant, annotate it, pull out the info, put back in bed
		v = vFromB(b)
		strict = false // can't be strict with bed query.
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
		src.AnnotateOne(v, vals, prefix)
	}
	if isBed {
		v.Info().Delete("SVLEN")
		b.Fields = append(b.Fields, v.Info().String())
	}
	return nil
}

func (src *Source) AnnotateOne(v *parsers.Variant, vals []interface{}, prefix string) {
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
func (a *Annotator) UpdateHeader(h *vcfgo.Header) {
	for _, src := range a.Sources {
		src.UpdateHeader(h, a.Ends)
	}
}

// TODO make an interface for Header so we can do this for cgtabix as well.
//func (src *Source) UpdateHeader(h HeaderUpdater, ends bool) {
func (src *Source) UpdateHeader(h *vcfgo.Header, ends bool) {
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
	h.Infos[src.Name] = &vcfgo.Info{Id: src.Name, Number: number, Type: ntype, Description: desc}
	if ends {
		for _, end := range []string{LEFT, RIGHT} {
			h.Infos[end+src.Name] = &vcfgo.Info{Id: end + src.Name, Number: "1", Type: ntype,
				Description: fmt.Sprintf("%s at end %s", desc, strings.TrimSuffix(end, "_"))}
		}
	}
}

// SetupStreams takes the query stream and sets everything up for annotation.
func (a *Annotator) SetupStreams(qStream interfaces.RelatableChannel) ([]interfaces.RelatableChannel, []interfaces.RandomGetter, error) {

	streams := make([]interfaces.RelatableChannel, 1)
	streams[0] = qStream
	getters := make([]interfaces.RandomGetter, 0)

	seen := make(map[int]bool)
	for _, src := range a.Sources {
		// have expanded so there are many sources per file.
		// use seen to just grab the file the first time it is seen and start a stream
		if _, ok := seen[src.Index]; ok {
			continue
		}
		if a.Sweep {
			seen[src.Index] = true
			s, err := irelate.Streamer(src.File, a.Region)
			if err != nil {
				if a.Region != "" && strings.HasSuffix(src.File, ".bam") {
					log.Println("ERROR: can't do regional queries on bam files")
				}
				return streams[:0], getters[:0], err
			}
			streams = append(streams, s)
		} else {
			tbx, err := bix.New(src.File)
			if err != nil {
				return streams[:0], getters[:0], err
			}
			getters = append(getters, tbx)
		}
	}
	return streams, getters, nil
}

// Annotate annotates a file with the sources in the Annotator.
// It accepts RelatableChannels, and returns a RelatableChannel on which it will send
// annotated variants.
func (a *Annotator) Annotate(streams []interfaces.RelatableChannel, getters []interfaces.RandomGetter) interfaces.RelatableChannel {
	ch := make(interfaces.RelatableChannel, 48)
	ends := INTERVAL
	if a.Ends {
		ends = BOTH
	}

	n := len(streams)
	go func(ch interfaces.RelatableChannel, a *Annotator, ends string) {
		for interval := range irelate.IRelate(irelate.CheckOverlapPrefix, 0, a.Less, streams...) {
			for i, getter := range getters {
				for _, rel := range getter.Get(interval) {
					// TODO: just check A.Ends and expand interval as needed.
					orel := rel.(interfaces.Relatable)
					orel.SetSource(uint32(n + i))
					interval.AddRelated(orel)
				}
			}
			a.AnnotateEnds(interval, ends)
			ch <- interval
		}
		close(ch)
	}(ch, a, ends)
	return ch
}
