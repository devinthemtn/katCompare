package differ

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// referenceLCSLength is the textbook table the diff used to be built on. It's
// always exact, so it's what the faster code is checked against.
func referenceLCSLength(left, right []string) int {
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for i := 1; i <= len(left); i++ {
		for j := 1; j <= len(right); j++ {
			switch {
			case left[i-1] == right[j-1]:
				current[j] = previous[j-1] + 1
			case previous[j] >= current[j-1]:
				current[j] = previous[j]
			default:
				current[j] = current[j-1]
			}
		}
		previous, current = current, previous
	}
	return previous[len(right)]
}

// checkMatches fails unless matches pair equal lines, strictly in order.
func checkMatches(t testing.TB, left, right []string, matches [][2]int) {
	t.Helper()
	lastI, lastJ := -1, -1
	for _, match := range matches {
		i, j := match[0], match[1]
		if i <= lastI || j <= lastJ {
			t.Fatalf("out of order: %v\nleft  %q\nright %q", matches, left, right)
		}
		if i < 0 || i >= len(left) || j < 0 || j >= len(right) {
			t.Fatalf("out of range: %v\nleft  %q\nright %q", match, left, right)
		}
		if left[i] != right[j] {
			t.Fatalf("%v pairs %q with %q", match, left[i], right[j])
		}
		lastI, lastJ = i, j
	}
}

// checkDiff fails unless a diff's lines rebuild both inputs exactly.
func checkDiff(t testing.TB, left, right []string) {
	t.Helper()
	diff := New().CompareStrings("l", "r", strings.Join(left, "\n"), strings.Join(right, "\n"))
	var gotLeft, gotRight []string
	for _, line := range diff.Lines {
		switch line.Type {
		case DiffEqual:
			gotLeft = append(gotLeft, line.Content)
			gotRight = append(gotRight, line.RightContent)
		case DiffDelete:
			gotLeft = append(gotLeft, line.Content)
		case DiffInsert:
			gotRight = append(gotRight, line.Content)
		}
	}
	// CompareStrings drops one trailing empty line, as left by a final newline.
	want := func(lines []string) []string {
		joined := strings.Split(strings.Join(lines, "\n"), "\n")
		if joined[len(joined)-1] == "" {
			joined = joined[:len(joined)-1]
		}
		return joined
	}
	if !reflect.DeepEqual(append([]string{}, gotLeft...), append([]string{}, want(left)...)) {
		t.Fatalf("left rebuilt as %q from %q", gotLeft, left)
	}
	if !reflect.DeepEqual(append([]string{}, gotRight...), append([]string{}, want(right)...)) {
		t.Fatalf("right rebuilt as %q from %q", gotRight, right)
	}
}

func randomLines(r *rand.Rand, n, distinct int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprint("line", r.Intn(distinct))
	}
	return lines
}

// mutate copies lines with some deleted, changed and inserted.
func mutate(r *rand.Rand, lines []string, rate float64, distinct int) []string {
	var out []string
	for _, line := range lines {
		switch p := r.Float64(); {
		case p < rate/3:
		case p < 2*rate/3:
			out = append(out, fmt.Sprint("line", r.Intn(distinct)))
		case p < rate:
			out = append(out, line, fmt.Sprint("line", r.Intn(distinct)))
		default:
			out = append(out, line)
		}
	}
	return out
}

func TestMatchesAreAsLongAsTheReference(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for trial := 0; trial < 3000; trial++ {
		distinct := 1 + r.Intn(12)
		left := randomLines(r, r.Intn(60), distinct)
		var right []string
		if r.Intn(2) == 0 {
			right = randomLines(r, r.Intn(60), distinct)
		} else {
			right = mutate(r, left, r.Float64(), distinct)
		}
		matches := lcsMatches(left, right)
		checkMatches(t, left, right, matches)
		if want := referenceLCSLength(left, right); len(matches) != want {
			t.Fatalf("%d matches, reference found %d\nleft  %q\nright %q", len(matches), want, left, right)
		}
		checkDiff(t, left, right)
	}
}

func TestMatchesOnLargerInputs(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	for _, c := range []struct {
		n, distinct int
		rate        float64
	}{
		{2000, 2000, 0.01},
		{2000, 2000, 0.5},
		{2000, 5, 0.1},
		{2000, 1, 0.1},
	} {
		left := randomLines(r, c.n, c.distinct)
		right := mutate(r, left, c.rate, c.distinct)
		matches := lcsMatches(left, right)
		checkMatches(t, left, right, matches)
		if want := referenceLCSLength(left, right); len(matches) != want {
			t.Errorf("%+v: %d matches, reference found %d", c, len(matches), want)
		}
	}
}

func TestMatchesEdgeCases(t *testing.T) {
	line := func(s ...string) []string { return s }
	cases := []struct {
		name        string
		left, right []string
		want        int
	}{
		{"both empty", nil, nil, 0},
		{"left empty", nil, line("a"), 0},
		{"right empty", line("a"), nil, 0},
		{"identical", line("a", "b", "c"), line("a", "b", "c"), 3},
		{"nothing in common", line("a", "b"), line("c", "d"), 0},
		{"insert at start", line("b", "c"), line("a", "b", "c"), 2},
		{"insert at end", line("a", "b"), line("a", "b", "c"), 2},
		{"delete in middle", line("a", "b", "c"), line("a", "c"), 2},
		{"swapped", line("a", "b"), line("b", "a"), 1},
		{"prefix and suffix overlap", line("a", "a"), line("a", "a", "a"), 2},
		{"all the same line", line("x", "x", "x", "x"), line("x", "x"), 2},
	}
	for _, c := range cases {
		matches := lcsMatches(c.left, c.right)
		checkMatches(t, c.left, c.right, matches)
		if len(matches) != c.want {
			t.Errorf("%s: %v, want %d matches", c.name, matches, c.want)
		}
	}
}

// Past the effort limit a diff may not be the shortest, but it's still correct.
func TestLargeHardInputsStayCorrect(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	r := rand.New(rand.NewSource(3))
	for _, c := range []struct{ n, distinct int }{{30000, 7}, {30000, 40}} {
		left := randomLines(r, c.n, c.distinct)
		right := randomLines(r, c.n, c.distinct)
		matches := lcsMatches(left, right)
		checkMatches(t, left, right, matches)
		if len(matches) < c.n/c.distinct {
			t.Errorf("%+v: only %d matches: the result is valid but far too poor", c, len(matches))
		}
		checkDiff(t, left, right)
	}
}

func FuzzMatches(f *testing.F) {
	f.Add("a\nb\nc\n", "a\nc\nd\n")
	f.Add("", "x\n")
	f.Add("x\nx\nx\n", "x\n")
	f.Add("a\r\nb\r\n", "a\nb\n")
	f.Fuzz(func(t *testing.T, left, right string) {
		l, r := strings.Split(left, "\n"), strings.Split(right, "\n")
		if len(l) > 300 || len(r) > 300 {
			t.Skip()
		}
		matches := lcsMatches(l, r)
		checkMatches(t, l, r, matches)
		if want := referenceLCSLength(l, r); len(matches) != want {
			t.Fatalf("%d matches, reference found %d", len(matches), want)
		}
	})
}
