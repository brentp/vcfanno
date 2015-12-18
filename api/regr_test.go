package api

import (
	"github.com/brentp/irelate/parsers"
	"github.com/brentp/vcfgo"
	. "gopkg.in/check.v1"
)

type RegrSuite struct {
	h *vcfgo.Header

	v1 *parsers.Variant
	v2 *parsers.Variant

	bed *parsers.Interval

	src_disease Source
	src_pmids   Source

	annotator *Annotator
}

var _ = Suite(&RegrSuite{})

func (s *RegrSuite) SetUpTest(c *C) {
	s.h = vcfgo.NewHeader()
	s.h.Infos["DOCM_DISEASE"] = &vcfgo.Info{Id: "DOCM_DISEASE", Description: "DOCM_DISEASE", Type: "String", Number: "."}
	s.h.Infos["DOCM_PMIDS"] = &vcfgo.Info{Id: "DOCM_PMIDS", Description: "DOCM_PMIDS", Type: "String", Number: "."}

	s.v1 = makeVariant("1", 22, "A", []string{"T"}, "query", "", s.h)
	s.v1.SetSource(0)
	s.v2 = makeVariant("1", 22, "A", []string{"T"}, "docm", "DOCM_DISEASE=chronic_myeloid_leukemia,acute_myeloid_leukemia;DOCM_PMIDS=23634996,23656643", s.h)
	s.v2.SetSource(1)

	s.src_disease = Source{
		File:  "docm.vcf.gz",
		Op:    "concat",
		Field: "DOCM_DISEASE",
		Name:  "docm_disease",
		Index: 0,
	}

	s.src_pmids = Source{
		File:  "docm.vcf.gz",
		Op:    "uniq",
		Field: "DOCM_PMIDS",
		Name:  "docm_pmids",
		Index: 0,
	}

	s.v1.AddRelated(s.v2)

	empty := make([]PostAnnotation, 0)
	s.annotator = NewAnnotator([]*Source{&s.src_disease, &s.src_pmids}, "", true, false, empty)

}

func (s *RegrSuite) TestAnno(c *C) {

	s.annotator.AnnotateOne(s.v1, true)
	c.Assert(s.v1.Info().String(), Equals, "docm_disease=chronic_myeloid_leukemia,acute_myeloid_leukemia;docm_pmids=23634996,23656643")
}
