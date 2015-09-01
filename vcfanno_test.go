package main

import (
	"testing"

	"github.com/brentp/irelate"
	"github.com/brentp/vcfanno/shared"
	"github.com/brentp/vcfgo"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type AnnoSuite struct {
	v1 *irelate.Variant
	v2 *irelate.Variant
	v3 *irelate.Variant
	b  *irelate.Interval
}

var h = vcfgo.NewHeader()

var v1 = &vcfgo.Variant{
	Chromosome: "chr1",
	Pos:        uint64(234),
	Id:         "id",
	Reference:  "A",
	Alternate:  []string{"T", "G"},
	Quality:    float32(555.5),
	Filter:     "PASS",
	Info_:      vcfgo.NewInfoByte("DP=35", h),
}

var _ = Suite(&AnnoSuite{})

func (s *AnnoSuite) SetUpTest(c *C) {

	h.Infos["DP"] = &vcfgo.Info{Id: "DP", Description: "depth", Number: "1", Type: "Integer"}

	s.v1 = &irelate.Variant{IVariant: v1}
	s.v1.SetSource(0)
	v2 := *v1
	v2.Info_ = vcfgo.NewInfoByte("DP=44", h)
	s.v2 = &irelate.Variant{IVariant: &v2}
	s.v2.SetSource(1)

	v3 := *v1
	v3.Info_ = vcfgo.NewInfoByte("DP=88", h)
	s.v3 = &irelate.Variant{IVariant: &v3}
	s.v3.SetSource(1)

	v, e := v1.Info_.Get("DP")
	c.Assert(v, Equals, 35)
	c.Assert(e, IsNil)

	v, e = v2.Info_.Get("DP")
	c.Assert(v, Equals, 44)
	c.Assert(e, IsNil)

	v, e = v3.Info().Get("DP")
	c.Assert(v, Equals, 88)
	c.Assert(e, IsNil)

	s.v1.AddRelated(s.v2)
	s.v1.AddRelated(s.v3)

	c.Assert(2, Equals, len(s.v1.Related()))

	sb, err := irelate.IntervalFromBedLine("chr1\t224\t244\t111\t222")
	s.b = sb.(*irelate.Interval)
	c.Assert(err, IsNil)
	s.b.SetSource(2)
	s.v1.AddRelated(s.b)

}

var cfg = shared.Annotation{
	File:   "example/query.vcf",
	Ops:    []string{"mean", "min", "max", "concat", "uniq", "first", "count"},
	Fields: []string{"DP", "DP", "DP", "DP", "DP", "DP", "DP", "DP"},
	Names:  []string{"dp_mean", "dp_min", "dp_max", "dp_concat", "dp_uniq", "dp_first", "dp_count"}}

func (s *AnnoSuite) TestFlatten(c *C) {

	cfgBed := shared.Annotation{
		File:    "example/fitcons.bed",
		Ops:     []string{"mean", "max", "flag"},
		Columns: []int{4, 5, 1},
		Names:   []string{"bed_mean", "bed_max", "bedFlag"},
	}

	f, err := cfgBed.Flatten(0, "")
	c.Assert(len(f), Equals, 3)
	c.Assert(err, IsNil)
}

/*
	sep := Partition(s.v1, 2)
	updateInfo(s.v1, sep, []annotation{cfg, cfgBed}, "", true)

	c.Assert(s.v1.Info["dp_mean"], Equals, float32(66.0))
	c.Assert(s.v1.Info["dp_min"], Equals, float32(44.0))
	c.Assert(s.v1.Info["dp_max"], Equals, float32(88.0))
	c.Assert(s.v1.Info["dp_concat"], Equals, "44,88")
	c.Assert(s.v1.Info["dp_uniq"], Equals, "44,88")
	c.Assert(s.v1.Info["dp_first"], Equals, uint32(44))
	c.Assert(s.v1.Info["dp_count"], Equals, 2)

	c.Assert(s.v1.Info["bed_mean"], Equals, float32(111))
	c.Assert(s.v1.Info["bed_max"], Equals, float32(222))

	c.Assert(s.v1.Info["bedFlag"], Equals, true)

	c.Assert(fmt.Sprintf("%s", s.v1.Info), Equals, "DP=35;dp_mean=66;dp_min=44;dp_max=88;dp_concat=44,88;dp_uniq=44,88;dp_first=44;dp_count=2;bed_mean=111;bed_max=222;bedFlag")
}

func (s *AnnoSuite) TestAnnoEnds(c *C) {

	cfg := annotation{
		File:   "fake file",
		Ops:    []string{"mean", "min", "max", "concat", "uniq", "first", "count"},
		Fields: []string{"DP", "DP", "DP", "DP", "DP", "DP", "DP", "DP"},
		Names:  []string{"dp_mean", "dp_min", "dp_max", "dp_concat", "dp_uniq", "dp_first", "dp_count"},
	}
	cfgBed := annotation{
		File:    "bed file",
		Ops:     []string{"mean", "max", "flag"},
		Columns: []int{4, 5, 1},
		Names:   []string{"bed_mean", "bed_max", "bedFlag"},
	}

	sep := Partition(s.v1, 2)
	sav := s.v1.Ref
	s.v1.Ref = "ATT"
	updateInfo(s.v1, sep, []annotation{cfg, cfgBed}, BOTH, true)
	s.v1.Ref = sav

	c.Assert(fmt.Sprintf("%s", s.v1.Info), Equals, "DP=35;dp_mean=66;dp_min=44;dp_max=88;dp_concat=44,88;dp_uniq=44,88;dp_first=44;dp_count=2;bed_mean=111;bed_max=222;bedFlag;left_dp_mean=66;left_dp_min=44;left_dp_max=88;left_dp_concat=44,88;left_dp_uniq=44,88;left_dp_first=44;left_dp_count=2;left_bed_mean=111;left_bed_max=222;left_bedFlag;right_bed_mean=111;right_bed_max=222;right_bedFlag")
}

var cfgBad = annotation{
	File:    "bed file",
	Ops:     []string{"mean", "max", "flag"},
	Columns: []int{4, 5},
	Names:   []string{"bed_mean", "bed_max", "bedFlag"},
}
var cfgBed = annotation{
	File:    "bed file",
	Ops:     []string{"mean", "max", "flag"},
	Columns: []int{4, 5, 5},
	Names:   []string{"bed_mean", "bed_max", "bedFlag"},
}

var interval = irelate.IntervalFromBedLine("chr1\t224\t244")

func (s *AnnoSuite) TestAnnoBed(c *C) {
	interval.SetSource(2)
	interval.AddRelated(s.v1)
	interval.AddRelated(s.v2)
	interval.AddRelated(s.v3)
	interval.AddRelated(s.b)

	sep := Partition(s.v1, 3)
	bed := interval.(*irelate.Interval)
	updateBed(bed, sep, []annotation{cfg, cfgBed}, BOTH)

	c.Assert(fmt.Sprintf("%s", bed.Fields[3]), Equals, "dp_mean=66;dp_min=44;dp_max=88;dp_concat=44,88;dp_uniq=44,88;dp_first=44;dp_count=2;bed_mean=111;bed_max=222;bedFlag;left_bed_mean=111;left_bed_max=222;left_bedFlag;right_bed_mean=111;right_bed_max=222;right_bedFlag")
}

func (s *AnnoSuite) TestCheck(c *C) {
	e := checkAnno(&cfgBad)
	c.Assert(e, ErrorMatches, "must specify same # of 'columns' as 'ops' for bed file")

	cfgBad.Fields = []string{"abc", "def"}
	e = checkAnno(&cfgBad)
	c.Assert(e, ErrorMatches, "specify only 'fields' or 'columns' not both bed file")

	cfgBad.Columns = nil
	e = checkAnno(&cfgBad)
	c.Assert(e, ErrorMatches, "must specify same # of 'fields' as 'ops' for bed file")
}

func (s *AnnoSuite) TestOverlap(c *C) {
	s1 := &irelate.Variant{Variant: v1}
	c.Assert(overlap(s1, s1), Equals, true)
	vv := *v1
	vv.Pos += 2
	s2 := &irelate.Variant{Variant: &vv}
	c.Assert(overlap(s1, s2), Equals, false)

}

func (s *AnnoSuite) TestUpdateHeader(c *C) {
	vcf := irelate.Vopen("example/query.vcf")
	updateHeader([]annotation{cfgBed}, 0, vcf, false)

	for _, k := range cfgBed.Names {
		_, ok := vcf.Header.Infos[k]
		c.Assert(ok, Equals, true, Commentf(k))
		_, ok = vcf.Header.Infos[LEFT+k]
		c.Assert(ok, Equals, false, Commentf(k))
		_, ok = vcf.Header.Infos[RIGHT+k]
		c.Assert(ok, Equals, false, Commentf(k))
	}

}

func (s *AnnoSuite) TestUpdateHeaderEnds(c *C) {
	vcf := irelate.Vopen("example/query.vcf")
	updateHeader([]annotation{cfgBed}, 0, vcf, true)

	for _, k := range cfgBed.Names {
		_, ok := vcf.Header.Infos[k]
		c.Assert(ok, Equals, true, Commentf(k))
		_, ok = vcf.Header.Infos[LEFT+k]
		c.Assert(ok, Equals, true, Commentf(k))
		_, ok = vcf.Header.Infos[RIGHT+k]
		c.Assert(ok, Equals, true, Commentf(k))
	}

}

func (s *AnnoSuite) TestAnnoMain(c *C) {
	var configs Config

	if _, err := toml.DecodeFile("example/conf.toml", &configs); err != nil {
		panic(err)
	}

	out := bufio.NewWriter(ioutil.Discard)
	Anno("example/query.vcf", configs, out, false, true)
	out.Flush()
	Anno("example/query.vcf", configs, out, true, true)
	out.Flush()

}

func (s *AnnoSuite) TestAnnoMainBed(c *C) {
	var configs Config

	if _, err := toml.DecodeFile("example/conf.toml", &configs); err != nil {
		panic(err)
	}

	out := bufio.NewWriter(ioutil.Discard)
	Anno("example/fitcons.bed", configs, out, false, false)
	out.Flush()

}
*/
