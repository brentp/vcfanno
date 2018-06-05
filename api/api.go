package api

import (
	"fmt"
	"log"
	"os"
	"reflect"
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

// LEFT prefix
const LEFT = "left_"

// RIGHT prefix
const RIGHT = "right_"

// BOTH prefix
const BOTH = "both_"

// INTERVAL prefix
const INTERVAL = ""

// HeaderUpdater allows adding an info to a Header
type HeaderUpdater interface {
	AddInfoToHeader(id string, itype string, number string, description string)
}

// HeaderTyped allows getting the type and Number of a (VCF) field
type HeaderTyped interface {
	GetHeaderType(field string) string
	GetHeaderNumber(field string) string
}

// Source holds the information for a single annotation to be added to a query.
// Many sources can come from the same file, but each must have their own Source.
type Source struct {
	File string
	Op   string
	Name string
	// Number from header of annotation is A (Number=A)
	NumberA bool
	// column number in bed file or ...
	Column int
	// info name in VCF. (can also be ID or FILTER).
	Field string
	// 0-based index of the file order this source is from.
	Index int
	mu    sync.Mutex
	code  string
	Vm    *goluaez.State
}

// IsNumber indicates that we expect the Source to return a number given the op
func (s *Source) IsNumber() bool {
	return s.Op == "mean" || s.Op == "max" || s.Op == "min" || s.Op == "count" || s.Op == "median" || s.Op == "sum"
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
		"ref":   v.Ref(),
		"alt":   v.Alt(),
		"vals":  vals})
	if err != nil {
		log.Printf("ERROR in at %s:%d. %s\nvals:%+v", v.Chrom(), v.Start()+1, err, vals)
		return fmt.Sprintf("err:%v", value)
	}
	return fmt.Sprintf("%v", value)
}

// PostAnnotation is created from the conf file
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
			//postannos[i].Vms[k], err = goluaez.NewState(lua)
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
	parted := make([][]interfaces.Relatable, 0, 0)
	for _, o := range r.Related() {
		s := int(o.Source()) - 1
		for len(parted) <= s {
			parted = append(parted, make([]interfaces.Relatable, 0))
		}
		parted[s] = append(parted[s], o)
	}
	return parted
}

func sameInterval(v interfaces.IVariant, other interfaces.Relatable, strict bool) (*parsers.Interval, bool) {
	if o, ok := other.(*parsers.Interval); ok {
		return o, true
	}
	if o, ok := other.(*parsers.RefAltInterval); ok {
		if !strict {
			return &o.Interval, true
		}
		return &o.Interval, interfaces.Same(v, o, strict)
	}
	return nil, false
}

func allEqual(a, b []string) bool {
	for i, aa := range a {
		if aa != b[i] {
			return false
		}
	}
	return true
}

// given the output from handleA and the alts:
// append new values to the appropriate alt.
// 22,33, A,G -> 22,33
// then XX, G -> 22,33|G
// then YY, A -> 22|YY,33|G
func byAlt(in []interface{}, qAlts []string, existing [][]string) [][]string {
	if existing == nil {
		existing = make([][]string, len(qAlts))
	}
	for i, v := range in {
		if v == "." || v == "" || v == nil {
			continue
		}
		existing[i] = append(existing[i], vcfgo.ItoS("", v))
	}
	return existing
}

// handleA converts the `val` to the correct slice of vals to match what's isnt
// qAlts and oAlts. Then length of the returned value should always be equal
// to the len of qAlts.
// query| db    | db values  | result
// C,G  | C,G | 22,23      | 22,23
// C,G  | C,T | 22,23      | 22,.
// C,G  | T,G | 22,23      | .,23
// G,C  | C,G | 22,23      | 23,22
func handleA(val interface{}, qAlts []string, oAlts []string, out []interface{}) []interface{} {
	vals := reflect.ValueOf(val)

	if vals.Kind() != reflect.Slice {
		val = []interface{}{val}
		vals = reflect.ValueOf(val)
	}
	if out == nil {
		out = make([]interface{}, len(qAlts))
		for i := 0; i < len(out); i++ {
			out[i] = "."
		}
	}

	altIdxs := make([]int, len(qAlts))
	for iq, q := range qAlts {
		var found bool
		for io, o := range oAlts {
			if q == o {
				found = true
				altIdxs[iq] = io
				break
			}
		}
		if !found {
			altIdxs[iq] = -1
		}
	}
	for i, ai := range altIdxs {
		if ai == -1 {
			continue
		}
		if ai >= vals.Len() {
			log.Printf("WARNING: out of bounds with query: %v, anno: %v, vals: %v", qAlts, oAlts, vals)
			if vals.Len() == 1 {
				out[i] = vals.Index(0).Interface()
			}
			continue
		}
		out[i] = vals.Index(ai).Interface()
	}
	return out
}

// collect applies the reduction (op) specified in src on the rels.
func collect(v interfaces.IVariant, rels []interfaces.Relatable, src *Source, strict bool) ([]interface{}, error) {
	coll := make([]interface{}, 0, len(rels))
	var val interface{}
	var valByAlt [][]string
	var finalerr error
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
				val = strings.Replace(val.(string), ";", ",", -1)
			} else if src.Field == "FILTER" {
				val = o.(interfaces.VarWrap).IVariant.(*vcfgo.Variant).Filter
				if val == "." || val == "" || val == "PASS" {
					continue
				}
				val = strings.Replace(val.(string), ";", ",", -1)
			} else {
				var err error
				val, err = o.Info().Get(src.Field)
				if err != nil {
					finalerr = err
				}
				if val == "" || val == nil {
					continue
				}
			}
			if src.Op == "by_alt" {
				// with alt uses handleA machinery and then concats each value with then
				// alternate allele.
				out := make([]interface{}, len(v.Alt()))
				handleA(val, v.Alt(), o.Alt(), out)
				valByAlt = byAlt(out, v.Alt(), valByAlt)
				continue
			}

			// special-case 'self' when the annotation has Number=A and either query or anno have multiple alts
			// so that we get the alts matched up.
			if src.NumberA && src.Op == "self" && src.Field != "ID" && src.Field != "FILTER" {
				//if (src.NumberA) && src.Op == "self" && src.Field != "ID" && src.Field != "FILTER" {
				var out []interface{}
				if len(coll) > 0 {
					out = coll[0].([]interface{})
				} else {
					out = make([]interface{}, len(v.Alt()))
					coll = append(coll, out)
				}
				if len(v.Alt()) == 1 && len(o.Alt()) == 1 && v.Alt()[0] == o.Alt()[0] {
					out[0] = val
				} else {
					handleA(val, v.Alt(), o.Alt(), out)
				}
				// coll updated in-place via out
				continue
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
			} else {

				coll = append(coll, val)
			}
		} else if o, ok := sameInterval(v, other, strict); o != nil {
			if !ok {
				continue
			}
			sval := string(o.Fields[src.Column-1])
			if src.IsNumber() {

				v, e := strconv.ParseFloat(sval, 32)
				if e != nil {
					finalerr = e
				}
				coll = append(coll, v)
			} else {
				coll = append(coll, strings.Replace(sval, ";", ",", -1))
			}
		} else if bam, ok := other.(*parsers.Bam); ok {
			if bam.MapQ() < 1 || (bam.Flags&(sam.QCFail|sam.Unmapped|sam.Duplicate|sam.Secondary) != 0) {
				continue
			}
			if src.Field == "" {
				// for coverage, we just sum the values.
				if len(coll) == 0 {
					coll = append(coll, 1)
				} else {
					coll[0] = coll[0].(int) + 1
				}
			} else {
				switch src.Field {
				case "mapq":
					coll = append(coll, bam.MapQ())
				case "seq":
					coll = append(coll, string(bam.Seq.Expand()))
				case "DP2":
					coll = append(coll, (bam.Flags&sam.Reverse) != 0)
				default:
					if src.Op != "sum" {
						if src.Op == "count" {
							// backwards compat.
							src.Op = "sum"
						} else {
							log.Fatalf("unknown field %s specifed for bam: %s with op: %s\n", src.Field, src.File, src.Op)
						}
					}
					// for coverage, we just sum the values.
					if len(coll) == 0 {
						coll = append(coll, 1)
					} else {
						coll[0] = coll[0].(int) + 1
					}
				}
			}
		} else {
			msg := fmt.Sprintf("not supported for: %T", other)
			log.Println(msg)
			coll = []interface{}{msg}
		}
	}
	if valByAlt != nil {
		for _, v := range valByAlt {
			if len(v) == 0 {
				coll = append(coll, ".")
			} else {
				coll = append(coll, strings.Join(v, "|"))
			}
		}
	}
	return coll, finalerr
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
	var e error
	for i := range a.Sources {
		src = a.Sources[i]
		if len(parted) <= src.Index {
			continue
		}

		related := parted[src.Index]
		if len(related) == 0 {
			continue
		}
		vals, err := collect(v, related, src, strict)
		if err != nil {
			e = err
		}
		src.AnnotateOne(v, vals, prefix)
	}
	return e
}

func v0len(vals []interface{}) int {
	if len(vals) == 0 {
		return -1
	}
	if v, ok := vals[0].([]interface{}); ok {
		return len(v)
	}
	return -1

}

// AnnotateOne annotates a single variant with the vals
func (s *Source) AnnotateOne(v interfaces.IVariant, vals []interface{}, prefix string) {
	if len(vals) == 0 {
		return
	}
	if s.code != "" {
		luaval := s.LuaOp(v, s.code, vals)
		if luaval == "true" || luaval == "false" && strings.Contains(s.Op, "_flag(") {
			if luaval == "true" {
				v.Info().Set(prefix+s.Name, true)
			}
		} else {
			v.Info().Set(prefix+s.Name, luaval)
		}
	} else {
		if len(vals) > 0 && s.Op == "self" && len(v.Alt()) > 1 && (len(vals) > 1 || v0len(vals) > 1) {
			// multiple annotations writing to a mult-allelic. grab the current and replace any empty
			// incoming vals with the current ones.
			// there is a lot of BS here to check that the current info values and in the incoming
			// vals are the same length.
			current, _ := v.Info().Get(prefix + s.Name)
			if current != nil && reflect.ValueOf(current).Kind() == reflect.Slice {
				cv := reflect.ValueOf(current)
				rv := reflect.ValueOf(vals)
				if rv.Kind() == reflect.Slice {
					if rv.Len() == 1 {
						vals = rv.Index(0).Interface().([]interface{})
						rv = reflect.ValueOf(vals)
					}
					if rv.Kind() == reflect.Slice && rv.Len() == cv.Len() {
						for i, v := range vals {
							if v == nil {
								newv := cv.Index(i)
								// don't set vals if we just got the zero value out of necessity.
								// this kinda works around a bug in vcfgo where it returns the empty value
								// for missing which, e.g. for float is 0.
								if reflect.Zero(newv.Type()) != newv {
									vals[i] = cv.Index(i).Interface()
								}
							}
						}
					}
				}

			}
		}
		val := Reducers[s.Op](vals)
		v.Info().Set(prefix+s.Name, val)
	}
}

// UpdateHeader does what it suggests but handles left and right ends for svs
func (s *Source) UpdateHeader(r HeaderUpdater, ends bool, htype string, number string, desc string) {
	ntype := "String"
	if s.Op == "by_alt" {
		number = "A"
		ntype = "String"
		// for 'self' and 'first', we can get the type from the header of the annotation file.
	} else if htype != "" && (s.Op == "self" || s.Op == "first") {
		ntype = htype
	} else {
		if strings.HasSuffix(s.Name, "_float") {
			s.Name = s.Name[:len(s.Name)-6]
			ntype, number = "Float", "1"
		} else if strings.HasSuffix(s.Name, "_int") {
			s.Name = s.Name[:len(s.Name)-4]
			ntype, number = "Integer", "1"
		} else if strings.HasSuffix(s.Name, "_flag") || strings.Contains(s.Op, "flag(") {
			s.Name = s.Name[:len(s.Name)-5]
			ntype, number = "Flag", "0"
		} else if s.Op == "mean" || s.Op == "max" || s.Op == "min" {
			ntype, number = "Float", "1"
		} else {
			if s.Op == "flag" {
				ntype, number = "Flag", "0"
			}
			if (strings.HasSuffix(s.File, ".bam") && s.Field == "") || s.IsNumber() {
				ntype = "Float"
			} else if s.code != "" {
				if strings.Contains(s.Op, "_flag(") {
					ntype, number = "Flag", "0"
				} else {
					ntype = "String"
				}
			}
		}
		// use Number="." for stringy ops.
		if s.Op == "uniq" || s.Op == "concat" {
			number = "."
		}
	}
	if (s.Op == "first" || s.Op == "self") && htype == ntype {
		desc = fmt.Sprintf("%s (from %s)", desc, s.File)
	} else if strings.HasSuffix(s.File, ".bam") && s.Field == "" {
		desc = fmt.Sprintf("calculated by coverage from %s", s.File)
	} else if s.Field == "DP2" {
		desc = fmt.Sprintf("calculated by coverage from %s values are numbers of forward,reverse reads", s.File)
		number = "2"
		ntype = "Integer"
	} else if s.Field != "" {
		desc = fmt.Sprintf("calculated by %s of overlapping values in field %s from %s", s.Op, s.Field, s.File)
	} else {
		desc = fmt.Sprintf("calculated by %s of overlapping values in column %d from %s", s.Op, s.Column, s.File)
	}
	r.AddInfoToHeader(s.Name, number, ntype, desc)
	if ends {
		if s.Op == "self" {
			// what to do here?
		}
		for _, end := range []string{LEFT, RIGHT} {
			d := fmt.Sprintf("%s at end %s", desc, strings.TrimSuffix(end, "_"))
			r.AddInfoToHeader(end+s.Name, number, ntype, d)
		}
	}
}

// PostAnnotate happens after everything is done.
func (a *Annotator) PostAnnotate(chrom string, start int, end int, info interfaces.Info, prefix string, id string) (error, string) {
	var e, err error
	vals := make([]interface{}, 0, 2)
	fields := make([]string, 0, 2)
	missing := make([]string, 0, 2)
	var val interface{}
	newid := ""
	for i := range a.PostAnnos {
		post := a.PostAnnos[i]
		vals = vals[:0]
		fields = fields[:0]
		missing = missing[:0]
		// lua code
		if post.code != "" {
			for _, field := range post.Fields {
				if field == "ID" {
					val = id
				} else {
					val, e = info.Get(field)
				}
				if val != nil {
					vals = append(vals, val)
					fields = append(fields, field)
				} else {
					missing = append(missing, field)
				}
				if e != nil {
					err = e
				}
			}
			// we need to try even if it didn't get all values.
			if len(vals) == 0 && len(post.Fields) > 0 {
				continue
			}

			k := 0
		out:
			// could also use fanIn where all channels send to a single
			// channel and pull from that.
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
			post.Vms[k].SetGlobal("chrom", chrom)
			post.Vms[k].SetGlobal("start", start)
			post.Vms[k].SetGlobal("stop", end)

			// need to unset missing values so we don't use those
			// from previous run.
			for _, miss := range missing {
				post.Vms[k].SetGlobal(miss, nil)
			}
			value, e := post.Vms[k].Run(post.code)
			post.mus[k] <- k
			if value == nil {
				if e != nil {
					code := post.code
					if len(code) > 40 {
						code = code[:37] + "..."
					}
					log.Printf("ERROR: in lua postannotation at %s:%d for %s.\n%s\nempty values were: %+v\nvalues were: %+v\ncode is: %s", chrom, start+1, post.Name, e, missing, vals, code)
				}
				continue
			}
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
				if post.Name == "ID" && prefix == "" {
					newid = val
				} else if e := info.Set(prefix+post.Name, val); e != nil {
					err = e
				}
			}

		} else {
			// built in function.
			// re-use vals
			var e error
			for _, field := range post.Fields {
				// ignore error when field isnt found. we expect that to occur a lot.
				val, e = info.Get(field)
				if val != nil {
					vals = append(vals, val)
				} else {
					if e != nil {
						//log.Println(field, e, vals, chrom, start, end)
						err = e
					}
				}
			}
			// run this as long as we found any of the values.
			if len(vals) != 0 {
				if post.Op == "div2" && len(vals) < 2 {
					continue
				}
				fn := Reducers[post.Op]
				if post.Name == "ID" && prefix == "" {
					newid = fmt.Sprintf("%s", fn(vals))
				} else {
					if post.Op == "delete" {
						for _, f := range post.Fields {
							info.(*vcfgo.InfoByte).Delete(prefix + f)
						}
					} else {
						info.Set(prefix+post.Name, fn(vals))
					}
				}
			}
		}
	}
	return err, newid
}

func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getSize(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return -1
	}
	fi, err := f.Stat()
	if err != nil {
		return -1
	}
	return fi.Size()
}

// Setup reads all the tabix indexes and setups up the Queryables
func (a *Annotator) Setup(query HeaderUpdater) ([]interfaces.Queryable, error) {
	files, fmap, err := a.setupStreams()
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	wg.Add(len(files))
	queryables := make([]interfaces.Queryable, len(files))
	for i, file := range files {
		go func(idx int, file string) {
			var q interfaces.Queryable
			var err error
			if strings.HasSuffix(file, ".bam") {
				q, err = parsers.NewBamQueryable(file, 2)
			} else {
				if getSize(file) > 2320303098 {
					q, err = bix.New(file, 2)
				} else {
					q, err = bix.New(file, 1)
				}
			}
			if err != nil {
				log.Fatal(err)
			}
			queryables[idx] = q
			wg.Done()
		}(i, file)

	}
	wg.Wait()

	for i, file := range files {
		if q, ok := queryables[i].(*bix.Bix); ok {
			for _, src := range fmap[file] {
				num := q.GetHeaderNumber(src.Field)
				// must set this to accurately represent multi-allelics.
				if num == "1" && src.Op == "self" {
					log.Printf("WARNING: using op 'self' when with Number='1' for '%s' from '%s' can result in out-of-order values when the query is multi-allelic", src.Field, src.File)
					log.Printf("       : this is not an issue if the query has been decomposed.")
				}
				/*
					if num == "1" && src.Op == "self" {
						num = "A"
					}
				*/
				desc := q.GetHeaderDescription(src.Field)
				src.UpdateHeader(query, a.Ends, q.GetHeaderType(src.Field), num, desc)
				src.NumberA = num == "A"
			}
		} else if _, ok := queryables[i].(*parsers.BamQueryable); ok {
			for _, src := range fmap[file] {
				htype := "String"
				if src.IsNumber() {
					htype = "Float"
				}
				src.UpdateHeader(query, a.Ends, htype, "1", "")
			}
		} else {
			log.Printf("type not known: %T\n", queryables[i])
		}
	}

	for _, post := range a.PostAnnos {
		if post.Name == "" || post.Name == "ID" || post.Name == "FILTER" {
			continue
		}
		number := "."
		if strings.Contains(strings.ToLower(post.Name), "af_") || strings.Contains(strings.ToLower(post.Name), "_af") {
			number = "A"
		}
		if post.Type == "Flag" {
			number = "0"
		}
		query.AddInfoToHeader(post.Name, number, post.Type, fmt.Sprintf("calculated field: %s", post.Name))
	}
	return queryables, nil
}

// SetupStreams takes the query stream and sets everything up for annotation.
func (a *Annotator) setupStreams() ([]string, map[string][]*Source, error) {

	lookup := make(map[string][]*Source, len(a.Sources))
	files := make([]string, 0, 4)
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

// AnnotateEnds makes a new 1-base interval for the left and one for the right end
// so that it can use the same machinery to annotate the ends and the entire interval.
// Output into the info field is prefixed with "left_" or "right_".
func (a *Annotator) AnnotateEnds(v interfaces.Relatable, ends string) error {

	var err error
	// if Both, call the interval, left, and right version to annotate.
	id := v.(*parsers.Variant).IVariant.(*vcfgo.Variant).Id()
	if ends == BOTH {
		if e := a.AnnotateOne(v, a.Strict); e != nil {
			err = e
		}
		if e, _ := a.PostAnnotate(v.Chrom(), int(v.Start()), int(v.End()), v.(interfaces.IVariant).Info(), "", id); e != nil {
			err = e
		}
		if e := a.AnnotateEnds(v, LEFT); e != nil {
			err = e
		}
		if e := a.AnnotateEnds(v, RIGHT); e != nil {
			err = e
		}
	}
	if ends == INTERVAL {
		err := a.AnnotateOne(v, a.Strict)
		err2, newid := a.PostAnnotate(v.Chrom(), int(v.Start()), int(v.End()), v.(interfaces.IVariant).Info(), "", id)
		if newid != "" {
			v.(*parsers.Variant).IVariant.(*vcfgo.Variant).Id_ = newid
		}
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
			l, r, ok = variant.CIPos()
		} else {
			l, r, ok = variant.CIEnd()
		}
		// dont reannotate same interval
		if !ok && (l == v.Start() && r == v.End()) {
			return nil
		}

		m := vcfgo.NewInfoByte([]byte(fmt.Sprintf("SVLEN=%d;END=%d", r-l-1, r)), variant.(*vcfgo.Variant).Header)
		v2 := parsers.NewVariant(&vcfgo.Variant{Chromosome: v.Chrom(), Pos: uint64(l + 1),
			Reference: "A", Alternate: []string{"<DEL>"}, Info_: m}, v.Source(), v.Related())

		err = a.AnnotateOne(v2, false, ends)
		var val interface{}
		for _, key := range v2.Info().Keys() {
			if key == "SVLEN" || key == "END" {
				continue
			}
			val, err = v2.Info().Get(key)
			variant.Info().Set(key, val)
		}
		err2, _ := a.PostAnnotate(v.Chrom(), int(l), int(r), variant.Info(), ends, id)
		if err2 != nil {
			err = err2
		}
	}
	return err
}
