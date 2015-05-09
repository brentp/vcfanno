package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// annotation holds information about the annotation files parsed from the toml config.
type annotation struct {
	File    string
	Ops     []string
	Fields  []string
	Columns []int
	// the names in the output.
	Names []string
}

// turn an annotation into a slice of Sources. Pass in the index of the file.
// having it as a Source makes the code cleaner, but it's simpler for the user to
// specify multiple ops per file in the toml config.
func (a *annotation) flatten(index int) []Source {
	n := len(a.Ops)
	sources := make([]Source, n)
	for i := 0; i < n; i++ {
		isjs := strings.HasPrefix(a.Ops[i], "js:")
		op := a.Ops[i]
		if isjs {
			op = op[3:]
		}
		sources[i] = Source{File: a.File, Op: op, Name: a.Names[i], Index: index, IsJs: isjs}
		if nil != a.Fields {
			sources[i].Field = a.Fields[i]
		} else {
			sources[i].Column = a.Columns[i]
		}
	}
	return sources
}

type Config struct {
	Annotation []annotation
	Js         string // custom js funcs to pre-populate otto.
}

/*
// updateHeader adds a new info item to the header for each new annotation
func updateHeader(files []annotation, j int, query *vcfgo.Reader, ends bool) {
	cfg := files[j]
	for i, name := range cfg.Names {
		ntype := "Character"
		if strings.HasSuffix(cfg.File, ".bam") || cfg.isNumber(i) {
			ntype = "Float"
		}
		var desc string
		// write the VCF header.
		if strings.HasSuffix(cfg.File, ".bam") {
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

const LEFT = "left_"
const RIGHT = "right_"
const BOTH = "both_"
const INTERVAL = ""

func updateInfo(iv *irelate.Variant, sep [][]irelate.Relatable, files []annotation, ends string, strict bool) {
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
			// removed js
			v.Info.Add(ends+cfg.Names[i], Reducers[cfg.Ops[i]](vals))
		}
	}
}
*/

func checkAnno(a *annotation) error {
	if strings.HasSuffix(a.File, ".bam") {
		if nil == a.Columns {
			a.Columns = []int{1}
		}
		if nil == a.Ops {
			a.Ops = []string{"count"}
		}
	}
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
	} else {
		// Fields: VCF
		if a.Columns != nil {
			if strings.HasSuffix(a.File, ".bam") {
				a.Columns = make([]int, len(a.Ops))
			} else {
				return fmt.Errorf("specify only 'fields' or 'columns' not both %s", a.File)
			}
		}
		if len(a.Ops) != len(a.Fields) {
			return fmt.Errorf("must specify same # of 'fields' as 'ops' for %s", a.File)
		}
	}
	if len(a.Names) == 0 {
		a.Names = a.Fields
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

	var config Config
	if _, err := toml.DecodeFile(inFiles[0], &config); err != nil {
		panic(err)
	}
	sources := make([]Source, 0, len(config.Annotation))
	for i, a := range config.Annotation {
		err := checkAnno(&a)
		if err != nil {
			log.Fatal("checkAnno err:", err)
		}
		sources = append(sources, a.flatten(i)...)
	}
	strict := !*notstrict
	var a = NewAnnotator(sources, config.Js, *ends, strict)
	a.Annotate(inFiles[1])
	//Anno(inFiles[1], config, os.Stdout, *ends, strict)
}
