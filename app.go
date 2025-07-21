package main

import (
	"BeamNGMode-Wails/service/downloader"
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) SelectFolder() string {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{})
	if err != nil {
		fmt.Println("Error:", err.Error())
	}
	fmt.Println("Selected directory:", dir)
	return dir
}

func (a *App) ProcessMods(urls []string, outputDir string) {
	for i, url := range urls {
		fmt.Println("URL", i, ":", url)
	}
	processor := downloader.NewModsProcessor(urls, outputDir, 10, 3, 3, a.ctx)
	processor.ProcessMods()
}
