package main

import (
	"BeamNGMode-Wails/service/catalog"
	"BeamNGMode-Wails/service/downloader"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// App struct
type App struct {
	ctx   context.Context
	queue *QueueController
	db    *gorm.DB
}

type QueueMod struct {
	URL          string `json:"url"`
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

type InstalledMod struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Disabled bool   `json:"disabled"`
}

type AppSettings struct {
	OutputPath string `json:"outputPath"`
	GamePath   string `json:"gamePath"`
}

type SettingRecord struct {
	Key       string `gorm:"primaryKey"`
	Value     string
	UpdatedAt time.Time
}

type InstalledModRecord struct {
	URL          string    `gorm:"primaryKey" json:"url"`
	Title        string    `json:"title"`
	ThumbnailURL string    `json:"thumbnailUrl"`
	Path         string    `json:"path"`
	InstalledAt  time.Time `json:"installedAt"`
}

type QueueController struct {
	ctx       context.Context
	mu        sync.Mutex
	cond      *sync.Cond
	paused    map[string]bool
	cancelled map[string]bool
	completed map[string]bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	q := &QueueController{
		paused:    map[string]bool{},
		cancelled: map[string]bool{},
		completed: map[string]bool{},
	}
	q.cond = sync.NewCond(&q.mu)
	return &App{queue: q}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.queue.ctx = ctx
	if err := a.initDB(); err != nil {
		fmt.Println("DB init error:", err.Error())
	}
}

func (a *App) initDB() error {
	dbPath := filepath.Join(appDir(), "index.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}
	if err := db.AutoMigrate(&SettingRecord{}, &InstalledModRecord{}); err != nil {
		return err
	}
	a.db = db
	return nil
}

func appDir() string {
	if exePath, err := os.Executable(); err == nil {
		return filepath.Dir(exePath)
	}
	if path, err := exec.LookPath(os.Args[0]); err == nil {
		return filepath.Dir(path)
	}
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "."
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
	mods := make([]QueueMod, 0, len(urls))
	for _, url := range urls {
		mods = append(mods, QueueMod{URL: url})
	}
	a.QueueMods(mods, outputDir)
}

func (a *App) QueueMods(mods []QueueMod, outputDir string) {
	urls := make([]string, 0, len(mods))
	meta := make(map[string]downloader.QueueMeta, len(mods))
	for _, mod := range mods {
		url := strings.TrimSpace(mod.URL)
		if url == "" {
			continue
		}
		if a.IsModInstalled(url) {
			runtime.EventsEmit(a.ctx, "queue:item", map[string]interface{}{
				"url":          url,
				"title":        mod.Title,
				"status":       "Уже установлен",
				"progress":     100,
				"thumbnailUrl": mod.ThumbnailURL,
			})
			continue
		}
		urls = append(urls, url)
		meta[url] = downloader.QueueMeta{
			Title:        mod.Title,
			ThumbnailURL: mod.ThumbnailURL,
		}
		a.queue.Reset(url)
		runtime.EventsEmit(a.ctx, "queue:item", map[string]interface{}{
			"url":              url,
			"title":            mod.Title,
			"status":           "В очереди",
			"progress":         0,
			"downloadedBytes":  0,
			"totalBytes":       0,
			"speedBytesPerSec": 0,
			"etaSeconds":       0,
			"thumbnailUrl":     mod.ThumbnailURL,
		})
	}
	if len(urls) == 0 {
		return
	}

	processor := downloader.NewModsProcessor(urls, outputDir, 10, 3, 3, a.ctx)
	processor.SetQueueMeta(meta)
	processor.SetController(a.queue)
	go func() {
		processor.ProcessMods()
		for _, url := range urls {
			if !a.queue.IsCompleted(url) {
				continue
			}
			item := meta[url]
			_ = a.MarkModInstalled(url, item.Title, item.ThumbnailURL, filepath.Join(outputDir, "BeamNGWOWD"))
		}
	}()
}

func (a *App) GetCatalogPage(page int) ([]catalog.ModCard, error) {
	return catalog.FetchCarsPage(page)
}

func (a *App) SearchCatalog(query string, startPage int, limit int) (catalog.SearchResult, error) {
	return catalog.SearchCars(query, startPage, limit)
}

func (a *App) GetModDetails(url string) (catalog.ModDetails, error) {
	return catalog.FetchModDetails(url)
}

func (a *App) PauseMod(url string) {
	a.queue.Pause(url)
	runtime.EventsEmit(a.ctx, "queue:paused", map[string]interface{}{"url": url})
}

func (a *App) ResumeMod(url string) {
	a.queue.Resume(url)
}

func (a *App) CancelMod(url string) {
	a.queue.Cancel(url)
	runtime.EventsEmit(a.ctx, "queue:cancelled", map[string]interface{}{"url": url})
}

func (a *App) RemoveQueueItem(url string) {
	a.queue.Remove(url)
	runtime.EventsEmit(a.ctx, "queue:removed", map[string]interface{}{"url": url})
}

func (a *App) ClearCompleted() {
	a.queue.ClearCompleted()
	runtime.EventsEmit(a.ctx, "queue:clear-completed")
}

func (a *App) ListInstalledMods(path string) ([]InstalledMod, error) {
	modsDir := filepath.Join(path, "BeamNGWOWD")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []InstalledMod{}, nil
		}
		return nil, err
	}

	mods := []InstalledMod{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		disabled := strings.HasSuffix(name, ".disabled")
		isZip := strings.HasPrefix(name, "processed-") && (strings.HasSuffix(name, ".zip") || disabled)
		if !isZip {
			continue
		}
		fullPath := filepath.Join(modsDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}
		displayName := strings.TrimPrefix(strings.TrimSuffix(name, ".disabled"), "processed-")
		displayName = strings.TrimSuffix(displayName, ".zip")
		mods = append(mods, InstalledMod{
			Name:     displayName,
			Path:     fullPath,
			Size:     info.Size(),
			Disabled: disabled,
		})
	}
	return mods, nil
}

func (a *App) DisableMod(path string) error {
	if strings.HasSuffix(path, ".disabled") {
		return nil
	}
	return os.Rename(path, path+".disabled")
}

func (a *App) EnableMod(path string) error {
	if !strings.HasSuffix(path, ".disabled") {
		return nil
	}
	return os.Rename(path, strings.TrimSuffix(path, ".disabled"))
}

func (a *App) DeleteInstalledMod(path string) error {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "processed-") {
		return errors.New("refusing to delete a non-processed mod")
	}
	if a.db != nil {
		a.db.Where("path = ? OR title = ?", path, strings.TrimSuffix(strings.TrimPrefix(strings.TrimSuffix(base, ".disabled"), "processed-"), ".zip")).Delete(&InstalledModRecord{})
	}
	return os.Remove(path)
}

func (a *App) GetSettings() AppSettings {
	settings := AppSettings{OutputPath: "./downloaded"}
	if a.db == nil {
		return settings
	}
	var records []SettingRecord
	if err := a.db.Find(&records).Error; err != nil {
		return settings
	}
	for _, record := range records {
		switch record.Key {
		case "outputPath":
			if strings.TrimSpace(record.Value) != "" {
				settings.OutputPath = record.Value
			}
		case "gamePath":
			settings.GamePath = record.Value
		}
	}
	return settings
}

func (a *App) SaveSettings(settings AppSettings) error {
	if a.db == nil {
		return errors.New("database is not ready")
	}
	return a.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		rows := []SettingRecord{
			{Key: "outputPath", Value: settings.OutputPath, UpdatedAt: now},
			{Key: "gamePath", Value: settings.GamePath, UpdatedAt: now},
		}
		for _, row := range rows {
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *App) ListInstalledModRecords() []InstalledModRecord {
	if a.db == nil {
		return []InstalledModRecord{}
	}
	var records []InstalledModRecord
	if err := a.db.Order("installed_at desc").Find(&records).Error; err != nil {
		return []InstalledModRecord{}
	}
	return records
}

func (a *App) IsModInstalled(url string) bool {
	if a.db == nil || strings.TrimSpace(url) == "" {
		return false
	}
	var count int64
	a.db.Model(&InstalledModRecord{}).Where("url = ?", strings.TrimSpace(url)).Count(&count)
	return count > 0
}

func (a *App) MarkModInstalled(url string, title string, thumbnailURL string, path string) error {
	if a.db == nil || strings.TrimSpace(url) == "" {
		return nil
	}
	record := InstalledModRecord{
		URL:          strings.TrimSpace(url),
		Title:        title,
		ThumbnailURL: thumbnailURL,
		Path:         path,
		InstalledAt:  time.Now(),
	}
	return a.db.Save(&record).Error
}

func (a *App) DetectGamePath() string {
	candidates := []string{
		`C:\Program Files (x86)\Steam\steamapps\common\BeamNG.drive`,
		`C:\Program Files\Steam\steamapps\common\BeamNG.drive`,
		`C:\Program Files\Epic Games\BeamNG.drive`,
		`C:\Program Files (x86)\Epic Games\BeamNG.drive`,
	}
	for _, candidate := range candidates {
		if a.ValidateGamePath(candidate) {
			return candidate
		}
	}
	return ""
}

func (a *App) ValidateGamePath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(path, "BeamNG.drive.exe"))
	return err == nil
}

func (qc *QueueController) Reset(url string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	delete(qc.paused, url)
	delete(qc.cancelled, url)
	delete(qc.completed, url)
	qc.cond.Broadcast()
}

func (qc *QueueController) WaitIfPaused(url string) error {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	for qc.paused[url] && !qc.cancelled[url] {
		qc.cond.Wait()
	}
	if qc.cancelled[url] {
		return errors.New("cancelled")
	}
	return nil
}

func (qc *QueueController) IsCancelled(url string) bool {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	return qc.cancelled[url]
}

func (qc *QueueController) Pause(url string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.paused[url] = true
}

func (qc *QueueController) Resume(url string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	delete(qc.paused, url)
	qc.cond.Broadcast()
}

func (qc *QueueController) Cancel(url string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.cancelled[url] = true
	delete(qc.paused, url)
	qc.cond.Broadcast()
}

func (qc *QueueController) Remove(url string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	delete(qc.paused, url)
	delete(qc.cancelled, url)
	delete(qc.completed, url)
	qc.cond.Broadcast()
}

func (qc *QueueController) MarkCompleted(url string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.completed[url] = true
}

func (qc *QueueController) IsCompleted(url string) bool {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	return qc.completed[url]
}

func (qc *QueueController) ClearCompleted() {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.completed = map[string]bool{}
}
