package api

import (
	"fmt"
	"testing"

	"github.com/biogo/hts/sam"
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
	bam *parsers.Bam

	src    Source
	src0   Source
	srcBam Source

	annotator *Annotator
}

var _ = Suite(&APISuite{})
var h = vcfgo.NewHeader()

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
	Id_:        "id",
	Reference:  "A",
	Alternate:  []string{"T", "G"},
	Quality:    float32(555.5),
	Filter:     "PASS",
	Info_:      vcfgo.NewInfoByte("DP=35", h),
}
var sv1 = &vcfgo.Variant{
	Chromosome: "chr1",
	Pos:        uint64(230),
	Id_:        "id",
	Reference:  "A",
	Alternate:  []string{"<DEL>"},
	Quality:    float32(555.5),
	Filter:     "PASS",
	Info_:      vcfgo.NewInfoByte("DP=35;SVLEN=15;CIPOS=-5,5;CIEND=-8,8", h),
}

func (s *APISuite) SetUpTest(c *C) {

	h.Infos["DP"] = &vcfgo.Info{Id: "DP", Description: "depth", Number: "1", Type: "Integer"}
	h.Infos["SVLEN"] = &vcfgo.Info{Id: "SVLEN", Description: "SVLEN", Number: "1", Type: "Integer"}
	h.Infos["CIPOS"] = &vcfgo.Info{Id: "CIPOS", Description: "CIPOS", Number: "2", Type: "Integer"}
	h.Infos["CIEND"] = &vcfgo.Info{Id: "CIEND", Description: "CIEND", Number: "2", Type: "Integer"}
	h.Infos["AF"] = &vcfgo.Info{Id: "AF", Description: "AF", Number: "1", Type: "Float"}
	h.Infos["AC_AFR"] = &vcfgo.Info{Id: "AF_AFR", Description: "AF_AFR", Number: "1", Type: "Float"}

	vv := *v1
	vv.Info_ = vcfgo.NewInfoByte("DP=35", h)
	s.v1 = &parsers.Variant{IVariant: &vv}
	s.v1.SetSource(0)
	v2 := *v1
	v2.Info_ = vcfgo.NewInfoByte("DP=44", h)
	s.v2 = &parsers.Variant{IVariant: &v2}
	s.v2.Info().Set("AC_AFR", 33)
	s.v2.SetSource(1)

	sv1.Info_ = vcfgo.NewInfoByte("DP=35;SVLEN=15;CIPOS=-5,5;CIEND=-8,8", h)
	s.sv1 = &parsers.Variant{IVariant: sv1}
	s.sv1.SetSource(0)

	v3 := *v1
	v3.Info_ = vcfgo.NewInfoByte("DP=88", h)
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

	sbed, err := parsers.IntervalFromBedLine("chr1\t224\t244\t111\t222")
	c.Assert(err, IsNil)
	s.bed = sbed.(*parsers.Interval)
	s.bed.SetSource(2)
	s.v1.AddRelated(s.bed)
	s.sv1.AddRelated(s.bed)

	s.bam = &parsers.Bam{Record: bam_rec, Chromosome: "chr1"}
	s.bam.SetSource(3)
	s.v1.AddRelated(s.bam)
	s.sv1.AddRelated(s.bam)

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

	s.annotator = NewAnnotator([]*Source{&s.src0, &s.src, &s.srcBam}, "function mean(vals) {sum=0; for(i=0;i<vals.length;i++){sum+=vals[i]}; return sum/vals.length}", false, true, true, "")

}

func (s *APISuite) TestPartition(c *C) {

	parted := s.annotator.partition(s.v1)
	c.Assert(len(parted), Equals, 3)

	c.Assert(parted[0], DeepEquals, []interfaces.Relatable{s.v2, s.v3})
	c.Assert(parted[1], DeepEquals, []interfaces.Relatable{s.bed})
	c.Assert(parted[2], DeepEquals, []interfaces.Relatable{s.bam})
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
	c.Assert(r, DeepEquals, []interface{}{33})
}

func (s *APISuite) TestCollectBam(c *C) {
	parted := s.annotator.partition(s.v1)
	r := collect(s.v1, parted[2], &s.srcBam, false)
	c.Assert(r, DeepEquals, []interface{}{30})
}

func (s *APISuite) TestAnnotateOne(c *C) {
	s.annotator.AnnotateOne(s.v1, s.annotator.Strict)
	c.Assert(s.v1.IVariant.(*vcfgo.Variant).Info_.String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111;bam_qual=30")
}

func (s *APISuite) TestAnnotateEndsLeft(c *C) {
	s.annotator.AnnotateEnds(s.sv1, LEFT)
	c.Assert(s.sv1.IVariant.(*vcfgo.Variant).Info_.String(), Equals, "DP=35;SVLEN=15;CIPOS=-5,5;CIEND=-8,8;left_AC_AFR=33;left_fitcons_mean=111;left_bam_qual=30")
}

func (s *APISuite) TestAnnotateEndsRight(c *C) {
	s.annotator.AnnotateEnds(s.v1, RIGHT)
	c.Assert(s.v1.IVariant.(*vcfgo.Variant).Info_.String(), Equals, "DP=35")

	s.annotator.AnnotateEnds(s.sv1, RIGHT)
	c.Assert(s.sv1.IVariant.(*vcfgo.Variant).Info_.String(), Equals, "DP=35;SVLEN=15;CIPOS=-5,5;CIEND=-8,8;right_fitcons_mean=111;right_bam_qual=30")
}

func (s *APISuite) TestAnnotateEndsBoth(c *C) {
	//pos:= 230 vcfgo.NewInfoByte("DP=35;SVLEN=15;CIPOS=-5,5;CIEND=-8,8", h),
	s.annotator.Strict = false
	s.annotator.AnnotateEnds(s.sv1, BOTH)
	c.Assert(s.sv1.Info().String(), Equals, "DP=35;SVLEN=15;CIPOS=-5,5;CIEND=-8,8;AC_AFR=33;fitcons_mean=111;bam_qual=30;left_AC_AFR=33;left_fitcons_mean=111;left_bam_qual=30;right_fitcons_mean=111;right_bam_qual=30")
}

func (s *APISuite) TestAnnotateEndsInterval(c *C) {
	s.annotator.AnnotateEnds(s.v1, INTERVAL)
	c.Assert(s.v1.Info().String(), Equals, "DP=35;AC_AFR=33;fitcons_mean=111;bam_qual=30")
}

func (s *APISuite) TestVFromB(c *C) {

	v := vFromB(s.bed)

	c.Assert(v.End(), Equals, s.bed.End())
	c.Assert(v.Start(), Equals, s.bed.Start())
	c.Assert(v.Chrom(), Equals, s.bed.Chrom())
	c.Assert(len(v.Related()), Equals, 0)
}

// utility functions.

func makeBed(chrom string, start int, end int, val float32) *parsers.Interval {
	i, _ := parsers.IntervalFromBedLine(fmt.Sprintf("%s\t%d\t%d\t%.3f", chrom, start, end, val))
	return i.(*parsers.Interval)
}

func makeVariant(chrom string, pos int, ref string, alt []string, name string, info string) *parsers.Variant {

	binfo := vcfgo.NewInfoByte(info, h)
	v := vcfgo.Variant{Chromosome: chrom, Pos: uint64(pos), Reference: ref, Alternate: alt,
		Id_: name, Info_: binfo}
	return parsers.NewVariant(&v, 0, make([]interfaces.Relatable, 0))
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

	a := NewAnnotator([]*Source{&bsrc}, "", true, true, false, "")
	v := makeVariant("chr1", 57, "AAAAAAAA", []string{"T"}, "rs", "CIPOS=-10,10;CIEND=-10,10")

	v.SetSource(0)
	v.AddRelated(b1)
	v.AddRelated(b2)
	a.AnnotateEnds(v, BOTH)
	c.Assert(v.Info().String(), Equals, "CIPOS=-10,10;CIEND=-10,10;some_mean=54.275;left_some_mean=54.275;right_some_mean=54.275")

	v = makeVariant("chr1", 57, "AAAAAAAA", []string{"T"}, "rs", "CIPOS=0,0;CIEND=-5,5")
	v.SetSource(0)
	v.AddRelated(b1)
	v.AddRelated(b2)
	c.Assert(v.Info().String(), Equals, "CIPOS=0,0;CIEND=-5,5")
	a.AnnotateEnds(v, BOTH)
	c.Assert(v.Info().String(), Equals, "CIPOS=0,0;CIEND=-5,5;some_mean=54.275;left_some_mean=9.11;right_some_mean=54.275")

	v = makeVariant("chr1", 57, "AAAAAAAA", []string{"T"}, "rs", "CIPOS=0,0;CIEND=-5,5")
	v.SetSource(0)
	v.AddRelated(b1)
	v.AddRelated(b2)
	a.AnnotateEnds(v, BOTH)
	c.Assert(v.Info().String(), Equals, "CIPOS=0,0;CIEND=-5,5;some_mean=54.275;left_some_mean=9.11;right_some_mean=54.275")
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

	a := NewAnnotator([]*Source{&bsrc}, "", true, false, false, "")
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
	v := makeVariant("chr1", 57, "AAAAAAAA", []string{"T"}, "rs", "")
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

	a := NewAnnotator([]*Source{&vsrc}, "", true, true, false, "")

	a.AnnotateOne(v, a.Strict)
	c.Assert(v.Info().String(), Equals, "o_id=rs")

	b := makeBed("chr1", 50, 66, 99.44)
	b.AddRelated(v)
	a.AnnotateOne(b, false)
	c.Assert(b.Fields[4], Equals, "o_id=rs")

	b2 := makeBed("chr1", 50, 66, 99.44)
	b2.AddRelated(v)
	a.AnnotateEnds(b2, BOTH)
	// variant only overlaps in middle.
	c.Assert(b2.Fields[4], Equals, "o_id=rs", Commentf("%s", b2.Fields))
}

// TODO: test with bam.
