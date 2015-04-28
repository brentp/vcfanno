package main

import (
	"github.com/brentp/irelate"
	"github.com/brentp/vcfgo"
	. "gopkg.in/check.v1"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type AnnoSuite struct {
	v1 *irelate.Variant
	v2 *irelate.Variant
	v3 *irelate.Variant
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

}

func (s *AnnoSuite) TestPartition(c *C) {

	sep := Partition(s.v1, 1)
	c.Assert(sep[0], DeepEquals, []irelate.Relatable{s.v2, s.v3})

}

func (s *AnnoSuite) TestAnno(c *C) {

	cfg := anno{
		File:   "fake file",
		Ops:    []string{"mean", "min", "max", "concat", "uniq", "first"},
		Fields: []string{"DP", "DP", "DP", "DP", "DP", "DP", "DP"},
		Names:  []string{"dp_mean", "dp_min", "dp_max", "dp_concat", "dp_uniq", "dp_first"},
	}
	sep := Partition(s.v1, 1)
	updateInfo(s.v1.Variant, sep, []anno{cfg})

	c.Assert(s.v1.Info["dp_mean"], Equals, float32(66.0))
	c.Assert(s.v1.Info["dp_min"], Equals, float32(44.0))
	c.Assert(s.v1.Info["dp_max"], Equals, float32(88.0))
	c.Assert(s.v1.Info["dp_concat"], Equals, "44,88")
	c.Assert(s.v1.Info["dp_uniq"], Equals, "44,88")
	c.Assert(s.v1.Info["dp_first"], Equals, uint32(44))

}
