// vcfanno is a command-line application and an api for annotating intervals (bed or vcf).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/brentp/bix"
	"github.com/brentp/irelate"
	"github.com/brentp/irelate/interfaces"
	. "github.com/brentp/vcfanno/api"
	vhttp "github.com/brentp/vcfanno/http"
	. "github.com/brentp/vcfanno/shared"
	"github.com/brentp/vcfgo"
	"github.com/brentp/xopen"
)

const VERSION = "0.0.8"

func main() {
	fmt.Fprintf(os.Stderr, `
=============================================
vcfanno version %s [built with %s]

see: https://github.com/brentp/vcfanno
=============================================
`, VERSION, runtime.Version())

	if len(os.Args) > 1 && os.Args[1] == "server" {
		vhttp.Server()
		os.Exit(0)
	}

	ends := flag.Bool("ends", false, "annotate the start and end as well as the interval itself.")
	notstrict := flag.Bool("permissive-overlap", false, "annotate with an overlapping variant even it doesn't"+
		" share the same ref and alt alleles. Default is to require exact match between variants.")
	js := flag.String("js", "", "optional path to a file containing custom javascript functions to be used as ops")
	lexsort := flag.Bool("lexicographical", false, "expect chromosomes in order of 1,10,11 ... 19, 2, 20... "+
		" default is 1, 10, 11, ..., 19, 2, 20... . All files must be in the same order.")
	region := flag.String("region", "", "optional region (chrom:start-end) to restrict annnotation. Useful for parallelization")
	base := flag.String("base-path", "", "optional base-path to prepend to annotation files in the config")
	flag.Parse()
	inFiles := flag.Args()
	if len(inFiles) != 2 {
		fmt.Printf(`Usage:
%s config.toml intput.vcf > annotated.vcf

To run a server:

%s server

`, os.Args[0], os.Args[0])
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
		err := CheckAnno(&a)
		if err != nil {
			log.Fatal("CheckAnno err:", err)
		}
	}
	sources, e := config.Sources()
	if e != nil {
		log.Fatal(e)
	}

	log.Printf("found %d sources from %d files\n", len(sources), len(config.Annotation))

	jsString := ReadJs(*js)
	strict := !*notstrict
	var a = NewAnnotator(sources, jsString, *ends, strict, !*lexsort, *region)

	var out io.Writer = os.Stdout
	defer os.Stdout.Close()

	var rdr *vcfgo.Reader
	var err error
	var queryStream irelate.RelatableChannel
	if strings.HasSuffix(queryFile, ".bed") || strings.HasSuffix(queryFile, ".bed.gz") {
		queryStream, err = irelate.Streamer(queryFile, "")
		if err != nil {
			log.Fatal(err)
		}
	} else {

		var q io.Reader
		if *region == "" {
			q, err = xopen.XReader(queryFile)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			bx, err := bix.New(queryFile)
			if err != nil {
				log.Fatal(err)
			}
			chrom, start, end, err := irelate.RegionToParts(*region)
			if err != nil {
				log.Fatal(err)
			}
			q, err = bx.Query(chrom, start, end)
			if err != nil {
				log.Fatal(err)
			}
		}

		rdr = irelate.Vopen(q)
		queryStream = irelate.StreamVCF(rdr)
		a.UpdateHeader(rdr.Header)
	}

	streams, getters, err := a.SetupStreams(queryStream)
	if err != nil {
		log.Fatal(err)
	}
	var cadd *CaddIdx

	if nil != rdr { // it was vcf, print the header
		var err error
		cadd, err = config.Cadd(rdr.Header, a.Ends)
		if err != nil {
			log.Fatal(err)
		}

		out, err = vcfgo.NewWriter(out, rdr.Header)
		if err != nil {
			log.Fatal(err)
		}

	} else {
		if &config.CaddIdx != nil || config.CaddIdx.File != "" {
			log.Println("can't currently use CADD on BED files")
		}
		bw := bufio.NewWriter(out)
		defer bw.Flush()
		out = bw
	}
	start := time.Now()
	n := 0

	if os.Getenv("IRELATE_PROFILE") == "TRUE" {
		log.Println("profiling to: irelate.pprof")
		f, err := os.Create("irelate.pprof")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	for interval := range a.Annotate(streams, getters) {
		cadd3(cadd, interval)
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

func cadd3(cadd *CaddIdx, interval interfaces.Relatable) {
	if cadd == nil {
		return
	}
	var v *irelate.Variant
	var ok bool

	if v, ok = interval.(*irelate.Variant); !ok {
		return
	}
	caddAnno(cadd, v, "")
	cip0, cip1, okp := v.IVariant.(CIFace).CIPos()
	cie0, cie1, oke := v.IVariant.(CIFace).CIEnd()
	ends := []string{LEFT, RIGHT}
	oks := []bool{okp, oke}
	for i, lr := range [][]uint32{{cip0, cip1}, {cie0, cie1}} {
		if !oks[i] {
			continue
		}
		l, r := lr[0], lr[1]
		prefix := ends[i]

		if l == v.Start() && r == v.End() {
			return
		}

		m := vcfgo.NewInfoByte(fmt.Sprintf("SVLEN=%d", r-l-1), nil)
		v2 := irelate.NewVariant(&vcfgo.Variant{Chromosome: v.Chrom(), Pos: uint64(v.Start() + 1),
			Reference: "A", Alternate: []string{"<DEL>"}, Info_: m}, v.Source(), nil)

		caddAnno(cadd, v2, prefix)
		for _, key := range v2.Info().Keys() {
			if key == "SVLEN" {
				continue
			}
			val, err := v2.Info().Get(key)
			if err != nil {
				log.Println(err)
			}
			v.Info().Set(key, val)
		}

	}
}

// if the cadd index was requested, annotate the variant.
func caddAnno(cadd *CaddIdx, v *irelate.Variant, prefix string) {
	if cadd == nil {
		return
	}
	if v.End()-v.Start() > 100000 {
		log.Printf("skipping long variant at %s:%d (%d bases)\n", v.Chrom(), v.Start()-1, v.End()-v.Start())
		return
	}

	for _, src := range cadd.Sources {
		vals := make([][]interface{}, len(v.Alt()))
		vStr := make([]string, len(v.Alt()))
		// handle multiple alts.
		for iAlt, alt := range v.Alt() {
			vals[iAlt] = make([]interface{}, 0)
			// report list of changes from ref[i] to C.
			for pos := int(v.Start()) + 1; pos <= int(v.End()); pos++ {
				score, err := cadd.Idx.At(v.Chrom(), pos, alt)
				if err != nil && (alt[0] != '<' && !(alt[0] == ']' || alt[0] == '[' || alt[len(alt)-1] == ']' || alt[len(alt)-1] == '[')) {
					log.Println("cadd error:", err)
				}
				vals[iAlt] = append(vals[iAlt], score)
			}
			src.AnnotateOne(v, vals[iAlt], prefix)
			var err error
			var val interface{}
			val, err = v.Info().Get(prefix + src.Name)
			vStr[iAlt] = fmt.Sprintf("%.3f", val)
			if err != nil {
				log.Println(err)
			}
		}
		v.Info().Set(prefix+src.Name, strings.Join(vStr, ","))
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
