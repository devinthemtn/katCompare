package conflict

import (
	"reflect"
	"testing"
)

const twoWay = `a
<<<<<<< HEAD
ours1
ours2
=======
theirs1
>>>>>>> feature
b
<<<<<<< HEAD
x
=======
y
>>>>>>> feature
c
`

func TestParseTwoWay(t *testing.T) {
	f := Parse(twoWay)
	if len(f.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(f.Blocks))
	}
	b := f.Blocks[0]
	if b.StartLine != 2 || b.OursLabel != "HEAD" || b.TheirsLabel != "feature" {
		t.Errorf("block 0 = %+v", b)
	}
	if !reflect.DeepEqual(b.Ours, []string{"ours1", "ours2"}) || !reflect.DeepEqual(b.Theirs, []string{"theirs1"}) {
		t.Errorf("block 0 sides = %q / %q", b.Ours, b.Theirs)
	}
	if f.Blocks[1].StartLine != 9 {
		t.Errorf("block 1 start = %d, want 9", f.Blocks[1].StartLine)
	}
}

func TestRender(t *testing.T) {
	f := Parse(twoWay)
	tests := []struct {
		choices []Choice
		want    string
	}{
		{[]Choice{Ours, Theirs}, "a\nours1\nours2\nb\ny\nc\n"},
		{[]Choice{OursThenTheirs, TheirsThenOurs}, "a\nours1\nours2\ntheirs1\nb\ny\nx\nc\n"},
		{nil, twoWay}, // nothing chosen: unchanged
		{[]Choice{Theirs}, "a\ntheirs1\nb\n<<<<<<< HEAD\nx\n=======\ny\n>>>>>>> feature\nc\n"}, // partial
	}
	for _, tt := range tests {
		if got := f.Render(tt.choices); got != tt.want {
			t.Errorf("Render(%v) =\n%q\nwant\n%q", tt.choices, got, tt.want)
		}
	}
}

func TestParseDiff3(t *testing.T) {
	f := Parse("<<<<<<< ours\no\n||||||| base\nb\n=======\nt\n>>>>>>> theirs\n")
	if len(f.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(f.Blocks))
	}
	b := f.Blocks[0]
	if !reflect.DeepEqual(b.Base, []string{"b"}) || b.BaseLabel != "base" ||
		!reflect.DeepEqual(b.Ours, []string{"o"}) || !reflect.DeepEqual(b.Theirs, []string{"t"}) {
		t.Errorf("block = %+v", b)
	}
	if got := f.Render([]Choice{Theirs}); got != "t\n" {
		t.Errorf("Render = %q", got)
	}
}

func TestCRLFAndNoTrailingNewline(t *testing.T) {
	f := Parse("a\r\n<<<<<<< HEAD\r\no\r\n=======\r\nt\r\n>>>>>>> x\r\nz")
	if len(f.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(f.Blocks))
	}
	if got := f.Render([]Choice{Ours}); got != "a\r\no\r\nz" {
		t.Errorf("Render = %q", got)
	}
}

func TestUnterminatedIsPlain(t *testing.T) {
	in := "a\n<<<<<<< HEAD\no\n=======\nt\n"
	f := Parse(in)
	if len(f.Blocks) != 0 {
		t.Fatalf("blocks = %d, want 0", len(f.Blocks))
	}
	if got := f.Render(nil); got != in {
		t.Errorf("Render = %q, want unchanged", got)
	}
}

func TestContextAndRemaining(t *testing.T) {
	f := Parse(twoWay)
	before, after := f.Context(1, 3)
	if !reflect.DeepEqual(before, []string{"b"}) || !reflect.DeepEqual(after, []string{"c"}) {
		t.Errorf("Context(1) = %q, %q", before, after)
	}
	if n := Remaining(2, []Choice{Ours}); n != 1 {
		t.Errorf("Remaining = %d, want 1", n)
	}
}

func TestChoiceNames(t *testing.T) {
	for _, c := range []Choice{Unresolved, Ours, Theirs, OursThenTheirs, TheirsThenOurs} {
		if ParseChoice(c.String()) != c {
			t.Errorf("round trip failed for %v", c)
		}
	}
}
