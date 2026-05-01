package downloader

import (
	"BeamNGMode-Wails/service/widgets"
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/wailsapp/wails/v2/pkg/logger"
)

type Mod struct {
	Name        *string
	PathToFile  *string
	URL         string
	DownloadURL *string
	Widget      *widgets.ProgressWidget
	Cancelled   bool
	Error       error
}

type ModController interface {
	WaitIfPaused(url string) error
	IsCancelled(url string) bool
	MarkCompleted(url string)
}

type ModsProcessor struct {
	Ctx             context.Context
	URLs            []string
	QueueMeta       map[string]QueueMeta
	PathToMods      string
	Logger          *logger.Logger
	MetadataWorkers int
	DownloadWorkers int
	ZipWorkers      int
	Controller      ModController
}

type QueueMeta struct {
	Title        string
	ThumbnailURL string
}

func NewModsProcessor(urls []string, path string, metaWorkers, downloadWorkers, zipWorkers int, context context.Context) *ModsProcessor {
	return &ModsProcessor{
		URLs:            urls,
		PathToMods:      path + "/BeamNGWOWD",
		MetadataWorkers: metaWorkers,
		DownloadWorkers: downloadWorkers,
		ZipWorkers:      zipWorkers,
		Ctx:             context,
	}
}

func (mp *ModsProcessor) SetController(controller ModController) {
	mp.Controller = controller
}

func (mp *ModsProcessor) SetQueueMeta(meta map[string]QueueMeta) {
	mp.QueueMeta = meta
}

func (mp *ModsProcessor) ProcessMods() {
	metaChan := make(chan Mod)
	downloadChan := make(chan Mod)
	unpackChan := make(chan Mod)
	packChan := make(chan Mod)

	var metaWg sync.WaitGroup
	var downloadWg sync.WaitGroup
	var unpackWg sync.WaitGroup
	var packWg sync.WaitGroup

	for i := 0; i < mp.MetadataWorkers; i++ {
		metaWg.Add(1)
		go func() {
			defer metaWg.Done()
			for m := range metaChan {
				mp.processMetadata(&m)
				if !m.Cancelled && m.Error == nil {
					downloadChan <- m
				}
			}
		}()
	}

	for i := 0; i < mp.DownloadWorkers; i++ {
		downloadWg.Add(1)
		go func() {
			defer downloadWg.Done()
			for m := range downloadChan {
				mp.processDownload(&m)
				if !m.Cancelled && m.Error == nil {
					unpackChan <- m
				}
			}
		}()
	}

	for i := 0; i < mp.ZipWorkers; i++ {
		unpackWg.Add(1)
		go func() {
			defer unpackWg.Done()
			for m := range unpackChan {
				mp.processUnpack(&m)
				if !m.Cancelled && m.Error == nil {
					packChan <- m
				}
			}
		}()
	}

	for i := 0; i < mp.ZipWorkers; i++ {
		packWg.Add(1)
		go func() {
			defer packWg.Done()
			for m := range packChan {
				mp.processPack(&m)
			}
		}()
	}

	for _, url := range mp.URLs {
		url = strings.TrimSpace(url)
		widget := widgets.NewProgressWidget(mp.Ctx, url)
		if meta, exists := mp.QueueMeta[url]; exists {
			if strings.TrimSpace(meta.Title) != "" {
				widget.SetTitle(meta.Title)
			}
			if strings.TrimSpace(meta.ThumbnailURL) != "" {
				widget.SetThumb(meta.ThumbnailURL)
			}
		}
		metaChan <- Mod{
			Name:        nil,
			PathToFile:  nil,
			DownloadURL: nil,
			URL:         url,
			Widget:      widget,
		}
	}

	close(metaChan)
	metaWg.Wait()
	close(downloadChan)
	downloadWg.Wait()
	close(unpackChan)
	unpackWg.Wait()
	close(packChan)
	packWg.Wait()
	mp.CleanUp()
}

func (mp *ModsProcessor) processMetadata(m *Mod) {
	if mp.shouldStop(m) {
		return
	}

	// mp.ProgressBarsBox.Add(m.Widget.Container)

	m.Widget.SetStatus("Получение метаданных...")
	html := mp.getModPageHtml(m.URL)
	if mp.shouldStop(m) {
		return
	}
	m.Name = mp.getModName(html)
	m.Widget.SetTitle(*m.Name)
	downloadPageUrl := mp.getDownloadPageUrl(html)

	mp.getModPageHtml(downloadPageUrl).Find("a.btn.btn-success").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			m.DownloadURL = &href
		}
	})
	if m.DownloadURL == nil {
		m.Error = errors.New("download link not found")
		m.Widget.SetStatus("Ошибка: ссылка загрузки не найдена")
	}

	// TODO: загрузка мода
}

func (mp *ModsProcessor) getModPageHtml(url string) *goquery.Document {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		panic(err)
	}

	return doc
}

func (mp *ModsProcessor) getModName(html *goquery.Document) *string {
	var modName string

	html.Find("h1.overflow-gradient").Each(func(i int, s *goquery.Selection) {
		modName = strings.ReplaceAll(s.Text(), " for BeamNG Drive", "")
		modName = strings.ReplaceAll(modName, "для BeamNG Drive", "")

	})

	return &modName
}

func (mp *ModsProcessor) getDownloadPageUrl(html *goquery.Document) string {
	var zipHref string

	html.Find("a").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(strings.ToLower(s.Text()), "zip") {
			href, exists := s.Attr("href")
			if exists {
				zipHref = href
				return // досрочно выйти не получится, просто перезаписывает
			}
		}
	})

	return "https://www.worldofmods.com/" + zipHref + "?ajax=true"
}

func (mp *ModsProcessor) processDownload(m *Mod) {
	if mp.shouldStop(m) {
		return
	}
	m.Widget.SetStatus("Загрузка...")

	resp, err := http.Get(*m.DownloadURL)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	pathToFile := fmt.Sprintf("%s/%s", mp.PathToMods, *m.Name+".zip")
	m.PathToFile = &pathToFile
	err = os.MkdirAll(filepath.Dir(pathToFile), os.ModePerm)
	if err != nil {
		panic(err)
	}
	file, err := os.Create(pathToFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	total := resp.ContentLength
	var downloaded int64 = 0

	buf := make([]byte, 1024)
	for {
		if mp.shouldStop(m) {
			return
		}
		nr, err := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := file.Write(buf[0:nr])
			if nw > 0 {
				downloaded += int64(nw)
				if total > 0 {
					progress := float64(downloaded) / float64(total)
					m.Widget.SetProgress(progress) // обновляем UI
				}
				m.Widget.SetProgressBytes(downloaded, total)
			}
			if ew != nil {
				break
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
	}

}
func (mp *ModsProcessor) processUnpack(m *Mod) {
	if mp.shouldStop(m) {
		return
	}
	m.Widget.SetStatus("Распаковка...")

	pathToZip := *m.PathToFile

	if !strings.HasSuffix(pathToZip, ".zip") || strings.HasPrefix(filepath.Base(pathToZip), "processed-") {
		return
	}

	r, err := zip.OpenReader(pathToZip)
	if err != nil {
		panic(err)
	}
	defer r.Close()

	extractPath := filepath.Join(filepath.Dir(pathToZip), strings.TrimSuffix(filepath.Base(pathToZip), ".zip"))

	totalFiles := len(r.File)
	if totalFiles == 0 {
		m.Widget.SetProgress(1)
		return
	}

	for i, f := range r.File {
		if mp.shouldStop(m) {
			return
		}
		cleanPath := filepath.FromSlash(strings.ReplaceAll(f.Name, "\\", "/"))
		fpath := filepath.Join(extractPath, cleanPath)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err == nil {
				dstFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
				if err == nil {
					fileInArchive, err := f.Open()
					if err == nil {
						io.Copy(dstFile, fileInArchive)
						fileInArchive.Close()
					}
					dstFile.Close()
				}
			}
		}

		progress := float64(i+1) / float64(totalFiles)
		m.Widget.SetProgress(progress)
	}
}

func (mp *ModsProcessor) processPack(m *Mod) {
	if mp.shouldStop(m) {
		return
	}
	m.Widget.SetStatus("Упаковка...")

	baseFolder := filepath.Join(mp.PathToMods, *m.Name, "00 - Copy to folder in My documents", "unpacked")

	entries, err := os.ReadDir(baseFolder)
	if err != nil || len(entries) == 0 {
		panic(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			baseFolder = filepath.Join(baseFolder, e.Name())
			break
		}
	}

	// Считаем количество файлов для прогресса
	var files []string
	err = filepath.Walk(baseFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	archivePath := filepath.Join(mp.PathToMods, "processed-"+*m.Name+".zip")
	zipFile, err := os.Create(archivePath)
	if err != nil {
		panic(err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for i, path := range files {
		if mp.shouldStop(m) {
			return
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		relPath, err := filepath.Rel(baseFolder, path)
		if err != nil {
			continue
		}

		fileIn, err := os.Open(path)
		if err != nil {
			continue
		}

		fh, err := zip.FileInfoHeader(info)
		if err != nil {
			fileIn.Close()
			continue
		}
		fh.Name = relPath
		fh.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(fh)
		if err != nil {
			fileIn.Close()
			continue
		}

		_, err = io.Copy(writer, fileIn)
		fileIn.Close()
		if err != nil {
			continue
		}

		progress := float64(i+1) / float64(len(files))
		m.Widget.SetProgress(progress)
	}

	m.Widget.SetStatus("Завершено.")
	if mp.Controller != nil {
		mp.Controller.MarkCompleted(m.URL)
	}
}

func (mp *ModsProcessor) shouldStop(m *Mod) bool {
	if mp.Controller == nil {
		return false
	}
	if mp.Controller.IsCancelled(m.URL) {
		m.Cancelled = true
		m.Widget.SetStatus("Отменено")
		return true
	}
	if err := mp.Controller.WaitIfPaused(m.URL); err != nil {
		m.Cancelled = true
		m.Error = err
		m.Widget.SetStatus("Отменено")
		return true
	}
	return false
}

func (mp *ModsProcessor) CleanUp() {
	fmt.Println("🧹 Очистка папки с модами...")

	files, err := os.ReadDir(mp.PathToMods)
	if err != nil {
		panic(err)
	}

	for _, entry := range files {
		name := entry.Name()
		fullPath := filepath.Join(mp.PathToMods, name)

		if strings.HasPrefix(name, "processed-") {
			continue
		}

		err := removeAllWithRetry(fullPath, 5, 250*time.Millisecond)
		if err != nil {
			fmt.Println("⚠️ Не удалось удалить временный файл:", fullPath, err)
		} else {
			fmt.Println("🧹 Удален файл:", fullPath)
		}
	}
}

func removeAllWithRetry(path string, attempts int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := os.RemoveAll(path); err == nil || os.IsNotExist(err) {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(delay)
	}
	return lastErr
}
