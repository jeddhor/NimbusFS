// Package mimetypes maps file extensions to human-readable type names
// (e.g. "py" -> "Python script") by reading the freedesktop.org
// shared-mime-info database — the same source GNOME, KDE, and XFCE use to
// label files — rather than maintaining our own hand-written list.
package mimetypes

import (
	"encoding/xml"
	"log"
	"os"
	"strconv"
	"strings"
)

// candidatePaths are the standard install locations of the shared-mime-info
// package's core database across Linux distributions.
var candidatePaths = []string{
	"/usr/share/mime/packages/freedesktop.org.xml",
	"/usr/local/share/mime/packages/freedesktop.org.xml",
}

type mimeInfoXML struct {
	Types []mimeTypeXML `xml:"mime-type"`
}

type mimeTypeXML struct {
	Globs    []globXML    `xml:"glob"`
	Comments []commentXML `xml:"comment"`
}

type globXML struct {
	Pattern       string `xml:"pattern,attr"`
	Weight        string `xml:"weight,attr"`
	CaseSensitive string `xml:"case-sensitive,attr"`
}

type commentXML struct {
	Lang  string `xml:"lang,attr"`
	Value string `xml:",chardata"`
}

// Load builds an extension (lowercase, no leading dot) -> human-readable
// type name map from the system's shared-mime-info database. It returns an
// empty, non-nil map if the database isn't installed, so callers can fall
// back to a generic label rather than failing.
func Load() map[string]string {
	for _, path := range candidatePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		names, err := parse(data)
		if err != nil {
			log.Printf("mimetypes: parsing %s: %v", path, err)
			continue
		}
		return names
	}
	log.Printf("mimetypes: no shared-mime-info database found (expected shared-mime-info package); file type names will be generic")
	return map[string]string{}
}

type candidate struct {
	weight int
	name   string
}

// parse extracts an extension -> description mapping from a shared-mime-info
// package XML document, using each mime-type's unlocalized <comment> and its
// single-extension <glob> patterns (e.g. "*.py"). Globs covering multiple
// extensions or bare filenames (e.g. "*.tar.gz", "Makefile") are skipped, to
// match how the rest of nimbusfs derives a file's extension. When more than
// one mime-type claims the same extension, the entry with the higher glob
// weight wins (shared-mime-info's own disambiguation signal); ties keep
// whichever was encountered first.
func parse(data []byte) (map[string]string, error) {
	var doc mimeInfoXML
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	best := map[string]candidate{}
	for _, t := range doc.Types {
		name := defaultComment(t.Comments)
		if name == "" {
			continue
		}
		for _, g := range t.Globs {
			ext, ok := singleExtension(g.Pattern, g.CaseSensitive == "true")
			if !ok {
				continue
			}
			weight := 50 // shared-mime-info's documented default glob weight
			if g.Weight != "" {
				if w, err := strconv.Atoi(g.Weight); err == nil {
					weight = w
				}
			}
			if cur, exists := best[ext]; !exists || weight > cur.weight {
				best[ext] = candidate{weight: weight, name: name}
			}
		}
	}

	names := make(map[string]string, len(best))
	for ext, c := range best {
		names[ext] = c.name
	}
	return names, nil
}

func defaultComment(comments []commentXML) string {
	for _, c := range comments {
		if c.Lang == "" {
			return strings.TrimSpace(c.Value)
		}
	}
	return ""
}

// singleExtension recognizes only simple "*.ext" glob patterns and returns
// the lowercased extension. Case-sensitive patterns whose extension contains
// an uppercase letter (e.g. "*.C" distinguishing C++ from C's "*.c") are
// rejected, since they'd never match the lowercased extension we look up.
func singleExtension(pattern string, caseSensitive bool) (string, bool) {
	if !strings.HasPrefix(pattern, "*.") {
		return "", false
	}
	ext := pattern[2:]
	if ext == "" || strings.ContainsAny(ext, "*?[].") {
		return "", false
	}
	if caseSensitive && ext != strings.ToLower(ext) {
		return "", false
	}
	return strings.ToLower(ext), true
}
