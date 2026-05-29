package main

import (
	"strings"
	"testing"
	"time"
)

func TestStripHTML_removesTagsAndUnescapesEntities(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"<b>Hello</b>", "Hello"},
		{"<span class=\"v\">In the beginning</span>", "In the beginning"},
		{"God &amp; man", "God & man"},
		{"<p>Line &quot;one&quot;</p>", `Line "one"`},
		{"  <br/>  spaced  ", "spaced"},
		{"no tags here", "no tags here"},
		{"", ""},
		{"<span><span>nested</span></span>", "nested"},
	}
	for _, tt := range tests {
		got := stripHTML(tt.input)
		if got != tt.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChapterKey_threePartUSFM(t *testing.T) {
	got := chapterKey("JHN.3.16")
	if got != "JHN.3" {
		t.Errorf("chapterKey(JHN.3.16) = %q, want %q", got, "JHN.3")
	}
}

func TestChapterKey_twoPartUSFM(t *testing.T) {
	got := chapterKey("PSA.23")
	if got != "PSA.23" {
		t.Errorf("chapterKey(PSA.23) = %q, want %q", got, "PSA.23")
	}
}

func TestChapterKey_singlePart(t *testing.T) {
	got := chapterKey("GEN")
	if got != "GEN" {
		t.Errorf("chapterKey(GEN) = %q, want %q", got, "GEN")
	}
}

func TestChapterKey_multiDigitChapter(t *testing.T) {
	got := chapterKey("REV.22.20")
	if got != "REV.22" {
		t.Errorf("chapterKey(REV.22.20) = %q, want %q", got, "REV.22")
	}
}

func TestParseVersesFromHTML_singleVerse(t *testing.T) {
	content := `<span class="verse" data-usfm="JHN.3.16"><span class="label">16</span> For God so loved the world</span>`
	verses := parseVersesFromHTML(content)
	if len(verses) != 1 {
		t.Fatalf("expected 1 verse, got %d", len(verses))
	}
	if verses[0].USFM != "JHN.3.16" {
		t.Errorf("USFM = %q, want %q", verses[0].USFM, "JHN.3.16")
	}
	if !strings.Contains(verses[0].Content, "For God so loved") {
		t.Errorf("Content %q does not contain expected text", verses[0].Content)
	}
}

func TestParseVersesFromHTML_multipleVerses(t *testing.T) {
	content := `<span class="verse" data-usfm="PSA.23.1"><span class="label">1</span> The Lord is my shepherd</span>` +
		`<span class="verse" data-usfm="PSA.23.2"><span class="label">2</span> He makes me lie down</span>`
	verses := parseVersesFromHTML(content)
	if len(verses) != 2 {
		t.Fatalf("expected 2 verses, got %d", len(verses))
	}
	if verses[0].USFM != "PSA.23.1" {
		t.Errorf("first verse USFM = %q, want %q", verses[0].USFM, "PSA.23.1")
	}
	if verses[1].USFM != "PSA.23.2" {
		t.Errorf("second verse USFM = %q, want %q", verses[1].USFM, "PSA.23.2")
	}
}

func TestParseVersesFromHTML_emptyContent(t *testing.T) {
	verses := parseVersesFromHTML("")
	if len(verses) != 0 {
		t.Errorf("expected 0 verses, got %d", len(verses))
	}
}

func TestParseVersesFromHTML_skipsBlankVerses(t *testing.T) {
	content := `<span class="verse" data-usfm="GEN.1.1"><span class="label">1</span>   </span>` +
		`<span class="verse" data-usfm="GEN.1.2"><span class="label">2</span> Now the earth was formless</span>`
	verses := parseVersesFromHTML(content)
	if len(verses) != 1 {
		t.Fatalf("expected 1 verse (blank filtered), got %d", len(verses))
	}
	if verses[0].USFM != "GEN.1.2" {
		t.Errorf("USFM = %q, want %q", verses[0].USFM, "GEN.1.2")
	}
}

func TestParseVersesFromHTML_htmlEntitiesDecoded(t *testing.T) {
	content := `<span class="verse" data-usfm="GEN.1.1"><span class="label">1</span> God &amp; creation</span>`
	verses := parseVersesFromHTML(content)
	if len(verses) != 1 {
		t.Fatalf("expected 1 verse, got %d", len(verses))
	}
	if !strings.Contains(verses[0].Content, "God & creation") {
		t.Errorf("Content %q does not contain decoded entity", verses[0].Content)
	}
}

func TestParseVersesFromHTML_stripsNoteSpans(t *testing.T) {
	content := `<span class="verse v1" data-usfm="HEB.12.1"><span class="label">1</span><span class="content">Main text</span><span class="note x"><span class="label">#</span><span class=" body">1Cor 9:24; Phil 3:12</span></span><span class="content">  </span></span>`
	verses := parseVersesFromHTML(content)
	if len(verses) != 1 {
		t.Fatalf("expected 1 verse, got %d", len(verses))
	}
	if strings.Contains(verses[0].Content, "1Cor") || strings.Contains(verses[0].Content, "Phil") {
		t.Errorf("Content %q still contains cross-reference text", verses[0].Content)
	}
	if !strings.Contains(verses[0].Content, "Main text") {
		t.Errorf("Content %q missing expected verse text", verses[0].Content)
	}
}

func TestParseVersesFromHTML_stripsSectionHeadings(t *testing.T) {
	content := `<div class="s"><span class="heading">Section Title</span></div>` +
		`<span class="verse v1" data-usfm="GEN.1.1"><span class="label">1</span><span class="content">In the beginning</span></span>`
	verses := parseVersesFromHTML(content)
	if len(verses) != 1 {
		t.Fatalf("expected 1 verse, got %d", len(verses))
	}
	if strings.Contains(verses[0].Content, "Section Title") {
		t.Errorf("Content %q contains section heading text", verses[0].Content)
	}
}

func TestParseVersesFromHTML_stripsParallelRefs(t *testing.T) {
	content := `<span class="verse v31" data-usfm="MRK.10.31"><span class="label">31</span><span class="content">Many who are first will be last</span></span>` +
		`</div><div class="s"><span class="heading">Next Section</span></div><div class="r"><span class="heading">(Mt 20,17–19; Lk 18,31–34)</span></div><div class="p">` +
		`<span class="verse v32" data-usfm="MRK.10.32"><span class="label">32</span><span class="content">They were on the way</span></span>`
	verses := parseVersesFromHTML(content)
	var v31 *nextDataVerse
	for i := range verses {
		if verses[i].USFM == "MRK.10.31" {
			v31 = &verses[i]
		}
	}
	if v31 == nil {
		t.Fatal("MRK.10.31 not found in parsed verses")
	}
	if strings.Contains(v31.Content, "Mt 20") || strings.Contains(v31.Content, "Lk 18") {
		t.Errorf("verse 31 content %q contains parallel reference text", v31.Content)
	}
	if !strings.Contains(v31.Content, "Many who are first") {
		t.Errorf("verse 31 content %q missing expected text", v31.Content)
	}
}

func TestRemoveNestedSpans_removesMatchingSpan(t *testing.T) {
	input := `before<span class="note x"><span class="label">#</span><span class="body">ref</span></span>after`
	got := removeNestedSpans(input, noteOpenRe)
	if strings.Contains(got, "ref") || strings.Contains(got, "label") {
		t.Errorf("removeNestedSpans still contains note content: %q", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("removeNestedSpans removed surrounding content: %q", got)
	}
}

func TestRemoveNestedSpans_noMatch(t *testing.T) {
	input := `<span class="content">hello</span>`
	got := removeNestedSpans(input, noteOpenRe)
	if got != input {
		t.Errorf("removeNestedSpans modified non-matching input: %q", got)
	}
}

func TestTokenState_setAndGet(t *testing.T) {
	s := &tokenState{}
	exp := time.Now().Add(time.Hour)
	s.set("mytoken", "user42", exp)

	yva, userID, gotExp := s.get()
	if yva != "mytoken" {
		t.Errorf("yva = %q, want %q", yva, "mytoken")
	}
	if userID != "user42" {
		t.Errorf("userID = %q, want %q", userID, "user42")
	}
	if gotExp.Unix() != exp.Unix() {
		t.Errorf("exp = %v, want %v", gotExp, exp)
	}
}

func TestTokenState_setResetsLastPage(t *testing.T) {
	s := &tokenState{}
	s.setLastPage(99)
	s.set("tok", "u", time.Now())
	if p := s.getLastPage(); p != 0 {
		t.Errorf("lastPage after set() = %d, want 0", p)
	}
}

func TestTokenState_lastPage(t *testing.T) {
	s := &tokenState{}
	if s.getLastPage() != 0 {
		t.Error("initial lastPage should be 0")
	}
	s.setLastPage(42)
	if s.getLastPage() != 42 {
		t.Errorf("lastPage = %d, want 42", s.getLastPage())
	}
}
