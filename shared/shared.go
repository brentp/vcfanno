package shared

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	. "github.com/brentp/vcfanno/api"
	"github.com/brentp/vcfanno/caddcode"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
)

// CaddIdx is same as annotation, but has extra fields due to custom nature of CADD score.
type CaddIdx struct {
	Annotation
	Idx     caddcode.Index
	Sources []*Source
}

type Config struct {
	Annotation []Annotation
	CaddIdx    CaddIdx
	// base path to prepend to all files.
	Base string
}

// annotation holds information about the annotation files parsed from the toml config.
type Annotation struct {
	File    string
	Ops     []string
	Fields  []string
	Columns []int
	// the names in the output.
	Names []string
}

// Flatten turns an annotation into a slice of Sources. Pass in the index of the file.
// having it as a Source makes the code cleaner, but it's simpler for the user to
// specify multiple ops per file in the toml config.
func (a *Annotation) Flatten(index int, basepath string) []*Source {
	if len(a.Ops) == 0 {
		if !strings.HasSuffix(a.File, ".bam") {
			log.Fatalf("no ops specified for %s\n", a.File)
		}
		// auto-fill bam to count.
		a.Ops = make([]string, len(a.Names))
		for i := range a.Names {
			a.Ops[i] = "count"
		}
	}
	if len(a.Columns) == 0 && len(a.Fields) == 0 {
		// index of -1 is cadd.
		if !strings.HasSuffix(a.File, ".bam") && index != -1 {
			log.Fatalf("no columns or fields specified for %s\n", a.File)
		}
		// auto-fill bam to count.
		if len(a.Fields) == 0 {
			a.Columns = make([]int, len(a.Names))
			for i := range a.Names {
				a.Columns[i] = 1
			}
		}
	}
	if !(xopen.Exists(a.File) || a.File == "-") {
		if basepath != "" {
			a.File = basepath + "/" + a.File
		}
		if !(xopen.Exists(a.File) || a.File == "-") {
			log.Fatalf("[Flatten] unable to open file: %s in %s\n", a.File, basepath)
		}
	}

	n := len(a.Ops)
	sources := make([]*Source, n)
	for i := 0; i < n; i++ {
		isjs := strings.HasPrefix(a.Ops[i], "js:")
		if !isjs {
			if _, ok := Reducers[a.Ops[i]]; !ok {
				log.Fatalf("requested op not found: %s for %s\n", a.Ops[i], a.File)
			}
		}
		op := a.Ops[i]
		if len(a.Names) == 0 {
			a.Names = a.Fields
		}
		sources[i] = &Source{File: a.File, Op: op, Name: a.Names[i], Index: index}
		if nil != a.Fields {
			sources[i].Field = a.Fields[i]
			sources[i].Column = -1
		} else {
			sources[i].Column = a.Columns[i]
		}
	}
	return sources
}

func (c Config) Sources() []*Source {
	s := make([]*Source, 0)
	for i, a := range c.Annotation {
		s = append(s, a.Flatten(i, c.Base)...)
	}
	return s
}

func CheckAnno(a *Annotation) error {
	if strings.HasSuffix(a.File, ".bam") {
		if nil == a.Columns && nil == a.Fields {
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

// Cadd parses the cadd fields and updates the vcf Header.
func (c Config) Cadd(h *vcfgo.Header, ends bool) *CaddIdx {
	if &c.CaddIdx == nil || c.CaddIdx.File == "" {
		return nil
	}
	c.CaddIdx.Sources = c.CaddIdx.Annotation.Flatten(-1, c.Base)
	c.CaddIdx.Idx = caddcode.Reader(c.CaddIdx.File)
	for _, src := range c.CaddIdx.Sources {
		src.UpdateHeader(h, ends)
		h.Infos[src.Name].Number = "A"
	}
	return &c.CaddIdx
}

func ReadJs(js string) string {
	var jsString string
	if js != "" {
		jsReader, err := xopen.Ropen(js)
		if err != nil {
			log.Fatal(err)
		}
		jsBytes, err := ioutil.ReadAll(jsReader)
		if err != nil {
			log.Fatal(err)
		}
		jsString = string(jsBytes)
	} else {
		jsString = ""
	}
	return jsString
}
