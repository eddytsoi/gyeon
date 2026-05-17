package importer

import (
	"strings"
	"sync"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

var (
	mdConverterOnce sync.Once
	mdConverter     *md.Converter
)

func htmlToMarkdown(htmlStr string) string {
	if htmlStr == "" {
		return ""
	}
	mdConverterOnce.Do(func() {
		mdConverter = md.NewConverter("", true, nil)
	})
	out, err := mdConverter.ConvertString(htmlStr)
	if err != nil {
		return htmlStr
	}
	return strings.TrimSpace(out)
}

// metaString returns the first meta_data entry matching key, as a string.
// Returns "" if the key is missing or the value isn't a JSON string.
func metaString(meta []wcMeta, key string) string {
	for _, m := range meta {
		if m.Key == key {
			return m.String()
		}
	}
	return ""
}

// metaStringPtr is like metaString but returns nil for missing/empty values,
// suited to optional *string fields on upsert requests.
func metaStringPtr(meta []wcMeta, key string) *string {
	v := strings.TrimSpace(metaString(meta, key))
	if v == "" {
		return nil
	}
	return &v
}

// wcMediaSlugs collects the six product-level image slugs WC stores in
// product meta (banner_1, banner_2, media_1..media_4). Each one points
// at a WP attachment that the importer resolves and downloads.
type wcMediaSlugs struct {
	Banner1, Banner2               string
	Media1, Media2, Media3, Media4 string
}

func extractMediaSlugs(prod wcProduct) wcMediaSlugs {
	return wcMediaSlugs{
		Banner1: strings.TrimSpace(metaString(prod.MetaData, "banner_1")),
		Banner2: strings.TrimSpace(metaString(prod.MetaData, "banner_2")),
		Media1:  strings.TrimSpace(metaString(prod.MetaData, "media_1")),
		Media2:  strings.TrimSpace(metaString(prod.MetaData, "media_2")),
		Media3:  strings.TrimSpace(metaString(prod.MetaData, "media_3")),
		Media4:  strings.TrimSpace(metaString(prod.MetaData, "media_4")),
	}
}

// buildContentFromMeta maps a WC product's ACF custom fields onto the
// (description, how_to_use, excerpt) trio Gyeon stores. Layout:
//
//   description  ← title_1 / content_1 + title_2 / content_2
//   how_to_use   ← title_3 / content_3
//   excerpt      ← prod.short_description (unchanged)
//
// Each title becomes an `### {title}` markdown heading; the content is
// converted from HTML to markdown so existing inline HTML inside ACF
// (lists, bold, nested h4s, …) survives. When the ACF fields aren't
// populated, description falls back to the WC `description` field so
// products that don't follow this template still get something useful.
// how_to_use is only set when at least one of title_3 / content_3 has
// content — there's no equivalent fallback source on the WC side.
func buildContentFromMeta(prod wcProduct) (description, howToUse, excerpt *string) {
	desc := buildMarkdownSections(
		[2]string{metaString(prod.MetaData, "title_1"), metaString(prod.MetaData, "content_1")},
		[2]string{metaString(prod.MetaData, "title_2"), metaString(prod.MetaData, "content_2")},
	)
	if desc == "" && prod.Description != "" {
		desc = htmlToMarkdown(prod.Description)
	}
	if desc != "" {
		description = &desc
	}

	hu := buildMarkdownSections(
		[2]string{metaString(prod.MetaData, "title_3"), metaString(prod.MetaData, "content_3")},
	)
	if hu != "" {
		howToUse = &hu
	}

	if prod.ShortDescription != "" {
		ex := htmlToMarkdown(prod.ShortDescription)
		excerpt = &ex
	}
	return
}

// buildMarkdownSections joins (title, htmlContent) pairs into a single
// markdown blob. Each non-empty title is emitted as an `### {title}`
// heading; the html content is converted via htmlToMarkdown so any
// inline HTML (lists, bold, headings already inside the content) maps
// to clean markdown. Pairs where both halves are empty are skipped, so
// products with only one populated section produce a tight result.
func buildMarkdownSections(pairs ...[2]string) string {
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		title := strings.TrimSpace(p[0])
		body := strings.TrimSpace(htmlToMarkdown(p[1]))
		if title == "" && body == "" {
			continue
		}
		var section string
		switch {
		case title != "" && body != "":
			// Title on its own line, content on the very next line — one
			// blank line only between sections (handled by the join below).
			section = "### " + title + "\n" + body
		case title != "":
			section = "### " + title
		default:
			section = body
		}
		parts = append(parts, section)
	}
	return strings.Join(parts, "\n\n")
}
