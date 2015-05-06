package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/brentp/irelate"
	"github.com/brentp/vcfgo"
	"github.com/robertkrimen/otto"
)

// anno holds information about the annotation files parsed from the toml config.
type anno struct {
	File    string
	Ops     []string
	Fields  []string
	Columns []int
	// the names in the output.
	Names []string
}

func (a *anno) isNumber(idx int) bool {
	return a.Ops[idx] == "mean" || a.Ops[idx] == "max" || a.Ops[idx] == "min" || a.Ops[idx] == "count"
}

type Annotations struct {
	Annotation []anno
}

// can get a bam without an op. default it to 'count'
func fixBam(as []anno, j int) anno {
	a := as[j]
	if strings.HasSuffix(a.File, ".bam") {
		as[j].Columns = []int{0}
		as[j].Ops = []string{"count"}
	}
	return as[j]
}

// updateHeader adds a new info item to the header for each new annotation
func updateHeader(files []anno, j int, query *vcfgo.Reader, ends bool) {
	cfg := files[j]
	for i, name := range cfg.Names {
		ntype := "Character"
		if strings.HasSuffix(cfg.File, ".bam") || cfg.isNumber(i) {
			ntype = "Float"
		}
		var desc string
		// write the VCF header.
		if strings.HasSuffix(cfg.File, ".bam") {
			cfg = fixBam(files, j)
			desc = fmt.Sprintf("calculated by coverage from %s", cfg.File)
		} else if cfg.Fields != nil {
			desc = fmt.Sprintf("calculated by %s of overlapping values in field %s from %s", cfg.Ops[i], cfg.Fields[i], cfg.File)
		} else {
			desc = fmt.Sprintf("calculated by %s of overlapping values in column %d from %s", cfg.Ops[i], cfg.Columns[i], cfg.File)
		}
		query.Header.Infos[name] = &vcfgo.Info{Id: name, Number: "1", Type: ntype, Description: desc}
		if ends {
			for _, end := range []string{LEFT, RIGHT} {
				query.Header.Infos[end+name] = &vcfgo.Info{Id: end + name, Number: "1", Type: ntype,
					Description: fmt.Sprintf("%s at %s end", desc, strings.TrimSuffix(end, "_"))}
			}
		}

	}
}

// if the annotation file is a bam, they aren't require to specify op or field, so we artificially
// set it here. this is only called if query is a bed file.
func fixBams(files []anno, j int) {
	cfg := files[j]
	if strings.HasSuffix(cfg.File, ".bam") {
		cfg = fixBam(files, j)
	}
}

// Anno takes a query vcf and a set of annotations, and writes to outw.
// If ends is specified, then the query is annotated for start, end and the interval itself.
// if strict is true a variant is only annotated with another variant if they share the same
// position, the same ref allele, and at least 1 alt allele.
func Anno(queryFile string, configs Annotations, outw io.Writer, ends bool, strict bool) {

	files := configs.Annotation

	isBed := false
	streams := make([]irelate.RelatableChannel, 0)
	var query *vcfgo.Reader
	if strings.HasSuffix(queryFile, ".bed") || strings.HasSuffix(queryFile, ".bed.gz") {
		isBed = true
		streams = append(streams, irelate.Streamer(queryFile))
	} else {
		query = irelate.Vopen(queryFile)
		streams = append(streams, irelate.StreamVCF(query))
	}

	for j, cfg := range files {
		if cfg.Names == nil {
			if cfg.Fields == nil {
				log.Fatal("must specify either fields or names")
			}
			cfg.Names = cfg.Fields
			files[j].Names = cfg.Fields
		}
		if !isBed {
			updateHeader(files, j, query, ends)
		} else {
			fixBams(files, j)
		}
		streams = append(streams, irelate.Streamer(cfg.File))
	}
	var out io.Writer
	var err error
	if !isBed {
		out, err = vcfgo.NewWriter(outw, query.Header)
		if err != nil {
			panic(err)
		}
	} else {
		out = bufio.NewWriter(outw)
	}

	annotateEnds := INTERVAL
	if ends {
		annotateEnds = BOTH
	}
	// the *Prefix functions let 'chr1' == '1'
	for interval := range irelate.IRelate(irelate.CheckOverlapPrefix, 0, irelate.LessPrefix, streams...) {
		if variant, ok := interval.(*irelate.Variant); ok {
			if len(variant.Related()) > 0 {
				sep := Partition(variant, len(streams)-1)
				updateInfo(variant, sep, files, annotateEnds, strict)

			}
			fmt.Fprintln(out, variant)
		} else {
			bed, ok := interval.(*irelate.Interval)
			if !ok {
				log.Fatalf("not an interval or a variant: %v", interval)
			}
			if len(interval.Related()) > 0 {
				sep := Partition(bed, len(streams)-1)
				updateBed(bed, sep, files, annotateEnds)
			}
			fmt.Fprintln(out, bed)

		}
	}
}

const LEFT = "left_"
const RIGHT = "right_"
const BOTH = "both_"
const INTERVAL = ""

func updateInfo(iv *irelate.Variant, sep [][]irelate.Relatable, files []anno, ends string, strict bool) {
	for i, cfg := range files {

		v := iv.Variant
		var valsByFld [][]interface{}
		if ends == BOTH {
			// we want to annotate ends of the interval independently.
			// note that we automatically set strict to false.
			updateInfo(iv, sep, files, INTERVAL, false)
			// only do this if the variant is longer than 1 base.
			if iv.End()-iv.Start() > 1 {
				updateInfo(iv, sep, files, LEFT, false)
				updateInfo(iv, sep, files, RIGHT, false)
			}
			return
		} else if ends == INTERVAL {
			valsByFld = Collect(iv, sep[i], cfg, strict)
		} else if ends == LEFT {
			// hack. We know end() is calculated as length of ref. so we set it to have len 1 temporarily.
			// and we use the variant itself so the info is updated in-place.
			ref := v.Ref
			alt := v.Alt
			v.Ref = "A"
			v.Alt = []string{"T"}
			//log.Println("left:", v.Start(), v.End())
			valsByFld = Collect(iv, sep[i], cfg, strict)
			v.Ref, v.Alt = ref, alt
		} else if ends == RIGHT {
			// artificially set end to be the right end of the interval.
			pos, ref, alt := v.Pos, v.Ref, v.Alt
			v.Pos = uint64(v.End())
			v.Ref, v.Alt = "A", []string{"T"}
			//log.Println("right:", v.Start(), v.End())
			valsByFld = Collect(iv, sep[i], cfg, strict)
			v.Pos, v.Ref, v.Alt = pos, ref, alt

		}

		for i, vals := range valsByFld {
			// currently we don't do anything without overlaps.
			if len(vals) == 0 {
				continue
			}
			if strings.HasPrefix(cfg.Ops[i], "js:") {
				// TODO when we see js: in the input, we can just make a custom reducer.
				js := cfg.Ops[i][3:]
				v.Info.Add(ends+cfg.Names[i], otto_run(v, js, vals))
			} else {
				v.Info.Add(ends+cfg.Names[i], Reducers[cfg.Ops[i]](vals))
			}
		}
	}
}

var vm = otto.New()

func otto_run(v *vcfgo.Variant, js string, vals []interface{}) interface{} {
	value, err := vm.Run(js)
	vm.Set("vals", vals)
	if err != nil {
		log.Println("js error:", err)
	}
	val, err := value.ToString()
	if err != nil {
		log.Println("js error:", err)
		val = fmt.Sprintf("error:%s", err)
	}
	return val
}

func updateBed(bed *irelate.Interval, sep [][]irelate.Relatable, files []anno, ends string) {
	// create irelate.Variant with start, end equal to bed
	// then add INFO to fields in bed.
	m := make(vcfgo.InfoMap)
	m["__order"] = []string{}
	m["SVLEN"] = int(bed.End()-bed.Start()) - 1
	v := &vcfgo.Variant{Chromosome: bed.Chrom(), Pos: uint64(bed.Start() + 1), Ref: "A",
		Alt: []string{"<DEL>"}, Info: m}
	if v.End() != bed.End() {
		log.Fatalf("ends: %d and %d should be the same", v.End(), bed.End())
	}
	iv := irelate.NewVariant(v, bed.Source(), bed.Related())
	updateInfo(iv, sep, files, ends, false)
	delete(m, "SVLEN")
	bed.Fields = append(bed.Fields, iv.Info.String())
}

func checkAnno(a anno) error {
	if a.Fields == nil {
		// Columns: BED/BAM
		if a.Columns == nil {
			return fmt.Errorf("must specify either 'fields' or 'columns' for %s", a.File)
		}
		if len(a.Ops) != len(a.Columns) && !strings.HasSuffix(a.File, ".bam") {
			return fmt.Errorf("must specify same # of 'columns' as 'ops' for %s", a.File)
		}
		if len(a.Names) != len(a.Columns) && !strings.HasSuffix(a.File, ".bam") {
			return fmt.Errorf("must specify same # of 'names' as 'ops' for %s", a.File)
		}
	}
	// Fields: VCF
	if a.Columns != nil {
		return fmt.Errorf("specify only 'fields' or 'columns' not both %s", a.File)
	}
	if len(a.Ops) != len(a.Fields) {
		return fmt.Errorf("must specify same # of 'fields' as 'ops' for %s", a.File)
	}
	return nil
}

func main() {

	ends := flag.Bool("ends", false, "annotate the start and end as well as the interval itself.")
	notstrict := flag.Bool("permissive-overlap", false, "allow variants to be annotated by another even if the don't"+
		"share the same ref and alt alleles. Default is to require exact match between variants.")
	flag.Parse()
	inFiles := flag.Args()
	if len(inFiles) != 2 {
		fmt.Printf(`Usage:
%s config.toml intput.vcf > annotated.vcf

`, os.Args[0])
		flag.PrintDefaults()
		return
	}

	var config Annotations
	if _, err := toml.DecodeFile(inFiles[0], &config); err != nil {
		panic(err)
	}
	for _, a := range config.Annotation {
		checkAnno(a)
	}
	strict := !*notstrict
	Anno(inFiles[1], config, os.Stdout, *ends, strict)
}
