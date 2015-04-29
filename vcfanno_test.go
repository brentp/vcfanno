package main

import (
	"fmt"
	"testing"

	"github.com/brentp/irelate"
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

var _ = Suite(&AnnoSuite{})

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

func (s *AnnoSuite) SetUpTest(c *C) {
	s.v1 = &irelate.Variant{Variant: v1}
	s.v1.SetSource(0)
	v2 := *v1
	v2.Info = map[string]interface{}{"DP": uint32(44)}
	s.v2 = &irelate.Variant{Variant: &v2}
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

	s.b = irelate.IntervalFromBedLine("chr1\t224\t244\t111\t222").(*irelate.Interval)
	s.b.SetSource(2)
	s.v1.AddRelated(s.b)

}

func (s *AnnoSuite) TestPartition(c *C) {

	sep := Partition(s.v1, 2)
	c.Assert(sep[0], DeepEquals, []irelate.Relatable{s.v2, s.v3})
	c.Assert(sep[1], DeepEquals, []irelate.Relatable{s.b})

}

func (s *AnnoSuite) TestAnno(c *C) {

	cfg := anno{
		File:   "fake file",
		Ops:    []string{"mean", "min", "max", "concat", "uniq", "first"},
		Fields: []string{"DP", "DP", "DP", "DP", "DP", "DP", "DP"},
		Names:  []string{"dp_mean", "dp_min", "dp_max", "dp_concat", "dp_uniq", "dp_first"},
	}
	cfgBed := anno{
		File:    "bed file",
		Ops:     []string{"mean", "max", "flag"},
		Columns: []int{4, 5, 1},
		Names:   []string{"bed_mean", "bed_max", "bedFlag"},
	}

	sep := Partition(s.v1, 2)
	updateInfo(s.v1.Variant, sep, []anno{cfg, cfgBed})

	c.Assert(s.v1.Info["dp_mean"], Equals, float32(66.0))
	c.Assert(s.v1.Info["dp_min"], Equals, float32(44.0))
	c.Assert(s.v1.Info["dp_max"], Equals, float32(88.0))
	c.Assert(s.v1.Info["dp_concat"], Equals, "44,88")
	c.Assert(s.v1.Info["dp_uniq"], Equals, "44,88")
	c.Assert(s.v1.Info["dp_first"], Equals, uint32(44))

	c.Assert(s.v1.Info["bed_mean"], Equals, float32(111))
	c.Assert(s.v1.Info["bed_max"], Equals, float32(222))

	c.Assert(s.v1.Info["bedFlag"], Equals, true)

	c.Assert(fmt.Sprintf("%s", s.v1.Info), Equals, "DP=35;dp_mean=66;dp_min=44;dp_max=88;dp_concat=44,88;dp_uniq=44,88;dp_first=44;bed_mean=111;bed_max=222;bedFlag")
}

func (s *AnnoSuite) TestCheck(c *C) {
	cfgBed := anno{
		File:    "bed file",
		Ops:     []string{"mean", "max", "flag"},
		Columns: []int{4, 5},
		Names:   []string{"bed_mean", "bed_max", "bedFlag"},
	}
	e := checkAnno(cfgBed)
	c.Assert(e, ErrorMatches, "must specify same # of 'columns' as 'ops' for bed file")

	cfgBed.Fields = []string{"abc", "def"}
	e = checkAnno(cfgBed)
	c.Assert(e, ErrorMatches, "specify only 'fields' or 'columns' not both bed file")

	cfgBed.Columns = nil
	e = checkAnno(cfgBed)
	c.Assert(e, ErrorMatches, "must specify same # of 'fields' as 'ops' for bed file")
}
