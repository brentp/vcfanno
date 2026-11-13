package api

import (
	"fmt"
	"github.com/brentp/irelate/interfaces"
	"github.com/brentp/irelate/parsers"
	"github.com/brentp/vcfgo"
	"reflect"
	"testing"
)

func TestBedNumericMissing(t *testing.T) {
	for _, op := range []string{"mean", "min", "max", "sum", "count", "self"} {
		for _, values := range [][]string{{"NA"}, {"."}, {"-5", "NA"}, {"6", "."}, {"0", "-2", "bad"}, {"1.5", "-2"}, {"1e1000"}} {
			t.Run(op+fmt.Sprint(values), func(t *testing.T) {
				header := vcfgo.NewHeader()
				query := makeVariant("chr1", 100, "A", []string{"C"}, ".", "", header)
				var related []interfaces.Relatable
				for _, value := range values {
					interval, err := parsers.IntervalFromBedLine([]byte("chr1\t99\t100\t" + value))
					if err != nil {
						t.Fatal(err)
					}
					interval.SetSource(1)
					related = append(related, interval)
				}
				src := Source{Op: op, Column: 4, Name: "score"}
				got, err := collect(query, related, &src, false)
				if op == "count" || op == "self" {
					if len(got) != len(values) || err != nil {
						t.Fatalf("non-numeric operation lost values: %v, %v", got, err)
					}
					return
				}
				// Explicit expected collections: missing/invalid values cannot be measured zero.
				var want []interface{}
				switch fmt.Sprint(values) {
				case "[-5 NA]":
					want = []interface{}{float64(-5)}
				case "[6 .]":
					want = []interface{}{float64(6)}
				case "[0 -2 bad]":
					want = []interface{}{float64(0), float64(-2)}
				case "[1.5 -2]":
					want = []interface{}{float64(1.5), float64(-2)}
				}
				if len(got) != len(want) || (len(want) > 0 && !reflect.DeepEqual(got, want)) {
					t.Fatalf("got %v; want %v", got, want)
				}
				if (err != nil) != (fmt.Sprint(values) != "[1.5 -2]") {
					t.Fatalf("unexpected diagnostic: %v", err)
				}
				src.AnnotateOne(query, got, "")
				if len(want) == 0 {
					value, _ := query.Info().Get("score")
					if value != nil {
						t.Fatalf("all-missing annotation emitted %v", value)
					}
				} else {
					value, _ := query.Info().Get("score")
					expected := Reducers[op](want)
					if fmt.Sprint(value) != fmt.Sprint(expected) {
						t.Fatalf("got annotation %v; want %v", value, expected)
					}
				}
			})
		}
	}
}
