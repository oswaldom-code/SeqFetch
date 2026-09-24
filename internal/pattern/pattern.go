// Package pattern parses a URL template containing a single sequential
// placeholder and generates concrete URLs for a given index.
//
// Supported placeholder forms:
//
//	{n}    -> the index without padding      (file_7.jpg)
//	{n:04} -> the index zero-padded to width (file_0007.jpg)
//
// Everything outside the placeholder is treated as a literal, so the same
// template mechanism covers purely numeric names and "static prefix +
// sequence" names.
package pattern

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

var placeholderRe = regexp.MustCompile(`\{n(?::(\d+))?\}`)

// Pattern is a parsed URL template.
type Pattern struct {
	prefix string
	suffix string
	pad    int
	// numberInName is true when the placeholder sits in the last path
	// segment, so rendered file names are unique per index.
	numberInName bool
}

// baseName returns the last path segment of rawURL, ignoring the query.
func baseName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return path.Base(rawURL)
	}
	return path.Base(u.Path)
}

// URL renders the template for index n.
func (p *Pattern) URL(n int) string {
	return p.prefix + fmt.Sprintf("%0*d", p.pad, n) + p.suffix
}

// FileName returns the file name for index n: the last path segment of the
// rendered URL, prefixed with the index when the placeholder is elsewhere
// (for example https://host/id/{n}/200/300 yields 7_300).
func (p *Pattern) FileName(n int) string {
	base := baseName(p.URL(n))
	if p.numberInName {
		return base
	}
	return fmt.Sprintf("%0*d_%s", p.pad, n, base)
}

// Parse validates the template and returns a Pattern.
// The template must contain exactly one placeholder and be an absolute
// http(s) URL whose path ends in a file name.
func Parse(template string) (*Pattern, error) {
	matches := placeholderRe.FindAllStringSubmatchIndex(template, -1)
	switch len(matches) {
	case 0:
		return nil, errors.New("template must contain a {n} or {n:WIDTH} placeholder")
	case 1:
	default:
		return nil, errors.New("template must contain exactly one placeholder")
	}

	m := matches[0]
	p := &Pattern{prefix: template[:m[0]], suffix: template[m[1]:]}
	if m[2] >= 0 {
		width, err := strconv.Atoi(template[m[2]:m[3]])
		if err != nil || width == 0 {
			return nil, fmt.Errorf("invalid padding width %q", template[m[2]:m[3]])
		}
		p.pad = width
	}

	sample, err := url.Parse(p.URL(0))
	if err != nil {
		return nil, fmt.Errorf("template is not a valid URL: %w", err)
	}
	if sample.Scheme != "http" && sample.Scheme != "https" {
		return nil, errors.New("template must use http or https")
	}
	if sample.Host == "" {
		return nil, errors.New("template must include a host")
	}
	if sample.Path == "" || strings.HasSuffix(sample.Path, "/") {
		return nil, errors.New("template path must end in a file name")
	}
	p.numberInName = baseName(p.URL(0)) != baseName(p.URL(1))
	return p, nil
}
