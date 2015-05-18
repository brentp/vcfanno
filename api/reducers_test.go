package api

import (
	"strings"

	. "gopkg.in/check.v1"
)

type ReducerSuite struct {
	ints    []interface{}
	one_int []interface{}

	strings    []interface{}
	one_string []interface{}
	floats     []interface{}
	one_float  []interface{}

	empty []interface{}
}

var _ = Suite(&ReducerSuite{})

func (s *ReducerSuite) SetUpTest(c *C) {
	s.ints = []interface{}{1, 2, 3, 4, 5, 6, 7}
	s.one_int = []interface{}{33}

	s.floats = []interface{}{1.3, 2.3, 3.3, 4.4, 5.5, 6.6, 7.7}
	s.one_float = []interface{}{33.33}

	s.strings = []interface{}{"aa", "bb", "cc", "ee", "ff", "gg", "hh"}
	s.one_string = []interface{}{"justme"}

}

func (s *ReducerSuite) TestFloats(c *C) {

	u := uniq(s.floats)
	toks := strings.Split(u.(string), ",")
	c.Assert(len(toks), Equals, len(s.floats))

	cnt := count(s.floats)
	c.Assert(cnt, Equals, len(s.floats))

	f := first(s.floats)
	c.Assert(f, Equals, s.floats[0])

	m := min(s.floats)
	c.Assert(m.(float32), Equals, float32(1.3))

	m = max(s.floats)
	c.Assert(m, Equals, float32(7.7))

}

func (s *ReducerSuite) TestFloat(c *C) {
	u := uniq(s.one_float)
	toks := strings.Split(u.(string), ",")
	c.Assert(len(toks), Equals, len(s.one_float))

	cnt := count(s.one_float)
	c.Assert(cnt, Equals, 1)

	f := first(s.one_float)
	c.Assert(f, Equals, s.one_float[0])

	m := min(s.one_float)
	c.Assert(float64(m.(float32))-s.one_float[0].(float64) < float64(0.0001), Equals, true)

	m = max(s.one_float)
	//c.Assert(float64(m.(float32)), Equals, s.one_float[0].(float64))
}
