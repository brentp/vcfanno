package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/brentp/vcfanno/caddcode"
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func itime(idx string) int {

	i := caddcode.Reader(idx)
	t := time.Now()
	n := 0
	for j := 0; j < 16; j++ {
		log.Println(j)
		ic := rand.Intn(len(i.Chroms))
		chrom := i.Chroms[ic]
		max_len := rand.Intn(i.Lengths[ic])
		for k := 10000; k < max_len; k += 50 {

			v, err := i.At(chrom, k, "C")
			check(err)
			fmt.Printf("%s\t%d\tC\t%.2f\n", chrom, k, v)
			n += 1
		}
	}
	dur := time.Now().Sub(t)

	log.Printf("tested %d sites (%.0f/second)\n", n, float64(n)/dur.Seconds())
	return 0
}

func main() {

	if os.Args[1] == "test" {
		os.Exit(itime(os.Args[2]))
	}

	idx := caddcode.Reader(os.Args[1])
	chr := os.Args[2]
	pos, err := strconv.Atoi(os.Args[3])
	check(err)
	base := os.Args[4]
	fmt.Println(idx.At(chr, pos, base))
}
