package catalog

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	WorldOfModsRu  = "https://www.worldofmods.ru"
	WorldOfModsCom = "https://www.worldofmods.com"
	MaxCarsPage    = 320
)

type ModCard struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	ImageURL  string `json:"imageUrl"`
	Category  string `json:"category"`
	Author    string `json:"author"`
	Published string `json:"published"`
	Views     string `json:"views"`
	Downloads string `json:"downloads"`
	Size      string `json:"size"`
	Page      int    `json:"page"`
}

type SearchResult struct {
	Mods     []ModCard `json:"mods"`
	NextPage int       `json:"nextPage"`
	HasMore  bool      `json:"hasMore"`
}

type ModDetails struct {
	ModCard
	Description string   `json:"description"`
	Screenshots []string `json:"screenshots"`
}

func FetchCarsPage(page int) ([]ModCard, error) {
	if page < 1 {
		page = 1
	}
	if page > MaxCarsPage {
		return []ModCard{}, nil
	}

	mods, err := fetchPage(page)
	if err != nil {
		return nil, err
	}
	enrichModSizes(mods)
	return mods, nil
}

func SearchCars(query string, startPage int, limit int) (SearchResult, error) {
	if startPage < 1 {
		startPage = MaxCarsPage
	}
	if startPage > MaxCarsPage {
		startPage = MaxCarsPage
	}
	if limit <= 0 {
		limit = 24
	}

	query = strings.ToLower(strings.TrimSpace(query))
	result := SearchResult{
		Mods:     []ModCard{},
		NextPage: startPage,
		HasMore:  startPage >= 1,
	}

	for page := startPage; page >= 1 && len(result.Mods) < limit; page-- {
		mods, err := fetchPage(page)
		if err != nil {
			return result, err
		}

		for _, mod := range mods {
			if query == "" || strings.Contains(strings.ToLower(mod.Title), query) {
				result.Mods = append(result.Mods, mod)
				if len(result.Mods) >= limit {
					break
				}
			}
		}

		result.NextPage = page - 1
		result.HasMore = result.NextPage >= 1
	}

	if result.NextPage < 1 {
		result.HasMore = false
	}

	enrichModSizes(result.Mods)
	return result, nil
}

func FetchModDetails(modURL string) (ModDetails, error) {
	doc, err := getDocument(modURL)
	if err != nil {
		return ModDetails{}, err
	}

	title := compactSpaces(doc.Find("h1.overflow-gradient").First().Text())
	title = strings.ReplaceAll(title, " for BeamNG Drive", "")
	title = strings.ReplaceAll(title, "для BeamNG Drive", "")

	details := ModDetails{
		ModCard: ModCard{
			ID:    fallbackID("", modURL),
			Title: title,
			URL:   modURL,
		},
		Screenshots: []string{},
	}

	doc.Find("section.description, article .description, .content-text").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := compactSpaces(s.Text())
		if text != "" {
			details.Description = text
			return false
		}
		return true
	})
	if details.Description == "" {
		doc.Find("article.content-block > p, article.content-block p, .content-block p").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			text := compactSpaces(s.Text())
			if text != "" {
				details.Description = text
				return false
			}
			return true
		})
	}

	if details.Description == "" {
		details.Description = compactSpaces(doc.Find("meta[name='description']").AttrOr("content", ""))
	}

	if details.Description == "" {
		details.Description = "Описание не найдено, но мод можно установить прямо отсюда."
	}

	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}
		if strings.Contains(src, "/large-fit/") || strings.Contains(src, "/wide/") {
			details.Screenshots = append(details.Screenshots, absoluteURL(WorldOfModsRu, src))
		}
	})

	if len(details.Screenshots) > 0 {
		details.ImageURL = details.Screenshots[0]
	}

	counters := doc.Find(".counters .col-sm-4")
	counters.Each(func(_ int, s *goquery.Selection) {
		text := compactSpaces(s.Text())
		lower := strings.ToLower(text)
		if strings.Contains(lower, "просмотров") {
			details.Views = strings.TrimSpace(strings.TrimPrefix(text, "Просмотров:"))
		}
		if strings.Contains(lower, "скачиваний") {
			details.Downloads = strings.TrimSpace(strings.TrimPrefix(text, "Скачиваний:"))
		}
	})

	details.Author = compactSpaces(doc.Find("[rel='author']").First().Text())
	details.Published = compactSpaces(doc.Find("time").First().Text())

	size, err := fetchModSize(modURL)
	if err == nil {
		details.Size = size
	}

	return details, nil
}

func fetchPage(page int) ([]ModCard, error) {
	doc, err := getDocument(carsPageURL(page))
	if err != nil {
		return nil, err
	}

	mods := make([]ModCard, 0, 16)
	doc.Find("article.catalog-content").Each(func(_ int, card *goquery.Selection) {
		titleLink := card.Find("header h3 a").First()
		media := card.Find("a.catalog-content-media").First()
		image := media.Find("img").First()
		href, _ := titleLink.Attr("href")
		imageURL, _ := image.Attr("src")
		id, _ := card.Find("button[data-content-id]").First().Attr("data-content-id")

		counters := card.Find(".counter span")
		views := strings.TrimSpace(counters.Eq(0).Text())
		downloads := strings.TrimSpace(counters.Eq(1).Text())
		category := strings.TrimSpace(card.Find("footer .overflow-gradient").First().Text())
		category = compactSpaces(category)

		mod := ModCard{
			ID:        fallbackID(id, href),
			Title:     compactSpaces(titleLink.Text()),
			URL:       absoluteURL(WorldOfModsCom, href),
			ImageURL:  absoluteURL(WorldOfModsRu, imageURL),
			Category:  category,
			Author:    compactSpaces(card.Find("address.author a").First().Text()),
			Published: compactSpaces(card.Find("time").First().Text()),
			Views:     views,
			Downloads: downloads,
			Page:      page,
		}

		if mod.Title != "" && mod.URL != "" {
			mods = append(mods, mod)
		}
	})

	return mods, nil
}

func getDocument(pageURL string) (*goquery.Document, error) {
	client := http.Client{Timeout: 25 * time.Second}
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "BeamNGWOWD/1.0 (+https://www.worldofmods.ru)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("worldofmods returned %s for %s", resp.Status, pageURL)
	}

	return goquery.NewDocumentFromReader(resp.Body)
}

func enrichModSizes(mods []ModCard) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	for i := range mods {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			size, err := fetchModSize(mods[index].URL)
			if err == nil {
				mods[index].Size = size
			}
		}(i)
	}

	wg.Wait()
}

func fetchModSize(modURL string) (string, error) {
	detailDoc, err := getDocument(modURL)
	if err != nil {
		return "", err
	}

	manualURL := ""
	detailDoc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := strings.ToLower(s.Text())
		if strings.Contains(text, "zip") {
			href, exists := s.Attr("href")
			if exists {
				manualURL = absoluteURL(WorldOfModsCom, href)
				return false
			}
		}
		return true
	})
	if manualURL == "" {
		return "", fmt.Errorf("manual download link not found")
	}

	if !strings.Contains(manualURL, "?") {
		manualURL += "?ajax=true"
	}

	downloadDoc, err := getDocument(manualURL)
	if err != nil {
		return "", err
	}

	downloadURL := ""
	downloadDoc.Find("a.btn.btn-success").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		href, exists := s.Attr("href")
		if exists {
			downloadURL = absoluteURL(WorldOfModsCom, href)
			return false
		}
		return true
	})
	if downloadURL == "" {
		return "", fmt.Errorf("download link not found")
	}

	return probeDownloadSize(downloadURL)
}

func probeDownloadSize(downloadURL string) (string, error) {
	client := http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodHead, downloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "BeamNGWOWD/1.0 (+https://www.worldofmods.ru)")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.ContentLength <= 0 {
		return "", fmt.Errorf("content length missing")
	}

	return formatBytes(resp.ContentLength), nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}

	value := float64(bytes)
	units := []string{"KB", "MB", "GB"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}

	return fmt.Sprintf("%.1f TB", value/unit)
}

func carsPageURL(page int) string {
	if page <= 1 {
		return WorldOfModsRu + "/beamng/cars/"
	}
	return WorldOfModsRu + "/beamng/cars/page:" + strconv.Itoa(page) + "/"
}

func absoluteURL(baseURL string, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.IsAbs() {
		return raw
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return raw
	}
	return base.ResolveReference(parsed).String()
}

func compactSpaces(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func fallbackID(id string, href string) string {
	if strings.TrimSpace(id) != "" {
		return strings.TrimSpace(id)
	}
	href = strings.Trim(strings.TrimSpace(href), "/")
	parts := strings.Split(href, "/")
	if len(parts) == 0 {
		return href
	}
	return parts[len(parts)-1]
}
