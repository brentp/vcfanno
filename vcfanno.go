// vcfanno is a command-line application and an api for annotating intervals (bed or vcf).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/brentp/irelate"
	. "github.com/brentp/vcfanno/api"
	caddencode "github.com/brentp/vcfanno/caddencode"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
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

// flatten turns an annotation into a slice of Sources. Pass in the index of the file.
// having it as a Source makes the code cleaner, but it's simpler for the user to
// specify multiple ops per file in the toml config.
func (a *annotation) flatten(index int) []*Source {
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
		if !strings.HasSuffix(a.File, ".bam") {
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

type CaddIdx struct {
	File  string
	Ops   []string
	Names []string
	idx   caddencode.Index
}

type Config struct {
	Annotation []annotation
	Caddidx    CaddIdx
}

func (c Config) Sources() []*Source {
	s := make([]*Source, 0)
	for i, a := range c.Annotation {
		s = append(s, a.flatten(i)...)
	}
	return s
}

func (c Config) Cadd() *CaddIdx {
	if &c.Caddidx == nil || c.Caddidx.File == "" {
		return nil
	}
	log.Println(c.Caddidx.Ops, c.Caddidx.Names)
	c.Caddidx.idx = caddencode.Reader(c.Caddidx.File)
	return &c.Caddidx
}

func checkAnno(a *annotation) error {
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

func readJs(js string) string {
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

func main() {

	ends := flag.Bool("ends", false, "annotate the start and end as well as the interval itself.")
	notstrict := flag.Bool("permissive-overlap", false, "annotate with an overlapping variant even it doesn't"+
		" share the same ref and alt alleles. Default is to require exact match between variants.")
	js := flag.String("js", "", "optional path to a file containing custom javascript functions to be used as ops")
	natsort := flag.Bool("natural-sort", false, "expect chromosomes in order of 1, 2, ..., 9, 10, 11..."+
		" default is 1, 10, 11, ..., 19, 2, 20...")
	flag.Parse()
	inFiles := flag.Args()
	if len(inFiles) != 2 {
		fmt.Printf(`Usage:
%s config.toml intput.vcf > annotated.vcf

`, os.Args[0])
		flag.PrintDefaults()
		return
	}
	queryFile := inFiles[1]

	var config Config
	if _, err := toml.DecodeFile(inFiles[0], &config); err != nil {
		panic(err)
	}
	for _, a := range config.Annotation {
		err := checkAnno(&a)
		if err != nil {
			log.Fatal("checkAnno err:", err)
		}
	}
	sources := config.Sources()
	log.Printf("found %d sources from %d files\n", len(sources), len(config.Annotation))

	jsString := readJs(*js)
	strict := !*notstrict
	var a = NewAnnotator(sources, jsString, *ends, strict, *natsort)

	var out io.Writer = os.Stdout

	streams, rdr := a.SetupStreams(queryFile)
	if nil != rdr { // it was vcf, print the header
		var err error
		out, err = vcfgo.NewWriter(out, rdr.Header)
		if err != nil {
			log.Fatal(err)
		}

	} else {
		out = bufio.NewWriter(out)
	}
	start := time.Now()
	n := 0

	cadd := config.Cadd()

	for interval := range a.Annotate(streams...) {
		caddAnno(cadd, interval)
		fmt.Fprintf(out, "%s\n", interval)
		n++
	}
	printTime(start, n)
	e := rdr.Error()
	if e != nil {
		log.Println(e)
	}

}

// if the cadd index was requested, annotate the variant.
func caddAnno(cadd *CaddIdx, interval irelate.Relatable) {
	if cadd != nil {
		// TODO: tie this in with api machinery
		// or at least loop over and apply op.
		// TODO: update header.
		v := interval.(*irelate.Variant)
		score, err := cadd.idx.At(interval.Chrom(), int(interval.Start())+1, v.Alt[0])
		if err != nil {
			log.Println("cadd errro:", err)
		} else {
			v.Info.Add(cadd.Names[0], score)
		}
	}
}

func printTime(start time.Time, n int) {
	dur := time.Since(start)
	duri, duru := dur.Seconds(), "second"
	if duri > float64(600) {
		duri, duru = dur.Minutes(), "minute"
	}
	log.Printf("annotated %d variants in %.2f %ss (%.1f / %s)", n, duri, duru, float64(n)/duri, duru)
}
