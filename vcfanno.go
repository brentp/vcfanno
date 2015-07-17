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
	"github.com/brentp/vcfanno/caddcode"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
)

const VERSION = "0.0.6"

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
func (a *annotation) flatten(index int, basepath string) []*Source {
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
			log.Fatalf("[flatten] unable to open file: %s in %s\n", a.File, basepath)
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

// CaddIdx is same as annotation, but has extra fields due to custom nature of CADD score.
type CaddIdx struct {
	annotation
	idx     caddcode.Index
	Sources []*Source
}

type Config struct {
	Annotation []annotation
	CaddIdx    CaddIdx
	// base path to prepend to all files.
	Base string
}

func (c Config) Sources() []*Source {
	s := make([]*Source, 0)
	for i, a := range c.Annotation {
		s = append(s, a.flatten(i, c.Base)...)
	}
	return s
}

// Cadd parses the cadd fields and updates the vcf Header.
func (c Config) Cadd(h *vcfgo.Header, ends bool) *CaddIdx {
	if &c.CaddIdx == nil || c.CaddIdx.File == "" {
		return nil
	}
	c.CaddIdx.Sources = c.CaddIdx.annotation.flatten(-1, c.Base)
	c.CaddIdx.idx = caddcode.Reader(c.CaddIdx.File)
	for _, src := range c.CaddIdx.Sources {
		src.UpdateHeader(h, ends)
		h.Infos[src.Name].Number = "A"
	}
	return &c.CaddIdx
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
	fmt.Fprintf(os.Stderr, `
vcfanno version %s

see: https://github.com/brentp/vcfanno
`, VERSION)

	ends := flag.Bool("ends", false, "annotate the start and end as well as the interval itself.")
	notstrict := flag.Bool("permissive-overlap", false, "annotate with an overlapping variant even it doesn't"+
		" share the same ref and alt alleles. Default is to require exact match between variants.")
	js := flag.String("js", "", "optional path to a file containing custom javascript functions to be used as ops")
	lexsort := flag.Bool("lexicographical", false, "expect chromosomes in order of 1,10,11 ... 19, 2, 20... "+
		" default is 1, 10, 11, ..., 19, 2, 20... . All files must be in the same order.")
	base := flag.String("base-path", "", "optional base-path to prepend to annotation files in the config")
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
	if !(xopen.Exists(queryFile) || queryFile == "-") {
		fmt.Fprintf(os.Stderr, "\nERROR: can't find query file: %s\n", queryFile)
		os.Exit(2)
	}

	var config Config
	if _, err := toml.DecodeFile(inFiles[0], &config); err != nil {
		panic(err)
	}
	config.Base = *base
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
	var a = NewAnnotator(sources, jsString, *ends, strict, !*lexsort)

	var out io.Writer = os.Stdout
	defer os.Stdout.Close()

	streams, rdr := a.SetupStreams(queryFile)
	var cadd *CaddIdx

	if nil != rdr { // it was vcf, print the header
		var err error
		cadd = config.Cadd(rdr.Header, a.Ends)
		out, err = vcfgo.NewWriter(out, rdr.Header)
		if err != nil {
			log.Fatal(err)
		}

	} else {
		bw := bufio.NewWriter(out)
		defer bw.Flush()
		out = bw
	}
	start := time.Now()
	n := 0

	for interval := range a.Annotate(streams...) {
		caddAnno(cadd, interval)
		fmt.Fprintf(out, "%s\n", interval)
		n++
	}
	printTime(start, n)
	if rdr != nil {
		if e := rdr.Error(); e != nil {
			log.Println(e)
		}
	}

}

// if the cadd index was requested, annotate the variant.
func caddAnno(cadd *CaddIdx, interval irelate.Relatable) {
	if cadd != nil {
		if v, ok := interval.(*irelate.Variant); ok {
			for _, src := range cadd.Sources {
				vals := make([][]interface{}, len(v.Alt))
				vStr := make([]string, len(v.Alt))
				// handle multiple alts.
				for iAlt, alt := range v.Alt {
					vals[iAlt] = make([]interface{}, 0)
					// e.g ref is ACTGC, alt is C, report list of changes from ref[i] to C.
					if len(alt) == 1 {
						for pos := int(interval.Start()) + 1; pos <= int(interval.End()); pos++ {
							score, err := cadd.idx.At(interval.Chrom(), pos, alt)
							if err != nil {
								log.Println("cadd error:", err)
							}
							vals[iAlt] = append(vals[iAlt], score)
						}
					} else {
						// take the flanking positions.
						for j, pos := range []int{int(interval.Start() + 1), int(interval.End())} {
							k := 0
							if j > 0 {
								k = len(alt) - 1
							}
							score, err := cadd.idx.At(interval.Chrom(), pos, alt[k:k+1])
							if err != nil {
								log.Println("cadd error:", err)
							}
							vals[iAlt] = append(vals[iAlt], score)
						}
					}
					// TODO: handle ends (left, right end of SV?)
					src.AnnotateOne(v, vals[iAlt], "")
					vStr[iAlt] = string(v.Info.SGet(src.Name))
					v.Info.Delete(src.Name)
				}
				v.Info.Set(src.Name, strings.Join(vStr, ","))
			}
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
