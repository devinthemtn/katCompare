package differ

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// benchSizes are lines per side. The larger ones need KATCMP_BENCH_LARGE=1.
func benchSizes() []int {
	sizes := []int{1000, 12000}
	if os.Getenv("KATCMP_BENCH_LARGE") != "" {
		sizes = append(sizes, 50000, 200000)
	}
	return sizes
}

// benchInputs builds the two sides for each kind of file.
func benchInputs(n int) map[string][2]string {
	var sameL, sameR, diffL, diffR, repL, repR strings.Builder
	for i := 0; i < n; i++ {
		line := fmt.Sprintf("this is line number %d of the sample file\n", i)
		sameL.WriteString(line)
		if i%40 == 0 {
			sameR.WriteString("changed " + line)
		} else {
			sameR.WriteString(line)
		}
		diffL.WriteString(fmt.Sprintf("left only line %d\n", i))
		diffR.WriteString(fmt.Sprintf("right only line %d\n", i))
		// Few distinct lines, as in a file of braces and blanks.
		repL.WriteString(fmt.Sprintf("value %d\n", i%7))
		repR.WriteString(fmt.Sprintf("value %d\n", (i*3)%11))
	}
	return map[string][2]string{
		"few-changes":    {sameL.String(), sameR.String()},
		"all-different":  {diffL.String(), diffR.String()},
		"repeated-lines": {repL.String(), repR.String()},
	}
}

func BenchmarkCompare(b *testing.B) {
	for _, n := range benchSizes() {
		inputs := benchInputs(n)
		for _, kind := range []string{"few-changes", "all-different", "repeated-lines"} {
			in := inputs[kind]
			b.Run(fmt.Sprintf("%s/%d", kind, n), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					New().CompareStrings("l", "r", in[0], in[1])
				}
			})
		}
	}
}
