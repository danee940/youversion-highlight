package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const momentsBase = "https://moments.youversionapi.com"

var defaultHeaders = map[string]string{
	"Referer":                   "http://android.youversionapi.com/",
	"X-YouVersion-App-Platform": "android",
	"X-YouVersion-App-Version":  "17114",
	"X-YouVersion-Client":       "youversion",
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

type reference struct {
	Human     string   `json:"human"`
	VersionID int      `json:"version_id"`
	USFM      []string `json:"usfm"`
}

type moment struct {
	KindColor string `json:"kind_color"`
	CreatedDT string `json:"created_dt"`
	Base      struct {
		Title struct {
			Args map[string]string `json:"l_args"`
		} `json:"title"`
	} `json:"base"`
	Extras struct {
		Color      string      `json:"color"`
		References []reference `json:"references"`
	} `json:"extras"`
}

type momentsResponseData struct {
	Moments  []moment `json:"moments"`
	NextPage *int     `json:"next_page"`
}

type momentsResponse struct {
	Response struct {
		Data momentsResponseData `json:"data"`
	} `json:"response"`
}

type enrichedHighlight struct {
	PassageID   string `json:"passage_id"`
	VersionID   int    `json:"version_id"`
	Color       string `json:"color"`
	Reference   string `json:"reference"`
	Translation string `json:"translation"`
	Text        string `json:"text"`
	Date        string `json:"date"`
}

func yvaGet(apiURL string, query url.Values, yva string, target interface{}) error {
	if len(query) > 0 {
		apiURL = fmt.Sprintf("%s?%s", apiURL, query.Encode())
	}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+yva)
	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", apiURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API %s returned %d: %s", apiURL, resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, target)
}

func fetchHighlightsPage(yva, userID string, page int) (*momentsResponse, error) {
	q := url.Values{}
	q.Set("kind", "highlight")
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("user_id", userID)

	var resp momentsResponse
	if err := yvaGet(momentsBase+"/3.1/items.json", q, yva, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func findLastPage(yva, userID string) (int, error) {
	first, err := fetchHighlightsPage(yva, userID, 1)
	if err != nil {
		return 0, err
	}
	if len(first.Response.Data.Moments) == 0 {
		return 0, fmt.Errorf("no highlights found")
	}
	if first.Response.Data.NextPage == nil {
		return 1, nil
	}

	// Exponential probe to find upper bound
	probe := 50
	lastKnown := 1
	for {
		p, err := fetchHighlightsPage(yva, userID, probe)
		if err != nil || len(p.Response.Data.Moments) == 0 {
			break
		}
		lastKnown = probe
		if p.Response.Data.NextPage == nil {
			return probe, nil
		}
		probe *= 2
	}

	// Binary search between lastKnown and probe
	lo, hi := lastKnown, probe
	for lo < hi-1 {
		mid := (lo + hi) / 2
		p, err := fetchHighlightsPage(yva, userID, mid)
		if err != nil || len(p.Response.Data.Moments) == 0 {
			hi = mid
		} else {
			lo = mid
			if p.Response.Data.NextPage == nil {
				return mid, nil
			}
		}
	}
	return lo, nil
}

func fetchRandomHighlight(yva, userID string, lastPage int) (*moment, error) {
	if lastPage < 1 {
		lastPage = 1
	}
	randomPage := 1 + rand.Intn(lastPage)
	page, err := fetchHighlightsPage(yva, userID, randomPage)
	if err != nil || len(page.Response.Data.Moments) == 0 {
		log.Printf("fetchRandomHighlight: page %d failed (%v), falling back to page 1", randomPage, err)
		page, err = fetchHighlightsPage(yva, userID, 1)
		if err != nil {
			return nil, err
		}
	}
	picks := page.Response.Data.Moments
	if len(picks) == 0 {
		return nil, fmt.Errorf("no highlights found")
	}
	picked := picks[rand.Intn(len(picks))]
	return &picked, nil
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, "")
	return strings.TrimSpace(html.UnescapeString(s))
}

type nextDataVerse struct {
	USFM    string
	Content string
}

type nextData struct {
	Props struct {
		PageProps struct {
			ChapterInfo struct {
				Content string `json:"content"`
			} `json:"chapterInfo"`
		} `json:"pageProps"`
	} `json:"props"`
}

var nextDataRe = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.+?)</script>`)
var verseOpenRe = regexp.MustCompile(`<span class="verse[^"]*" data-usfm="([^"]+)"[^>]*>`)
var labelRe = regexp.MustCompile(`(?s)<span class="label">[^<]*`)
var noteOpenRe = regexp.MustCompile(`<span class="note[^"]*"[^>]*>`)
var interVerseBlockRe = regexp.MustCompile(`(?s)<div class="s[^"]*"[^>]*>.*?</div>|<div class="r"[^>]*>.*?</div>`)

func removeNestedSpans(s string, openRe *regexp.Regexp) string {
	var result strings.Builder
	for {
		loc := openRe.FindStringIndex(s)
		if loc == nil {
			result.WriteString(s)
			break
		}
		result.WriteString(s[:loc[0]])
		rest := s[loc[1]:]
		depth := 1
		i := 0
		for i < len(rest) && depth > 0 {
			if strings.HasPrefix(rest[i:], "</span>") {
				depth--
				if depth == 0 {
					i += len("</span>")
					break
				}
				i += len("</span>")
			} else if strings.HasPrefix(rest[i:], "<span") {
				depth++
				i += len("<span")
			} else {
				i++
			}
		}
		s = rest[i:]
	}
	return result.String()
}

func parseVersesFromHTML(content string) []nextDataVerse {
	content = html.UnescapeString(content)
	content = interVerseBlockRe.ReplaceAllString(content, "")
	content = removeNestedSpans(content, noteOpenRe)

	indices := verseOpenRe.FindAllStringSubmatchIndex(content, -1)

	fragsByUSFM := make(map[string][]string)
	var usfmOrder []string

	for i, loc := range indices {
		usfmRaw := content[loc[2]:loc[3]]
		var segment string
		if i+1 < len(indices) {
			segment = content[loc[1]:indices[i+1][0]]
		} else {
			segment = content[loc[1]:]
		}

		inner := labelRe.ReplaceAllString(segment, "")
		text := strings.TrimSpace(stripHTML(inner))
		if text == "" {
			continue
		}
		for _, usfm := range strings.Fields(usfmRaw) {
			if _, seen := fragsByUSFM[usfm]; !seen {
				usfmOrder = append(usfmOrder, usfm)
			}
			fragsByUSFM[usfm] = append(fragsByUSFM[usfm], text)
		}
	}

	verses := make([]nextDataVerse, 0, len(usfmOrder))
	for _, usfm := range usfmOrder {
		verses = append(verses, nextDataVerse{
			USFM:    usfm,
			Content: strings.Join(fragsByUSFM[usfm], " "),
		})
	}
	return verses
}

func fetchPageVerses(yva string, versionID int, usfm string) ([]nextDataVerse, error) {
	pageURL := fmt.Sprintf("https://www.bible.com/bible/%d/%s", versionID, chapterKey(usfm))

	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", "yva="+yva)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching bible page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	match := nextDataRe.FindSubmatch(body)
	if match == nil {
		return nil, fmt.Errorf("__NEXT_DATA__ not found in page")
	}

	var nd nextData
	if err := json.Unmarshal(match[1], &nd); err != nil {
		return nil, fmt.Errorf("parsing __NEXT_DATA__: %w", err)
	}

	verses := parseVersesFromHTML(nd.Props.PageProps.ChapterInfo.Content)
	return verses, nil
}

func chapterKey(usfm string) string {
	parts := strings.Split(usfm, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return usfm
}

func fetchVerseText(yva string, versionID int, usfms []string) (string, string, error) {
	if len(usfms) == 0 {
		return "", "", fmt.Errorf("no USFM")
	}

	usfmSet := make(map[string]bool, len(usfms))
	for _, u := range usfms {
		usfmSet[u] = true
	}

	seenChapters := make(map[string]bool)
	var chapterUSFMs []string
	for _, u := range usfms {
		key := chapterKey(u)
		if !seenChapters[key] {
			seenChapters[key] = true
			chapterUSFMs = append(chapterUSFMs, u)
		}
	}

	var textParts []string

	for _, chapterUSFM := range chapterUSFMs {
		verses, err := fetchPageVerses(yva, versionID, chapterUSFM)
		if err != nil {
			return "", "", err
		}
		for _, v := range verses {
			if usfmSet[v.USFM] {
				textParts = append(textParts, v.Content)
			}
		}
	}

	if len(textParts) > 0 {
		return "", strings.Join(textParts, " "), nil
	}
	return "", "", fmt.Errorf("verse not found in page")
}

func enrichHighlight(yva, userID string, lastPage int) (*enrichedHighlight, error) {
	m, err := fetchRandomHighlight(yva, userID, lastPage)
	if err != nil {
		return nil, err
	}

	if len(m.Extras.References) == 0 {
		return nil, fmt.Errorf("highlight has no references")
	}
	ref := m.Extras.References[0]

	var allUSFMs []string
	for _, r := range m.Extras.References {
		allUSFMs = append(allUSFMs, r.USFM...)
	}
	usfm := ""
	if len(allUSFMs) > 0 {
		usfm = allUSFMs[0]
	}

	humanRef := ref.Human
	translation := ""
	if args := m.Base.Title.Args; args != nil {
		if r, ok := args["reference_human"]; ok && r != "" {
			humanRef = r
		}
		if v, ok := args["version_abbreviation"]; ok {
			translation = v
		}
	}

	color := m.Extras.Color
	if color == "" {
		color = m.KindColor
	}

	_, text, err := fetchVerseText(yva, ref.VersionID, allUSFMs)
	if err != nil {
		text = ""
	}

	date := ""
	if t, err := time.Parse(time.RFC3339, m.CreatedDT); err == nil {
		date = t.Format("January 2, 2006")
	}

	return &enrichedHighlight{
		PassageID:   usfm,
		VersionID:   ref.VersionID,
		Color:       color,
		Reference:   humanRef,
		Translation: translation,
		Text:        text,
		Date:        date,
	}, nil
}
