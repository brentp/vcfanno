package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/brentp/irelate"
	"github.com/brentp/vcfgo"
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
func updateHeader(files []anno, j int, query *vcfgo.Reader) {
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

	}
}

func Anno(queryVCF string, configs Annotations, outw io.Writer) {

	files := configs.Annotation

	streams := make([]irelate.RelatableChannel, 0)
	query := irelate.Vopen(queryVCF)

	streams = append(streams, irelate.StreamVCF(query))

	for j, cfg := range files {
		if cfg.Names == nil {
			if cfg.Fields == nil {
				log.Fatal("must specify either fields or names")
			}
			cfg.Names = cfg.Fields
			files[j].Names = cfg.Fields
		}
		updateHeader(files, j, query)
		if strings.HasSuffix(cfg.File, ".vcf.gz") || strings.HasSuffix(cfg.File, ".vcf") {
			v := irelate.Vopen(cfg.File)
			streams = append(streams, irelate.StreamVCF(v))
		} else {
			streams = append(streams, irelate.Streamer(cfg.File))
		}
	}
	out, err := vcfgo.NewWriter(outw, query.Header)
	if err != nil {
		panic(err)
	}

	for interval := range irelate.IRelate(irelate.CheckOverlapPrefix, 0, irelate.LessPrefix, streams...) {
		variant := interval.(*irelate.Variant)
		if len(variant.Related()) > 0 {
			sep := Partition(variant, len(streams)-1)
			updateInfo(variant.Variant, sep, files)

		}
		fmt.Fprintln(out, variant)
	}
}

func updateInfo(v *vcfgo.Variant, sep [][]irelate.Relatable, files []anno) {
	for i, cfg := range files {

		valsByFld := Collect(v, sep[i], cfg)

		for i, vals := range valsByFld {
			// currently we don't do anything without overlaps.
			if len(vals) == 0 {
				continue
			}
			v.Info.Add(cfg.Names[i], Reducers[cfg.Ops[i]](vals))
		}
	}
}

func main() {

	flag.Parse()
	inFiles := flag.Args()
	if len(inFiles) != 2 {
		fmt.Printf(`Usage:
%s config.toml intput.vcf > annotated.vcf
`, os.Args[0])
		return
	}

	var config Annotations
	if _, err := toml.DecodeFile(inFiles[0], &config); err != nil {
		panic(err)
	}
	Anno(inFiles[1], config, os.Stdout)
}
