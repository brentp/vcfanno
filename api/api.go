package api

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/biogo/hts/sam"
	"github.com/brentp/bix"
	"github.com/brentp/goluaez"
	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/irelate/parsers"
	"github.com/brentp/vcfgo"
)

const LEFT = "left_"
const RIGHT = "right_"
const BOTH = "both_"
const INTERVAL = ""

type HeaderUpdater interface {
	AddInfoToHeader(id string, itype string, number string, description string)
}

type HeaderTyped interface {
	GetHeaderType(field string) string
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
	mu    sync.Mutex
	code  string
	Vm    *goluaez.State
}

// IsNumber indicates that we expect the Source to return a number given the op
func (s *Source) IsNumber() bool {
	return s.Op == "mean" || s.Op == "max" || s.Op == "min" || s.Op == "count" || s.Op == "median"
}

// Annotator holds the information to annotate a file.
type Annotator struct {
	Sources   []*Source
	Strict    bool // require a variant to have same ref and share at least 1 alt
	Ends      bool // annotate the ends of the variant in addition to the interval itself.
	PostAnnos []*PostAnnotation
}

// LuaOp uses go-lua to run a lua snippet on a list of values and return a single value.
// It makes the chrom, start, end, and values available to the lua interpreter.
func (s *Source) LuaOp(v interfaces.IVariant, code string, vals []interface{}) string {
	value, err := s.Vm.Run(code, map[string]interface{}{
		"chrom": v.Chrom(),
		"start": v.Start(),
		"stop":  v.End(),
		"vals":  vals})
	if err != nil {
		log.Printf("lua-error @ %s:%d: %s\n", v.Chrom(), v.Start()+1, err)
		return fmt.Sprintf("err:%v", value)
	}
	return fmt.Sprintf("%v", value)
}

type PostAnnotation struct {
	Fields []string
	Op     string
	Name   string
	Type   string

	code string

	// use 8 of these to avoid contention in parallel contexts.
	mus [8]chan int
	Vms [8]*goluaez.State
}

// NewAnnotator returns an Annotator with the sources, seeded with some javascript.
// If ends is true, it will annotate the 1 base ends of the interval as well as the
// interval itself. If strict is true, when overlapping variants, they must share
// the ref allele and at least 1 alt allele.
func NewAnnotator(sources []*Source, lua string, ends bool, strict bool, postannos []PostAnnotation) *Annotator {
	for _, s := range sources {
		if e := checkSource(s); e != nil {
			log.Fatal(e)
		}
	}

	a := Annotator{
		Sources:   sources,
		Strict:    strict,
		Ends:      ends,
		PostAnnos: make([]*PostAnnotation, len(postannos)),
	}
	var err error
	for i := range postannos {
		for k := 0; k < len(postannos[i].Vms); k++ {
			postannos[i].Vms[k], err = goluaez.NewState(lua)
			postannos[i].mus[k] = make(chan int, 1)
			postannos[i].mus[k] <- k
		}
		if err != nil {
			log.Fatalf("error parsing custom lua:%s", err)
		}
		if strings.HasPrefix(postannos[i].Op, "lua:") {
			postannos[i].code = postannos[i].Op[4:]
		} else if _, ok := Reducers[postannos[i].Op]; !ok {
			log.Fatalf("unknown op from %s: %s", postannos[i].Name, postannos[i].Op)
		}
		a.PostAnnos[i] = &postannos[i]
	}
	for _, src := range a.Sources {
		src.Vm, err = goluaez.NewState(lua) // create a new vm for each source and lock in the source
		if err != nil {
			log.Fatalf("error parsing custom lua:%s", err)
		}
		if strings.HasPrefix(src.Op, "lua:") {
			var err error
			src.code = src.Op[4:]
			if err != nil {
				log.Fatalf("error parsing op: %s for file %s", src.Op, src.File)
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
				if src.Op == "uniq" || src.Op == "concat" {
					sarr := make([]string, len(arr))
					for i, v := range arr {
						sarr[i] = fmt.Sprintf("%v", v)
					}
					coll = append(coll, strings.Join(sarr, ","))
				} else {
					coll = append(coll, arr...)
				}
			} else if val == nil {
				continue
			} else {
				coll = append(coll, val)
			}
		} else if o, ok := other.(*parsers.Interval); ok {
			sval := string(o.Fields[src.Column-1])
			if src.IsNumber() {

				v, e := strconv.ParseFloat(sval, 32)
				if e != nil {
					log.Println(e)
				}
				coll = append(coll, v)
			} else {
				coll = append(coll, strings.Replace(sval, ";", ",", -1))
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
		log.Fatal("can't annotate non-IVariant", r)
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
	if src.code != "" {
		luaval := src.LuaOp(v, src.code, vals)
		if luaval == "true" || luaval == "false" && strings.Contains(src.Op, "_flag(") {
			if luaval == "true" {
				v.Info().Set(prefix+src.Name, true)
			}
		} else {
			v.Info().Set(prefix+src.Name, luaval)
		}
	} else {
		val := Reducers[src.Op](vals)
		v.Info().Set(prefix+src.Name, val)
	}
}

//func (src *Source) UpdateHeader(h HeaderUpdater, ends bool) {
func (src *Source) UpdateHeader(r HeaderUpdater, ends bool, htype string) {
	ntype, number := "Character", "1"
	var desc string
	// for 'self' and 'first', we can get the type from the header of the annotation file.
	if htype != "" && (src.Op == "self" || src.Op == "first") {
		ntype = htype
	} else {
		if src.Op == "mean" || src.Op == "max" {
			ntype, number = "Float", "1"
		} else if strings.HasSuffix(src.Field, "_float") {
			ntype, number = "Float", "1"
		} else if strings.HasSuffix(src.Field, "_int") {
			ntype, number = "Integer", "1"
		} else if strings.HasSuffix(src.Field, "_flag") || strings.Contains(src.Field, "flag(") {
			ntype, number = "Flag", "0"
		} else {
			if src.Op == "flag" {
				ntype, number = "Flag", "0"
			}
			if (strings.HasSuffix(src.File, ".bam") && src.Field == "") || src.IsNumber() {
				ntype = "Float"
			} else if src.code != "" {
				if strings.Contains(src.Op, "_flag(") {
					ntype, number = "Flag", "0"
				} else {
					ntype = "Character"
				}
			}
		}
	}
	if (src.Op == "first" || src.Op == "self") && htype == ntype {
		desc = fmt.Sprintf("transfered from matched variants in %s", src.File)
	} else if strings.HasSuffix(src.File, ".bam") && src.Field == "" {
		desc = fmt.Sprintf("calculated by coverage from %s", src.File)
	} else if src.Field != "" {
		desc = fmt.Sprintf("calculated by %s of overlapping values in field %s from %s", src.Op, src.Field, src.File)
	} else {
		desc = fmt.Sprintf("calculated by %s of overlapping values in column %d from %s", src.Op, src.Column, src.File)
	}
	r.AddInfoToHeader(src.Name, number, ntype, desc)
	if ends {
		if src.Op == "self" {
			// what to do here?
		}
		for _, end := range []string{LEFT, RIGHT} {
			d := fmt.Sprintf("%s at end %s", desc, strings.TrimSuffix(end, "_"))
			r.AddInfoToHeader(end+src.Name, number, ntype, d)
		}
	}
}

func (a *Annotator) PostAnnotate(info interfaces.Info) error {
	var err error
	vals := make([]interface{}, 0, 2)
	fields := make([]string, 0, 2)
	missing := make([]string, 0, 2)
	for i := range a.PostAnnos {
		post := a.PostAnnos[i]
		vals = vals[:0]
		fields = fields[:0]
		missing = missing[:0]
		// lua code
		if post.code != "" {
			for _, field := range post.Fields {
				val, _ := info.Get(field)
				// ignore the error as it means the field is not present.
				if val != nil {
					vals = append(vals, val)
					fields = append(fields, field)
				} else {
					missing = append(missing, field)
				}
			}
			// we need to try even if it didn't get all values.
			if len(vals) == 0 {
				continue
			}

			k := 0
		out:
			// could also use fanIn where all channels send to a single
			// channel and I pull from that.
			for {
				select {
				case k = <-post.mus[0]:
					break out
				case k = <-post.mus[1]:
					break out
				case k = <-post.mus[2]:
					break out
				case k = <-post.mus[3]:
					break out
				case k = <-post.mus[4]:
					break out
				case k = <-post.mus[5]:
					break out
				case k = <-post.mus[6]:
					break out
				case k = <-post.mus[7]:
					break out
				default:
				}
			}

			for i, val := range vals {
				post.Vms[k].SetGlobal(fields[i], val)
			}
			// need to unset missing values so we don't use those
			// from previous run.
			for _, miss := range missing {
				post.Vms[k].SetGlobal(miss, nil)
			}
			value, e := post.Vms[k].Run(post.code)
			post.mus[k] <- k
			if e != nil {
				err = e
			}
			val := fmt.Sprintf("%v", value)
			if post.Type == "Flag" {
				if !(strings.ToLower(val) == "false" || val == "0" || val == "") {
					e := info.Set(post.Name, true)
					if e != nil {
						err = e
					}
				}

			} else {
				if e := info.Set(post.Name, val); e != nil {
					err = e
				}
			}

		} else {
			// built in function.
			// re-use vals
			for _, field := range post.Fields {
				// ignore error when field isnt found. we expect that to occur a lot.
				val, _ := info.Get(field)
				if val != nil {
					vals = append(vals, val)
				}
			}
			// run this as long as we found any of the values.
			if len(vals) != 0 {
				fn := Reducers[post.Op]
				info.Set(post.Name, fn(vals))
			}
		}

	}
	return err
}

func (a *Annotator) Setup(query HeaderUpdater) ([]interfaces.Queryable, error) {
	queryables := make([]interfaces.Queryable, 0)
	files, fmap, err := a.setupStreams()
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		q, err := bix.New(file, 1)
		if err != nil {
			return nil, err
		}
		queryables = append(queryables, q)
		for _, src := range fmap[file] {
			src.UpdateHeader(query, a.Ends, q.GetHeaderType(src.Field))
		}
	}
	for _, post := range a.PostAnnos {
		query.AddInfoToHeader(post.Name, ".", post.Type, fmt.Sprintf("calculated field: %s", post.Name))
	}
	return queryables, nil
}

// SetupStreams takes the query stream and sets everything up for annotation.
func (a *Annotator) setupStreams() ([]string, map[string][]*Source, error) {

	lookup := make(map[string][]*Source, len(a.Sources))
	files := make([]string, 0)
	for _, src := range a.Sources {
		// have expanded so there are many sources per file.
		// use seen to just grab the file the first time it is seen and start a stream
		if _, ok := lookup[src.File]; !ok {
			lookup[src.File] = make([]*Source, 0)
			files = append(files, src.File)
		}
		lookup[src.File] = append(lookup[src.File], src)
	}
	return files, lookup, nil
}

// AnnotatedEnds makes a new 1-base interval for the left and one for the right end
// so that it can use the same machinery to annotate the ends and the entire interval.
// Output into the info field is prefixed with "left_" or "right_".
func (a *Annotator) AnnotateEnds(v interfaces.Relatable, ends string) error {

	var err error
	// if Both, call the interval, left, and right version to annotate.
	if ends == BOTH {
		if e := a.AnnotateOne(v, a.Strict); e != nil {
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
		return a.PostAnnotate(v.(interfaces.IVariant).Info())
	}
	if ends == INTERVAL {
		err := a.AnnotateOne(v, a.Strict)
		err2 := a.PostAnnotate(v.(interfaces.IVariant).Info())
		if err != nil {
			return err
		}
		return err2
	}
	// hack:
	// modify the variant in-place to create a 1-base variant at the end of
	// the interval. annotate that end and then change the position back to what it was.
	if ends == LEFT || ends == RIGHT {
		// the end is determined by the SVLEN, so we have to make sure it has length 1.
		variant := v.(*parsers.Variant).IVariant
		var l, r uint32
		var ok bool
		if ends == LEFT {
			l, r, ok = variant.(interfaces.CIFace).CIPos()
		} else {
			l, r, ok = variant.(interfaces.CIFace).CIEnd()
		}
		// dont reannotate same interval
		if !ok && (l == v.Start() && r == v.End()) {
			return nil
		}

		m := vcfgo.NewInfoByte([]byte(fmt.Sprintf("SVLEN=%d;END=%d", r-l-1, r)), variant.(*vcfgo.Variant).Header)
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
			variant.Info().Set(key, val)
		}
	}
	//err2 := a.PostAnnotate(v.(interfaces.IVariant).Info())
	return err
}
