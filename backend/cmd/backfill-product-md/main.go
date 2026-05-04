// One-shot tool to convert HTML stored in products.description / products.excerpt
// (and product_translations.description) into Markdown for all WC-imported rows.
//
// Run after deploying the importer's HTML→Markdown conversion so previously
// imported products render correctly under the storefront's Markdown renderer.
//
// Usage:
//   DATABASE_URL=postgres://... go run ./cmd/backfill-product-md           # dry run
//   DATABASE_URL=postgres://... go run ./cmd/backfill-product-md --apply   # write changes
package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"strings"
	"sync"

	md "github.com/JohannesKaufmann/html-to-markdown"
	_ "github.com/lib/pq"
)

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var (
	convOnce sync.Once
	conv     *md.Converter
)

func htmlToMarkdown(s string) string {
	if s == "" {
		return ""
	}
	convOnce.Do(func() { conv = md.NewConverter("", true, nil) })
	out, err := conv.ConvertString(s)
	if err != nil {
		return s
	}
	return strings.TrimSpace(out)
}

// looksLikeHTML returns true when the string clearly contains HTML markup
// the importer would have left untouched. Plain text and existing Markdown
// pass through unchanged.
func looksLikeHTML(s string) bool {
	if s == "" {
		return false
	}
	for _, tag := range []string{"<p", "<br", "<div", "<span", "<ul", "<ol", "<li",
		"<strong", "<em", "<b>", "<i>", "<a ", "<h1", "<h2", "<h3", "<h4",
		"<img", "<table", "<blockquote", "<pre", "<code"} {
		if strings.Contains(s, tag) {
			return true
		}
	}
	return false
}

func main() {
	apply := flag.Bool("apply", false, "Write changes (default: dry run, prints intended updates)")
	flag.Parse()

	dsn := getenv("DATABASE_URL", "postgres://gyeon:gyeon@localhost:5432/gyeon?sslmode=disable")
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		log.Fatalf("ping: %v", err)
	}

	mode := "DRY RUN"
	if *apply {
		mode = "APPLY"
	}
	log.Printf("backfill-product-md (%s) — DSN=%s", mode, dsn)

	// products: only WC-imported rows (manual rows untouched)
	rows, err := conn.Query(`
		SELECT id, COALESCE(description, ''), COALESCE(excerpt, '')
		FROM products
		WHERE wc_product_id IS NOT NULL
	`)
	if err != nil {
		log.Fatalf("query products: %v", err)
	}
	defer rows.Close()

	var updatedProducts, skippedProducts int
	for rows.Next() {
		var id, desc, excerpt string
		if err := rows.Scan(&id, &desc, &excerpt); err != nil {
			log.Fatalf("scan products: %v", err)
		}
		newDesc, newExcerpt := desc, excerpt
		changed := false
		if looksLikeHTML(desc) {
			newDesc = htmlToMarkdown(desc)
			changed = true
		}
		if looksLikeHTML(excerpt) {
			newExcerpt = htmlToMarkdown(excerpt)
			changed = true
		}
		if !changed {
			skippedProducts++
			continue
		}
		updatedProducts++
		log.Printf("product %s: description %d→%d chars, excerpt %d→%d chars",
			id, len(desc), len(newDesc), len(excerpt), len(newExcerpt))
		if *apply {
			if _, err := conn.Exec(
				`UPDATE products SET description = NULLIF($2,''), excerpt = NULLIF($3,''), updated_at = NOW() WHERE id = $1`,
				id, newDesc, newExcerpt,
			); err != nil {
				log.Fatalf("update product %s: %v", id, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("iterate products: %v", err)
	}

	// product_translations: scoped to rows whose product is WC-imported
	tRows, err := conn.Query(`
		SELECT pt.product_id, pt.locale, COALESCE(pt.description, '')
		FROM product_translations pt
		JOIN products p ON p.id = pt.product_id
		WHERE p.wc_product_id IS NOT NULL
	`)
	if err != nil {
		log.Fatalf("query translations: %v", err)
	}
	defer tRows.Close()

	var updatedTr, skippedTr int
	for tRows.Next() {
		var pid, locale, desc string
		if err := tRows.Scan(&pid, &locale, &desc); err != nil {
			log.Fatalf("scan translation: %v", err)
		}
		if !looksLikeHTML(desc) {
			skippedTr++
			continue
		}
		newDesc := htmlToMarkdown(desc)
		updatedTr++
		log.Printf("translation %s/%s: %d→%d chars", pid, locale, len(desc), len(newDesc))
		if *apply {
			if _, err := conn.Exec(
				`UPDATE product_translations SET description = NULLIF($3,''), updated_at = NOW() WHERE product_id = $1 AND locale = $2`,
				pid, locale, newDesc,
			); err != nil {
				log.Fatalf("update translation %s/%s: %v", pid, locale, err)
			}
		}
	}
	if err := tRows.Err(); err != nil {
		log.Fatalf("iterate translations: %v", err)
	}

	log.Printf("done — products: %d converted, %d unchanged | translations: %d converted, %d unchanged",
		updatedProducts, skippedProducts, updatedTr, skippedTr)
	if !*apply {
		log.Printf("(dry run — re-run with --apply to write changes)")
	}
}
