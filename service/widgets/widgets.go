package widgets

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ProgressWidget struct {
	Ctx      context.Context
	URL      string
	Title    string
	Progress float64
	Status   string
}

func NewProgressWidget(context context.Context, url string) *ProgressWidget {
	return &ProgressWidget{
		Ctx:      context,
		URL:      url,
		Title:    "...",
		Progress: 0,
		Status:   "",
	}
}

func (pw *ProgressWidget) SetProgress(value float64) {
	runtime.EventsEmit(pw.Ctx, "progress", map[string]interface{}{
		"url":      pw.URL,
		"progress": value * 100,
	})
}

func (pw *ProgressWidget) SetStatus(text string) {
	runtime.EventsEmit(pw.Ctx, "status", map[string]interface{}{
		"url":    pw.URL,
		"status": text,
	})
}

func (pw *ProgressWidget) SetTitle(text string) {
	runtime.EventsEmit(pw.Ctx, "title", map[string]interface{}{
		"url":   pw.URL,
		"title": text,
	})
}
