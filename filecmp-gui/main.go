package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// icon is the application/window icon on Linux (Windows uses the icon
// embedded from build/windows/icon.ico).
//
//go:embed build/appicon.png
var icon []byte

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	version = resolveVersion()
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" {
			fmt.Printf("filecmp-gui %s\n", version)
			return
		}
	}

	app := application.New(application.Options{
		Name:        "filecmp-gui",
		Description: "Compare files and directories side by side",
		Icon:        icon,
		Services: []application.Service{
			application.NewService(NewApp(initialPaths(os.Args[1:]))),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			ProgramName: "filecmp-gui",
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "File Compare",
		Width:            1280,
		Height:           860,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// resolveVersion prefers the ldflags-stamped version, falling back to the
// module version recorded by "go install module@version".
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
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
