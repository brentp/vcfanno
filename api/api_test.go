package api

import (
	"fmt"
	"testing"

	"github.com/biogo/hts/sam"
	"github.com/brentp/irelate"
	"github.com/brentp/vcfgo"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type APISuite struct {
	v1 *irelate.Variant
	v2 *irelate.Variant
	v3 *irelate.Variant

	bed *irelate.Interval
	bam *irelate.Bam

	src    Source
	src0   Source
	srcBam Source

	annotator *Annotator
}

var _ = Suite(&APISuite{})

var bam_rec = &sam.Record{Name: "read1",
	Flags:   sam.Paired | sam.ProperPair,
	Pos:     232,
	MatePos: -1,
	MapQ:    30,
	Cigar:   sam.Cigar{sam.NewCigarOp(sam.CigarMatch, 14)},
	Seq:     sam.NewSeq([]byte("AAAAGATAAGGATA")),
	Qual:    []uint8{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
}

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

	s.bam = &irelate.Bam{Record: bam_rec, Chromosome: "chr1"}
	s.bam.SetSource(3)
	s.v1.AddRelated(s.bam)

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

	s.srcBam = Source{
		File:   "example/some.bam",
		Op:     "mean",
		Column: -1,
		Field:  "mapq",
		Name:   "bam_qual",
		Index:  2,
	}

	s.annotator = NewAnnotator([]*Source{&s.src0, &s.src, &s.srcBam}, "function mean(vals) {sum=0; for(i=0;i<vals.length;i++){sum+=vals[i]}; return sum/vals.length}", false, true)

}

func (s *APISuite) TestPartition(c *C) {

	parted := s.annotator.partition(s.v1)
	c.Assert(len(parted), Equals, 3)

	c.Assert(parted[0], DeepEquals, []irelate.Relatable{s.v2, s.v3})
	c.Assert(parted[1], DeepEquals, []irelate.Relatable{s.bed})
	c.Assert(parted[2], DeepEquals, []irelate.Relatable{s.bam})
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
		script, err := s.annotator.vm.Compile("", jst.js)
		c.Assert(err, IsNil)
		v := s.annotator.JsOp(s.v1.Variant, script, []interface{}{})
		c.Assert(v, Equals, jst.result)
	}
}

func (s *APISuite) TestCollect(c *C) {
	parted := s.annotator.partition(s.v1)
	r := collect(s.v1, parted[0], &s.src0, false)
	c.Assert(r, DeepEquals, []interface{}{33})
}

func (s *APISuite) TestCollectBam(c *C) {
	parted := s.annotator.partition(s.v1)
	r := collect(s.v1, parted[2], &s.srcBam, false)
	c.Assert(r, DeepEquals, []interface{}{30})
}

func (s *APISuite) TestAnnotateOne(c *C) {
	s.annotator.AnnotateOne(s.v1, s.annotator.Strict)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111;bam_qual=30")
}

func (s *APISuite) TestAnnotateEndsLeft(c *C) {
	s.annotator.AnnotateEnds(s.v1, LEFT)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;left_AC_AFR=33;left_fitcons_mean=111;left_bam_qual=30")
}

func (s *APISuite) TestAnnotateEndsRight(c *C) {
	s.annotator.AnnotateEnds(s.v1, RIGHT)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;right_AC_AFR=33;right_fitcons_mean=111;right_bam_qual=30")
}

func (s *APISuite) TestAnnotateEndsBoth(c *C) {
	s.annotator.AnnotateEnds(s.v1, BOTH)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111;bam_qual=30;left_AC_AFR=33;left_fitcons_mean=111;left_bam_qual=30;right_AC_AFR=33;right_fitcons_mean=111;right_bam_qual=30")
}

func (s *APISuite) TestAnnotateEndsInterval(c *C) {
	s.annotator.AnnotateEnds(s.v1, INTERVAL)
	c.Assert(s.v1.Info.String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111;bam_qual=30")
}

func (s *APISuite) TestVFromB(c *C) {

	v := vFromB(s.bed)

	c.Assert(v.End(), Equals, s.bed.End())
	c.Assert(v.Start(), Equals, s.bed.Start())
	c.Assert(v.Chrom(), Equals, s.bed.Chrom())
	c.Assert(len(v.Related()), Equals, 0)
}

// utility functions.

func makeBed(chrom string, start int, end int, val float32) *irelate.Interval {
	i := irelate.IntervalFromBedLine(fmt.Sprintf("%s\t%d\t%d\t%.3f", chrom, start, end, val)).(*irelate.Interval)
	return i
}

func makeVariant(chrom string, pos int, ref string, alt []string, name string, info map[string]interface{}) *irelate.Variant {

	if _, ok := info["__order"]; !ok {
		info["__order"] = make([]string, 0)
	}
	v := vcfgo.Variant{Chromosome: chrom, Pos: uint64(pos), Ref: ref, Alt: alt,
		Id: name, Info: info}
	return irelate.NewVariant(&v, 0, make([]irelate.Relatable, 0))
}

func (s *APISuite) TestEndsDiff(c *C) {

	b1 := makeBed("chr1", 60, 66, 99.44)
	b1.SetSource(1)
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

	a := NewAnnotator([]*Source{&bsrc}, "", true, true)

	v := makeVariant("chr1", 57, "AAAAAAAA", []string{"T"}, "rs", make(map[string]interface{}))
	v.SetSource(0)

	v.AddRelated(b1)
	v.AddRelated(b2)

	a.AnnotateEnds(v, BOTH)

	// the 2 b intervals only overlap in the middle, so we see their respective values for the left
	// and right and their mean for the middle.
	c.Assert(v.Info.String(), Equals, "some_mean=54.275;left_some_mean=9.11;right_some_mean=99.44")
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

	a := NewAnnotator([]*Source{&bsrc}, "", true, false)
	a.AnnotateEnds(b1, BOTH)
	c.Assert(b1.Fields[4], Equals, "some_mean=9.11;left_some_mean=9.11")

	b1.SetSource(1)
	b2.SetSource(0)
	a.AnnotateEnds(b2, BOTH)
	c.Assert(b2.Fields[4], Equals, "some_mean=99.44;right_some_mean=99.44")

	b3 := makeBed("chr1", 50, 66, 99.44)
	b3.SetSource(0)
	b1.SetSource(1)
	b2.SetSource(1)
	b3.AddRelated(b1)

	a.AnnotateEnds(b3, INTERVAL)
	c.Assert(b3.Fields[4], Equals, "some_mean=99.44")
}

func (s *APISuite) TestIdAnno(c *C) {
	v := makeVariant("chr1", 57, "AAAAAAAA", []string{"T"}, "rs", make(map[string]interface{}))
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
	c.Assert(v.Info.String(), Equals, "o_id=rs")

	b := makeBed("chr1", 50, 66, 99.44)
	b.AddRelated(v)
	a.AnnotateOne(b, false)
	c.Assert(b.Fields[4], Equals, "o_id=rs")

	b2 := makeBed("chr1", 50, 66, 99.44)
	b2.AddRelated(v)
	a.AnnotateEnds(b2, BOTH)
	// variant only overlaps in middle.
	c.Assert(b2.Fields[4], Equals, "o_id=rs")
}

// TODO: test with bam.
