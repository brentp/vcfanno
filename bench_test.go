package main

import (
	"bufio"
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"testing"
)

func BenchmarkAnno(b *testing.B) {
	var configs Annotations
	if _, err := toml.DecodeFile("example/conf.toml", &configs); err != nil {
		panic(err)
	}

	out := bufio.NewWriter(ioutil.Discard)
	for n := 0; n < b.N; n++ {
		Anno("example/query.vcf", configs, out)
		out.Flush()
	}
}
