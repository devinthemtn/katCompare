package main

import (
	"embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp(initialPaths(os.Args[1:]))

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "File Compare",
		Width:     1280,
		Height:    860,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// initialPaths turns up to two command-line arguments into absolute paths for
// the left/right inputs. Flag-like arguments (e.g. macOS -psn_*) are ignored.
func initialPaths(args []string) []string {
	var paths []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if abs, err := filepath.Abs(arg); err == nil {
			arg = abs
		}
		paths = append(paths, arg)
		if len(paths) == 2 {
			break
		}
	}
	return paths
}
