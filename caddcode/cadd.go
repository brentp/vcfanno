package caddcode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/brentp/xopen"
	"github.com/edsrzf/mmap-go"
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

type Index struct {
	Chroms     []string
	Lengths    []int
	mmap       mmap.MMap
	offsets    map[string]int
	MapLengths map[string]int
	val        []byte
}

func (i Index) Offset(chrom string) (int, error) {
	if o, ok := i.offsets[chrom]; ok {
		return o, nil
	}
	offset := 0
	for j, chr := range i.Chroms {
		if chr == chrom {
			i.offsets[chr] = offset
			i.MapLengths[chr] = i.Lengths[j]
			return offset, nil
		}
		offset += i.Lengths[j]
	}
	return -1, fmt.Errorf("chromosome not found in index: %s\n", chrom)
}

var ErrorOutofRange = errors.New("requested position out of range")

func (i Index) Get(chrom string, pos int) (uint32, error) {
	off, err := i.Offset(chrom)
	check(err)
	off *= 4
	off += (pos * 4)
	if pos > i.MapLengths[chrom] {
		log.Println(chrom, pos, i.MapLengths[chrom])
		return 0, ErrorOutofRange
	}
	copy(i.val[0:4], i.mmap[off:off+4])

	v := binary.LittleEndian.Uint32(i.val)
	return v, nil
}

func (i Index) At(chrom string, pos int, alt string) (float64, error) {
	if (pos == 60830534 || pos == 60830763 || pos == 60830764) && chrom[0] == '3' {
		// these have ambiguous bases in the cadd v1.2 file so we just hard code the actual values
		// for all 4 bases.
		if pos == 60830534 {
			return 11.9, nil // all values are < 0.05 of this, so just use it.
		}
		if pos == 60830763 {
			return map[string]float64{"A": 0.45, "C": 0.445, "G": 0.478, "T": 0.429}[alt], nil
		}
		// 60830764
		return map[string]float64{"A": 2.71, "C": 2.69, "G": 2.81, "T": 2.624}[alt], nil
	}
	num, err := i.Get(chrom, pos)
	if err != nil {
		return float64(num), err
	}
	missing := int(num % 4)
	off := uint32(0x3FF) // 2^10 - 1

	letters := []byte("ACGT")
	if alt[0] == letters[missing] {
		return float64(0), nil
	}

	leftShift := uint32(2)
	max := uint32(0)
	for i, a := range letters {
		val := (num >> leftShift)
		if a == alt[0] {
			return float64(val&off) / 10.23, nil
		}
		if i != missing {
			leftShift += 10
		}
		if val > max {
			max = val
		}
	}
	// Return the maximum for use with SVs.
	return float64(max&off) / 10.23, fmt.Errorf("position not found %s:%d\n", chrom, pos)
}

func Reader(f string) Index {
	binPath := f[:len(f)-4] + ".bin"

	if !(xopen.Exists(f) && xopen.Exists(binPath)) {
		log.Fatalf("Error finding CADD files. both .bin and .idx files are required\n")
	}

	rdr, err := xopen.Ropen(f)
	check(err)

	mrdr, err := os.Open(binPath)
	check(err)
	mmap, err := mmap.Map(mrdr, mmap.RDONLY, 0)
	check(err)

	i := Index{make([]string, 0), make([]int, 0), mmap, make(map[string]int), make(map[string]int),
		make([]byte, 4)}
	for {
		line, err := rdr.ReadString('\n')
		if err == io.EOF {
			break
		}
		check(err)

		toks := strings.Split(line, "\t")
		length, err := strconv.Atoi(strings.TrimRight(toks[1], "\n\r"))
		check(err)
		i.Chroms = append(i.Chroms, toks[0])
		i.Lengths = append(i.Lengths, length)
	}
	return i
}
