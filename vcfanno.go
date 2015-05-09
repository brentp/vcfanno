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

const LEFT = "left_"
const RIGHT = "right_"
const BOTH = "both_"
const INTERVAL = ""

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
}
