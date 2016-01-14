package shared

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	. "github.com/brentp/vcfanno/api"
	"github.com/brentp/xopen"
)

type Config struct {
	Annotation     []Annotation
	PostAnnotation []PostAnnotation
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
func (a *Annotation) Flatten(index int, basepath string) ([]*Source, error) {
	if len(a.Ops) == 0 {
		if !strings.HasSuffix(a.File, ".bam") {
			return nil, fmt.Errorf("no ops specified for %s\n", a.File)
		}
		// auto-fill bam to count.
		a.Ops = make([]string, len(a.Names))
		for i := range a.Names {
			a.Ops[i] = "count"
		}
	}
	if len(a.Columns) == 0 && len(a.Fields) == 0 {
		if !strings.HasSuffix(a.File, ".bam") {
			return nil, fmt.Errorf("no columns or fields specified for %s\n", a.File)
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
			return nil, fmt.Errorf("[Flatten] unable to open file: %s in %s\n", a.File, basepath)
		}
	}

	n := len(a.Ops)
	sources := make([]*Source, n)
	for i := 0; i < n; i++ {
		isLua := strings.HasPrefix(a.Ops[i], "lua:")
		if !isLua {
			if _, ok := Reducers[a.Ops[i]]; !ok {
				return nil, fmt.Errorf("requested op not found: %s for %s\n", a.Ops[i], a.File)
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
	return sources, nil
}

func (c Config) Sources() ([]*Source, error) {
	annos := c.Annotation
	for i, a := range annos {
		if !xopen.Exists(a.File) && a.File != "-" {
			a.File = c.Base + "/" + a.File
			annos[i] = a
		}
	}
	s := make([]*Source, 0)
	for i, a := range annos {
		flats, err := a.Flatten(i, c.Base)
		if err != nil {
			return nil, err
		}
		s = append(s, flats...)
	}
	return s, nil
}

func CheckPostAnno(p *PostAnnotation) error {
	if len(p.Fields) == 0 {
		log.Println("warning: no specified 'fields' for postannotation:", p.Name)
	}
	if p.Op == "" {
		return fmt.Errorf("must specify an 'op' for postannotation")
	}
	if p.Name == "" {
		return fmt.Errorf("must specify a 'name' for postannotation")
	}
	if !(p.Type == "Float" || p.Type == "String" || p.Type == "Integer" || p.Type == "Flag") {
		return fmt.Errorf("must specify a type for postannotation that is 'Flag', 'Float', 'Integer' or 'String'")
	}
	return nil
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

func ReadLua(lua string) string {
	var luaString string
	if lua != "" {
		luaReader, err := xopen.Ropen(lua)
		if err != nil {
			log.Fatal(err)
		}
		luaBytes, err := ioutil.ReadAll(luaReader)
		if err != nil {
			log.Fatal(err)
		}
		luaString = string(luaBytes)
	} else {
		luaString = ""
	}
	return luaString
}
