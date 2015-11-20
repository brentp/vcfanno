package api

import (
	"fmt"
	"testing"

	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/irelate/parsers"
	"github.com/brentp/vcfgo"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type APISuite struct {
	sv1 *parsers.Variant
	v1  *parsers.Variant
	v2  *parsers.Variant
	v3  *parsers.Variant

	bed *parsers.Interval

	src  Source
	src0 Source

	annotator *Annotator
}

var _ = Suite(&APISuite{})
var h = vcfgo.NewHeader()

var v1 = &vcfgo.Variant{
	Chromosome: "chr1",
	Pos:        uint64(234),
	Id_:        "id",
	Reference:  "A",
	Alternate:  []string{"T", "G"},
	Quality:    float32(555.5),
	Filter:     "PASS",
	Info_:      vcfgo.NewInfoByte([]byte("DP=35"), h),
}
var sv1 = &vcfgo.Variant{
	Chromosome: "chr1",
	Pos:        uint64(230),
	Id_:        "id",
	Reference:  "A",
	Alternate:  []string{"<DEL>"},
	Quality:    float32(555.5),
	Filter:     "PASS",
	Info_:      vcfgo.NewInfoByte([]byte("DP=35;SVLEN=15;CIPOS=-5,5;CIEND=-8,8"), h),
}

func (s *APISuite) SetUpTest(c *C) {

	h.Infos["DP"] = &vcfgo.Info{Id: "DP", Description: "depth", Number: "1", Type: "Integer"}
	h.Infos["SVLEN"] = &vcfgo.Info{Id: "SVLEN", Description: "SVLEN", Number: "1", Type: "Integer"}
	h.Infos["CIPOS"] = &vcfgo.Info{Id: "CIPOS", Description: "CIPOS", Number: "2", Type: "Integer"}
	h.Infos["CIEND"] = &vcfgo.Info{Id: "CIEND", Description: "CIEND", Number: "2", Type: "Integer"}
	h.Infos["AF"] = &vcfgo.Info{Id: "AF", Description: "AF", Number: "1", Type: "Float"}
	h.Infos["AC_AFR"] = &vcfgo.Info{Id: "AF_AFR", Description: "AF_AFR", Number: "1", Type: "Float"}

	vv := *v1
	vv.Info_ = vcfgo.NewInfoByte([]byte("DP=35"), h)
	s.v1 = &parsers.Variant{IVariant: &vv}
	s.v1.SetSource(0)
	v2 := *v1
	v2.Info_ = vcfgo.NewInfoByte([]byte("DP=44"), h)
	s.v2 = &parsers.Variant{IVariant: &v2}
	s.v2.Info().Set("AC_AFR", 33)
	s.v2.SetSource(1)

	sv1.Info_ = vcfgo.NewInfoByte([]byte("DP=35;SVLEN=15;CIPOS=-5,5;CIEND=-8,8"), h)
	s.sv1 = &parsers.Variant{IVariant: sv1}
	s.sv1.SetSource(0)

	v3 := *v1
	v3.Info_ = vcfgo.NewInfoByte([]byte("DP=88"), h)
	s.v3 = &parsers.Variant{IVariant: &v3}
	s.v3.SetSource(1)

	v, e := v1.Info_.Get("DP")
	c.Assert(e, IsNil)
	c.Assert(v, Equals, 35)

	v, e = v2.Info_.Get("DP")
	c.Assert(e, IsNil)
	c.Assert(v, Equals, 44)

	v, e = v3.Info_.Get("DP")
	c.Assert(e, IsNil)
	c.Assert(v, Equals, 88)

	s.v1.AddRelated(s.v2)
	s.v1.AddRelated(s.v3)
	s.sv1.AddRelated(s.v2)
	s.sv1.AddRelated(s.v3)

	c.Assert(2, Equals, len(s.v1.Related()))

	sbed, err := parsers.IntervalFromBedLine([]byte("chr1\t224\t244\t111\t222"))
	c.Assert(err, IsNil)
	s.bed = sbed.(*parsers.Interval)
	s.bed.SetSource(2)
	s.v1.AddRelated(s.bed)
	s.sv1.AddRelated(s.bed)

	s.src = Source{
		File:   "example/fitcons.bed",
		Op:     "mean",
		Name:   "fitcons_mean",
		Column: 4,
		Field:  "",
		Index:  1,
	}

	s.src0 = Source{
		File:   "example/exac.vcf",
		Op:     "first",
		Column: -1,
		Field:  "AC_AFR",
		Name:   "AC_AFR",
		Index:  0,
	}

	s.annotator = NewAnnotator([]*Source{&s.src0, &s.src}, "function mean(vals) {sum=0; for(i=0;i<vals.length;i++){sum+=vals[i]}; return sum/vals.length}", false, true)

}

func (s *APISuite) TestPartition(c *C) {

	parted := s.annotator.partition(s.v1)
	c.Assert(len(parted), Equals, 2)

	c.Assert(parted[0], DeepEquals, []interfaces.Relatable{s.v2, s.v3})
	c.Assert(parted[1], DeepEquals, []interfaces.Relatable{s.bed})
}

func (s *APISuite) TestSource(c *C) {

	c.Assert(s.src.IsNumber(), Equals, true)

	s.src.Op = "concat"
	c.Assert(s.src.IsNumber(), Equals, false)
	s.src.Op = "mean"

}

func (s *APISuite) TestJsSetup(c *C) {
	vm := s.annotator.Sources[0].Vm

	vals := []interface{}{0, 1, 2}
	vm.Set("vals", vals)
	value, err := vm.Run("mean(vals)")
	c.Assert(err, IsNil)
	val, err := value.ToString()
	c.Assert(err, IsNil)
	c.Assert(val, Equals, "1")
}

var jstest = []struct {
	js     string
	result string
}{
	{"chrom", "chr1"},
	{"start", "233"},
	{"end", "234"},
}

func (s *APISuite) TestJsOp(c *C) {
	vm := s.annotator.Sources[0].Vm
	for _, jst := range jstest {
		script, err := vm.Compile("", jst.js)
		c.Assert(err, IsNil)
		v := s.annotator.Sources[0].JsOp(s.v1, script, []interface{}{})
		c.Assert(v, Equals, jst.result)
	}
}

func (s *APISuite) TestCollect(c *C) {
	parted := s.annotator.partition(s.v1)
	r := collect(s.v1, parted[0], &s.src0, false)
	c.Assert(len(r), Equals, 1)
	c.Assert(r[0], Equals, float64(33))
}

func (s *APISuite) TestAnnotateOne(c *C) {
	s.annotator.AnnotateOne(s.v1, s.annotator.Strict)
	c.Assert(s.v1.IVariant.(*vcfgo.Variant).Info_.String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111")
}

// utility functions.

func makeBed(chrom string, start int, end int, val float32) *parsers.Interval {
	i, _ := parsers.IntervalFromBedLine([]byte(fmt.Sprintf("%s\t%d\t%d\t%.3f", chrom, start, end, val)))
	return i.(*parsers.Interval)
}

func makeVariant(chrom string, pos int, ref string, alt []string, name string, info string, h *vcfgo.Header) *parsers.Variant {

	binfo := vcfgo.NewInfoByte([]byte(info), h)
	v := vcfgo.Variant{Chromosome: chrom, Pos: uint64(pos), Reference: ref, Alternate: alt,
		Id_: name, Info_: binfo}
	return parsers.NewVariant(&v, 0, make([]interfaces.Relatable, 0))
}

func (s *APISuite) TestEndsBedQuery(c *C) {

	b1 := makeBed("chr1", 50, 66, 99.44)
	b1.SetSource(0)
	b2 := makeBed("chr1", 45, 59, 9.11)
	b2.SetSource(1)

	bsrc := Source{
		File:   "some.bed",
		Op:     "mean",
		Column: 4,
		Name:   "some_mean",
		Field:  "",
		Index:  0,
	}
	b1.AddRelated(b2)
	b2.AddRelated(b1)

	NewAnnotator([]*Source{&bsrc}, "", true, false)
}

func (s *APISuite) TestIdAnno(c *C) {
	v := makeVariant("chr1", 57, "AAAAAAAA", []string{"T"}, "rs", "", h)
	vsrc := Source{
		File:   "some.vcf",
		Op:     "first",
		Column: -1,
		Name:   "o_id",
		Field:  "ID",
		Index:  0,
	}
	v.AddRelated(v)
	v.SetSource(1)

	a := NewAnnotator([]*Source{&vsrc}, "", true, true)

	a.AnnotateOne(v, a.Strict)
	c.Assert(v.Info().String(), Equals, "o_id=rs")

}
