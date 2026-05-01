package widgets

import (
	"context"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ProgressWidget struct {
	Ctx      context.Context
	URL      string
	Title    string
	Progress float64
	Status   string
	Thumb    string

	Downloaded int64
	Total      int64
	startedAt  time.Time
}

func NewProgressWidget(context context.Context, url string) *ProgressWidget {
	return &ProgressWidget{
		Ctx:       context,
		URL:       url,
		Title:     "...",
		Progress:  0,
		Status:    "",
		startedAt: time.Now(),
	}
}

func (pw *ProgressWidget) SetProgress(value float64) {
	pw.Progress = value * 100
	runtime.EventsEmit(pw.Ctx, "progress", map[string]interface{}{
		"url":      pw.URL,
		"progress": pw.Progress,
	})
	pw.EmitQueueItem()
}

func (pw *ProgressWidget) SetProgressBytes(downloaded, total int64) {
	pw.Downloaded = downloaded
	pw.Total = total
	if total > 0 {
		pw.Progress = float64(downloaded) / float64(total) * 100
	}
	runtime.EventsEmit(pw.Ctx, "progress", map[string]interface{}{
		"url":      pw.URL,
		"progress": pw.Progress,
	})
	pw.EmitQueueItem()
}

func (pw *ProgressWidget) SetStatus(text string) {
	pw.Status = text
	runtime.EventsEmit(pw.Ctx, "status", map[string]interface{}{
		"url":    pw.URL,
		"status": text,
	})
	pw.EmitQueueItem()
	if text == "Завершено." {
		runtime.EventsEmit(pw.Ctx, "queue:done", map[string]interface{}{
			"url": pw.URL,
		})
	}
	if strings.HasPrefix(text, "Ошибка:") {
		runtime.EventsEmit(pw.Ctx, "queue:error", map[string]interface{}{
			"url":   pw.URL,
			"error": text,
		})
	}
}

func (pw *ProgressWidget) SetTitle(text string) {
	pw.Title = text
	runtime.EventsEmit(pw.Ctx, "title", map[string]interface{}{
		"url":   pw.URL,
		"title": text,
	})
	pw.EmitQueueItem()
}

func (pw *ProgressWidget) SetThumb(url string) {
	pw.Thumb = url
	pw.EmitQueueItem()
}

func (pw *ProgressWidget) EmitQueueItem() {
	elapsed := time.Since(pw.startedAt).Seconds()
	var speed float64
	var eta float64
	if elapsed > 0 && pw.Downloaded > 0 {
		speed = float64(pw.Downloaded) / elapsed
		if pw.Total > pw.Downloaded && speed > 0 {
			eta = float64(pw.Total-pw.Downloaded) / speed
		}
	}

	runtime.EventsEmit(pw.Ctx, "queue:item", map[string]interface{}{
		"url":              pw.URL,
		"title":            pw.Title,
		"status":           pw.Status,
		"progress":         pw.Progress,
		"downloadedBytes":  pw.Downloaded,
		"totalBytes":       pw.Total,
		"speedBytesPerSec": speed,
		"etaSeconds":       eta,
		"thumbnailUrl":     pw.Thumb,
	})
}
