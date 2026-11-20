package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestBuildRendersEveryLanguage(t *testing.T) {
	out := t.TempDir()
	if err := build("..", out); err != nil {
		t.Fatal(err)
	}
	for _, l := range langs {
		b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(l.Path), "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		html := string(b)
		for _, want := range []string{
			`<html lang="` + l.Code + `">`,
			`<link rel="canonical" href="` + l.URL + `">`,
			`<meta property="og:locale" content="` + l.OGLocale + `">`,
			`hreflang="x-default"`,
		} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: missing %s", l.Code, want)
			}
		}
		if n := strings.Count(html, `<link rel="alternate" hreflang=`); n != len(langs)+1 {
			t.Errorf("%s: %d hreflang links, want %d", l.Code, n, len(langs)+1)
		}
		// Claude Code runs a pasted block as one command, so every copy button must hold exactly one.
		copies := dataCopy.FindAllStringSubmatch(html, -1)
		if len(copies) == 0 {
			t.Errorf("%s: no copy buttons", l.Code)
		}
		for _, m := range copies {
			if strings.Contains(m[1], "&#10;") || strings.Contains(m[1], "\n") {
				t.Errorf("%s: copy button holds more than one line: %q", l.Code, m[1])
			}
		}
	}
	sm, err := os.ReadFile(filepath.Join(out, "sitemap.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal(sm, new(struct{})); err != nil {
		t.Fatalf("sitemap.xml is not valid XML: %v", err)
	}
	for _, f := range []string{"robots.txt", "404.html", "og.png", "favicon.svg", "apple-touch-icon.png"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("public file not copied: %v", err)
		}
	}
}

var (
	codeTag  = regexp.MustCompile(`<code>`)
	dataCopy = regexp.MustCompile(`<button class="copy" data-copy="([^"]*)"`)
	href     = regexp.MustCompile(`href="([^"]*)"`)
)

// Translations must keep the inline markup of the English source: the same number of <code> spans and the same link targets, so a translator cannot drop a command name or break a link.
func TestTranslationsKeepMarkup(t *testing.T) {
	all, err := loadStrings(filepath.Join("..", "i18n"))
	if err != nil {
		t.Fatal(err)
	}
	base := all[langs[0].Code]
	for _, l := range langs[1:] {
		for k, en := range base {
			tr := all[l.Code][k]
			if a, b := len(codeTag.FindAllString(en, -1)), len(codeTag.FindAllString(tr, -1)); a != b {
				t.Errorf("%s %s: %d <code> spans, want %d", l.Code, k, b, a)
			}
			var enLinks, trLinks []string
			for _, m := range href.FindAllStringSubmatch(en, -1) {
				enLinks = append(enLinks, m[1])
			}
			for _, m := range href.FindAllStringSubmatch(tr, -1) {
				trLinks = append(trLinks, m[1])
			}
			if !slices.Equal(enLinks, trLinks) {
				t.Errorf("%s %s: links %v, want %v", l.Code, k, trLinks, enLinks)
			}
		}
	}
}
