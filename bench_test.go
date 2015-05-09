package main

import (
	"bufio"
	"io/ioutil"
	"testing"

	"github.com/BurntSushi/toml"
)

func BenchmarkAnno(b *testing.B) {
	var configs Config
	if _, err := toml.DecodeFile("example/conf.toml", &configs); err != nil {
		panic(err)
	}

	out := bufio.NewWriter(ioutil.Discard)
	a := NewAnnotator(configs.Sources(), configs.Js, false, true)
	for n := 0; n < b.N; n++ {
		a.Annotate("example/query.vcf", ioutil.Discard)
		out.Flush()
	}
}
