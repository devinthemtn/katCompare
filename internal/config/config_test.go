package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestInstallCreatesTheFileWithItsParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "config.toml")
	created, err := installAt(path)
	if err != nil || !created {
		t.Fatalf("installAt = %v, %v", created, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != template {
		t.Errorf("file = %q, %v", data, err)
	}
}

func TestInstallNeverOverwrites(t *testing.T) {
	cases := map[string]string{
		"a customised file": "[gui]\nauto_collapse_paths = true\n# my note\n",
		"an empty file":     "",
		"an invalid file":   "this is = = not toml",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			before, _ := os.Stat(path)

			created, err := installAt(path)
			if err != nil || created {
				t.Fatalf("installAt = %v, %v; want it left alone", created, err)
			}
			data, _ := os.ReadFile(path)
			if string(data) != content {
				t.Errorf("content changed to %q", data)
			}
			after, _ := os.Stat(path)
			if after.Mode() != before.Mode() || !after.ModTime().Equal(before.ModTime()) {
				t.Error("the file was touched")
			}
		})
	}
}

func TestInstallLeavesOtherThingsAtThePathAlone(t *testing.T) {
	dir := t.TempDir()

	// A directory
	path := filepath.Join(dir, "config.toml")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if created, _ := installAt(path); created {
		t.Error("created over a directory")
	}

	if runtime.GOOS == "windows" {
		return // Symlinks need extra privileges there.
	}

	// A symlink to the user's real file, as in a dotfiles repo
	real := filepath.Join(dir, "dotfiles-config.toml")
	if err := os.WriteFile(real, []byte("mine\n"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linked.toml")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if created, err := installAt(link); created || err != nil {
		t.Errorf("symlink: installAt = %v, %v", created, err)
	}
	if data, _ := os.ReadFile(real); string(data) != "mine\n" {
		t.Errorf("the link's target changed to %q", data)
	}

	// A symlink whose target doesn't exist yet: creating through it would put a
	// file somewhere the user didn't choose.
	target := filepath.Join(dir, "not-there-yet.toml")
	dangling := filepath.Join(dir, "dangling.toml")
	if err := os.Symlink(target, dangling); err != nil {
		t.Fatal(err)
	}
	if created, _ := installAt(dangling); created {
		t.Error("created through a dangling symlink")
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("wrote through a dangling symlink")
	}
}

func TestSimultaneousInstallsCreateOneFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := installAt(path); err == nil && ok {
				mu.Lock()
				created++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if created != 1 {
		t.Errorf("%d copies reported creating the file, want 1", created)
	}
	if data, _ := os.ReadFile(path); string(data) != template {
		t.Errorf("file = %q", data)
	}
}

func TestInstallReportsFailure(t *testing.T) {
	// The parent "directory" is a file, so nothing can be created under it.
	dir := t.TempDir()
	parent := filepath.Join(dir, "parent")
	if err := os.WriteFile(parent, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if created, err := installAt(filepath.Join(parent, "config.toml")); created || err == nil {
		t.Errorf("installAt = %v, %v; want an error", created, err)
	}
}

func TestTemplateMatchesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := installAt(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatalf("the template doesn't load: %v", err)
	}
	want := Defaults()
	want.Compare.Ignore = []string{} // the template's "ignore = []"
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("template gives %+v, defaults are %+v", cfg, want)
	}
}

func TestLoad(t *testing.T) {
	cases := []struct {
		name, content string
		want          bool
		wantErr       bool
	}{
		{"on", "[gui]\nauto_collapse_paths = true\n", true, false},
		{"off", "[gui]\nauto_collapse_paths = false\n", false, false},
		{"empty", "", false, false},
		{"setting missing", "[gui]\n", false, false},
		{"comments only", "# nothing\n", false, false},
		{"unknown settings are ignored", "future = 1\n[gui]\nauto_collapse_paths = true\nnew_thing = \"x\"\n[other]\nkey = 2\n", true, false},
		{"Windows line endings", "[gui]\r\nauto_collapse_paths = true\r\n", true, false},
		{"byte order mark from Notepad", "\xef\xbb\xbf[gui]\nauto_collapse_paths = true\n", true, false},
		{"wrong type", "[gui]\nauto_collapse_paths = \"yes\"\n", false, true},
		{"not TOML", "this is = = not toml", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(c.content), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadFrom(path)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, want error: %v", err, c.wantErr)
			}
			if cfg.GUI.AutoCollapsePaths != c.want {
				t.Errorf("AutoCollapsePaths = %v, want %v", cfg.GUI.AutoCollapsePaths, c.want)
			}
			if data, _ := os.ReadFile(path); string(data) != c.content {
				t.Error("loading changed the file")
			}
		})
	}
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	cfg, err := loadFrom(filepath.Join(t.TempDir(), "nothing.toml"))
	if err != nil || !reflect.DeepEqual(cfg, Defaults()) {
		t.Errorf("Load = %+v, %v", cfg, err)
	}
}

func TestPathIsUnderTheConfigDirectory(t *testing.T) {
	path, err := Path()
	if err != nil {
		t.Skipf("no config directory here: %v", err)
	}
	dir, _ := os.UserConfigDir()
	if want := filepath.Join(dir, "katcompare", "config.toml"); path != want {
		t.Errorf("Path = %q, want %q", path, want)
	}
}

func TestOldFilesKeepTheNewDefaults(t *testing.T) {
	// The file an earlier version installed: only [gui].
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[gui]\nauto_collapse_paths = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.GUI.AutoCollapsePaths = true
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestEverySettingLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := `[compare]
ignore = ["node_modules", "*.log", "docs/generated/"]
show_hidden = true
hide_identical = true
ignore_line_endings = true
ignore_whitespace = "trailing"
[view]
default_view = "sidebyside"
default_merge_target = "right"
[tools]
editor = '"C:\Program Files\x\x.exe" -n'
[gui]
auto_collapse_paths = true
diff_font_size = 14
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Compare: Compare{Ignore: []string{"node_modules", "*.log", "docs/generated/"}, ShowHidden: true, HideIdentical: true, IgnoreLineEndings: true, IgnoreWhitespace: "trailing"},
		View:    View{DefaultView: "sidebyside", DefaultMergeTarget: "right"},
		Tools:   Tools{Editor: `"C:\Program Files\x\x.exe" -n`},
		GUI:     GUI{AutoCollapsePaths: true, DiffFontSize: 14},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got %+v\nwant %+v", cfg, want)
	}
	if o := cfg.DifferOptions(); !o.IgnoreLineEndings || o.IgnoreWhitespace != "trailing" {
		t.Errorf("DifferOptions = %+v", o)
	}
	if o := cfg.FileOptions(); !o.ShowHidden || len(o.Ignore) != 3 {
		t.Errorf("FileOptions = %+v", o)
	}
}

func TestInvalidValuesGiveDefaultsAndSaySo(t *testing.T) {
	cases := map[string]struct{ content, mention string }{
		"whitespace":   {"[compare]\nignore_whitespace = \"some\"\n", "ignore_whitespace"},
		"empty value":  {"[compare]\nignore_whitespace = \"\"\n", "ignore_whitespace"},
		"view":         {"[view]\ndefault_view = \"merge\"\n", "default_view"},
		"target":       {"[view]\ndefault_merge_target = \"up\"\n", "default_merge_target"},
		"font small":   {"[gui]\ndiff_font_size = 2\n", "diff_font_size"},
		"font large":   {"[gui]\ndiff_font_size = 100\n", "diff_font_size"},
		"bad wildcard": {"[compare]\nignore = [\"[a\"]\n", "ignore"},
		"empty ignore": {"[compare]\nignore = [\"\"]\n", "ignore"},
		"only slash":   {"[compare]\nignore = [\"/\"]\n", "ignore"},
		"wrong type":   {"[compare]\nignore = \"node_modules\"\n", "ignore"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			// One good setting alongside: a bad file gives all defaults.
			content := "[tools]\neditor = \"vim\"\n" + c.content
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := loadFrom(path)
			if err == nil || !strings.Contains(err.Error(), c.mention) {
				t.Errorf("err = %v, want it to mention %q", err, c.mention)
			}
			if !reflect.DeepEqual(cfg, Defaults()) {
				t.Errorf("cfg = %+v, want the defaults", cfg)
			}
			if data, _ := os.ReadFile(path); string(data) != content {
				t.Error("the file changed")
			}
		})
	}
}

// The examples in the template's comments should work if uncommented.
func TestTemplateExamplesAreValid(t *testing.T) {
	var uncommented []string
	for _, line := range strings.Split(template, "\n") {
		if strings.HasPrefix(line, "# ignore = ") || strings.HasPrefix(line, "# editor = ") {
			line = strings.TrimPrefix(line, "# ")
		} else if strings.HasPrefix(line, "ignore = ") || strings.HasPrefix(line, "editor = ") {
			continue // replaced by the example
		}
		uncommented = append(uncommented, line)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join(uncommented, "\n")), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Compare.Ignore) != 5 {
		t.Errorf("ignore = %v", cfg.Compare.Ignore)
	}
	if want := `"C:\Program Files\Notepad++\notepad++.exe" -n`; cfg.Tools.Editor != want {
		t.Errorf("editor = %q, want %q", cfg.Tools.Editor, want)
	}
}
