package main

import (
	"github.com/brentp/irelate"
	"github.com/brentp/vcfgo"
	. "gopkg.in/check.v1"
)

type APISuite struct {
	v1 *irelate.Variant
	v2 *irelate.Variant
	v3 *irelate.Variant

	bed *irelate.Interval

	src  Source
	src0 Source

	annotator *Annotator
}

var _ = Suite(&APISuite{})

var v1 = &vcfgo.Variant{
	Chromosome: "chr1",
	Pos:        uint64(234),
	Id:         "id",
	Ref:        "A",
	Alt:        []string{"T", "G"},
	Quality:    float32(555.5),
	Filter:     "PASS",
	Info: map[string]interface{}{
		"DP":      uint32(35),
		"__order": []string{"DP"},
	},
}

func (s *APISuite) SetUpTest(c *C) {
	s.v1 = &irelate.Variant{Variant: v1}
	s.v1.Info = map[string]interface{}{
		"DP":      uint32(35),
		"__order": []string{"DP"},
	}
	s.v1.SetSource(0)
	v2 := *v1
	v2.Info = map[string]interface{}{"DP": uint32(44), "__order": []string{"DP"}}
	s.v2 = &irelate.Variant{Variant: &v2}
	s.v2.Info.Add("AC_AFR", 33)
	s.v2.SetSource(1)

	v3 := *v1
	v3.Info = map[string]interface{}{"DP": uint32(88)}
	s.v3 = &irelate.Variant{Variant: &v3}
	s.v3.SetSource(1)

	c.Assert(s.v1.Info["DP"], Equals, uint32(35))
	c.Assert(s.v2.Info["DP"], Equals, uint32(44))
	c.Assert(s.v3.Info["DP"], Equals, uint32(88))

	s.v1.AddRelated(s.v2)
	s.v1.AddRelated(s.v3)

	c.Assert(2, Equals, len(s.v1.Related()))

	s.bed = irelate.IntervalFromBedLine("chr1\t224\t244\t111\t222").(*irelate.Interval)
	s.bed.SetSource(2)
	s.v1.AddRelated(s.bed)

	s.src = Source{
		File:   "example/fitcons.bed",
		Op:     "mean",
		Name:   "fitcons_mean",
		Column: 4,
		Field:  "",
		Index:  1,
		IsJs:   false,
	}

	s.src0 = Source{
		File:   "example/exac.vcf",
		Op:     "first",
		Column: -1,
		Field:  "AC_AFR",
		Name:   "AC_AFR",
		Index:  0,
		IsJs:   false,
	}

	s.annotator = NewAnnotator([]Source{s.src0, s.src}, "function mean(vals) {sum=0; for(i=0;i<vals.length;i++){sum+=vals[i]}; return sum/vals.length}", false, true)

}

func (s *APISuite) TestPartition(c *C) {

	parted := s.annotator.partition(s.v1)

	c.Assert(parted[0], DeepEquals, []irelate.Relatable{s.v2, s.v3})
	c.Assert(parted[1], DeepEquals, []irelate.Relatable{s.bed})
	c.Assert(len(parted), Equals, 2)
}

func (s *APISuite) TestSource(c *C) {

	c.Assert(s.src.IsNumber(), Equals, true)

	s.src.Op = "concat"
	c.Assert(s.src.IsNumber(), Equals, false)
	s.src.Op = "mean"

}

func (s *APISuite) TestJsSetup(c *C) {

	vals := []interface{}{0, 1, 2}
	s.annotator.vm.Set("vals", vals)
	value, err := s.annotator.vm.Run("mean(vals)")
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
	for _, jst := range jstest {
		v := s.annotator.JsOp(s.v1.Variant, jst.js, []interface{}{})
		c.Assert(v, Equals, jst.result)
	}
}

func (s *APISuite) TestCollect(c *C) {
	parted := s.annotator.partition(s.v1)
	r := collect(s.v1, parted[0], &s.src0, false)
	c.Assert(r, DeepEquals, []interface{}{33})
}

func (s *APISuite) TestAnnotateOne(c *C) {
	s.annotator.AnnotateOne(s.v1)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111")
}

func (s *APISuite) TestAnnotateEndsLeft(c *C) {
	s.annotator.AnnotateEnds(s.v1, LEFT)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;left_AC_AFR=33;left_fitcons_mean=111")
}

func (s *APISuite) TestAnnotateEndsRight(c *C) {
	s.annotator.AnnotateEnds(s.v1, RIGHT)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;right_AC_AFR=33;right_fitcons_mean=111")
}

func (s *APISuite) TestAnnotateEndsBoth(c *C) {
	s.annotator.AnnotateEnds(s.v1, BOTH)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111;left_AC_AFR=33;left_fitcons_mean=111;right_AC_AFR=33;right_fitcons_mean=111")
}

func (s *APISuite) TestAnnotateEndsInterval(c *C) {
	s.annotator.AnnotateEnds(s.v1, INTERVAL)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111")
}

func (s *APISuite) TestVFromB(c *C) {

	v := vFromB(s.bed)

	c.Assert(v.End(), Equals, s.bed.End())
	c.Assert(v.Start(), Equals, s.bed.Start())
	c.Assert(v.Chrom(), Equals, s.bed.Chrom())
	c.Assert(len(v.Related()), Equals, 0)
}

// TODO: do test with ends and long interval to make sure everythign looks ok.
// TOOD: make functions
/*

makeBed(chrom string, start int, end int, val float32) *irelate.Interval
makeVariant(chrom string, ref string, alt []string, name string, info map[string]interface{})

*/
