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

func fixBam(as []anno, j int) {
	a := as[j]
	if strings.HasSuffix(a.File, ".bam") {
		as[j].Columns = []int{0}
		as[j].Ops = []string{"count"}
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
		for i, name := range cfg.Names {
			ntype := "Character"
			if strings.HasSuffix(cfg.File, ".bam") || cfg.isNumber(i) {
				ntype = "Float"
			}
			var desc string
			// write the VCF header.
			if strings.HasSuffix(cfg.File, ".bam") {
				fixBam(files, j)
				desc = fmt.Sprintf("calculated by coverage from %s", cfg.File)
			} else if cfg.Fields != nil {
				desc = fmt.Sprintf("calculated by %s of overlapping values in field %s from %s", cfg.Ops[i], cfg.Fields[i], cfg.File)
			} else {
				desc = fmt.Sprintf("calculated by %s of overlapping values in column %d from %s", cfg.Ops[i], cfg.Columns[i], cfg.File)
			}
			query.Header.Infos[name] = &vcfgo.Info{Id: name, Number: "1", Type: ntype, Description: desc}
		}
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

	for interval := range irelate.IRelate(irelate.CheckRelatedByOverlap, false, 0, streams...) {
		variant := interval.(*irelate.Variant)
		if len(variant.Related()) > 0 {
			for i, cfg := range files {
				valsByFld := Collect(variant, cfg, uint32(i))
				for i, vals := range valsByFld {
					if len(vals) == 0 {
						continue
					}
					variant.Info.Add(cfg.Names[i], Reducers[cfg.Ops[i]](vals))
				}
			}

		}
		fmt.Fprintln(out, variant)
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
