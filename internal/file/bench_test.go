package file

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// retained returns the heap in use after a collection.
func retained() float64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.HeapAlloc)
}

func loadBench(b *testing.B, root string) {
	b.Helper()
	b.ReportAllocs()
	// The apps keep the tree and the file map for as long as a comparison is open.
	var tree *FileInfo
	var keep map[string]*FileComparison
	before := retained()
	for i := 0; i < b.N; i++ {
		info, err := New().LoadPath(root)
		if err != nil {
			b.Fatal(err)
		}
		tree, keep = info, FindAllFiles(info, info)
		Identical(keep, nil)
	}
	b.ReportMetric((retained()-before)/(1<<20), "MB-retained")
	runtime.KeepAlive(tree)
	runtime.KeepAlive(keep)
}

func BenchmarkLoadSmallFiles(b *testing.B) {
	root := b.TempDir()
	content := strings.Repeat("some text in a small file\n", 160) // about 4 KB
	for i := 0; i < 4000; i++ {
		dir := filepath.Join(root, fmt.Sprintf("d%d", i%40))
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte(content), 0644); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	loadBench(b, root)
}

func BenchmarkLoadWithBinaries(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("text\n"), 0644); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		f, err := os.Create(filepath.Join(root, fmt.Sprintf("video%d.bin", i)))
		if err != nil {
			b.Fatal(err)
		}
		// A sparse 100 MB file of NUL bytes, which reads as binary.
		if err := f.Truncate(100 << 20); err != nil {
			b.Fatal(err)
		}
		f.Close()
	}
	b.ResetTimer()
	loadBench(b, root)
}
