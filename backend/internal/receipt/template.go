package receipt

import (
	_ "embed"
	"html/template"
)

//go:embed receipt.html.tmpl
var receiptTemplateSource string

var receiptTemplate = template.Must(template.New("receipt").Funcs(template.FuncMap{
	"add":    func(a, b int) int { return a + b },
	"hasAny": func(ss ...string) bool {
		for _, s := range ss {
			if s != "" {
				return true
			}
		}
		return false
	},
}).Parse(receiptTemplateSource))

// labels is the i18n bundle for receipt copy. Embedded in this file rather
// than loaded from disk so the receipt endpoint has no extra runtime
// dependencies. New keys here must be added to every supported locale to
// avoid blank strings rendering on the PDF.
var labels = map[string]map[string]string{
	"en": {
		"receipt_title":      "Receipt",
		"receipt_number":     "Receipt no",
		"receipt_date":       "Receipt date",
		"order_number":       "Order no",
		"placed_on":          "Placed on",
		"bill_to":            "Bill to",
		"ship_to":            "Ship to",
		"item":               "Item",
		"image":              "Image",
		"product":            "Product",
		"qty":                "Qty",
		"unit_price":         "Price",
		"line_total":         "Total price",
		"subtotal":           "Subtotal",
		"shipping":           "Shipping",
		"discount":           "Discount",
		"tax":                "Tax",
		"total":              "Total",
		"payment_method":     "Payment method",
		"credit_card":        "Credit card",
		"included_in_bundle": "Included in bundle",
		"thank_you":          "Thank you for your order!",
		"contact_us":         "Questions? Contact us at",
		"registration_no":    "BR",
		"page":               "Page",
		"of":                 "of",
	},
	"zh-Hant": {
		"receipt_title":      "收據",
		"receipt_number":     "收據編號",
		"receipt_date":       "收據日期",
		"order_number":       "訂單編號",
		"placed_on":          "下單日期",
		"bill_to":            "帳單地址",
		"ship_to":            "送貨地址",
		"item":               "項",
		"image":              "圖片",
		"product":            "產品",
		"qty":                "數量",
		"unit_price":         "單價",
		"line_total":         "小計",
		"subtotal":           "貨品總額",
		"shipping":           "運費",
		"discount":           "折扣",
		"tax":                "稅項",
		"total":              "總額",
		"payment_method":     "付款方式",
		"credit_card":        "信用卡",
		"included_in_bundle": "套裝包含",
		"thank_you":          "感謝您的訂購！",
		"contact_us":         "如有疑問請聯絡我們：",
		"registration_no":    "商業登記",
		"page":               "第",
		"of":                 "頁，共",
	},
}

// resolveLocale normalises an incoming locale value into one of the supported
// keys, defaulting to "en" for anything unrecognised. Accepts common
// case-insensitive aliases ("zh-hant", "zh-tw", "zh-hk", "zh") so callers
// don't need to know our exact key spelling.
func resolveLocale(raw string) string {
	switch raw {
	case "zh-Hant", "zh-hant", "zh-TW", "zh-tw", "zh-HK", "zh-hk", "zh":
		return "zh-Hant"
	}
	return "en"
}

func t(locale, key string) string {
	if bundle, ok := labels[locale]; ok {
		if v, ok := bundle[key]; ok {
			return v
		}
	}
	if v, ok := labels["en"][key]; ok {
		return v
	}
	return key
}
