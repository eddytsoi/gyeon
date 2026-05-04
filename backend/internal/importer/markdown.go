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
