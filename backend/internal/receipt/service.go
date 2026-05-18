// Package receipt builds PDF order receipts. The pipeline is:
//
//  1. service.go loads order + shipping address (via orders.Service) and
//     enriches each line item with a product/variant image URL straight from
//     the order_items → product_variants → product_images → media_files
//     relation, then merges in the company branding from site_settings.
//  2. template.go renders the embedded HTML template against that view model.
//  3. renderer.go feeds the resulting HTML to a headless Chromium and returns
//     the printed PDF bytes.
package receipt

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gyeon/backend/internal/orders"
	"gyeon/backend/internal/settings"
)

// ErrNotReceiptable indicates the order exists but is in a status that should
// not produce a receipt (pending / cancelled / refunded). The handler maps
// this to 409 so the storefront can show a useful message rather than
// rendering an empty PDF for a fully refunded order.
var ErrNotReceiptable = errors.New("order is not in a receiptable status")

// receiptableStatuses are the order statuses for which a receipt can be
// downloaded. Refunded orders are intentionally excluded — they had a
// receipt at one point, but the refund flow already emails the customer
// and re-issuing a "Receipt" for a fully reversed payment is misleading.
var receiptableStatuses = map[orders.OrderStatus]bool{
	orders.StatusPaid:       true,
	orders.StatusProcessing: true,
	orders.StatusShipped:    true,
	orders.StatusDelivered:  true,
}

type Service struct {
	db          *sql.DB
	orderSvc    *orders.OrderService
	settingsSvc *settings.Service
	renderer    *Renderer
}

func NewService(db *sql.DB, orderSvc *orders.OrderService, settingsSvc *settings.Service, renderer *Renderer) *Service {
	return &Service{
		db:          db,
		orderSvc:    orderSvc,
		settingsSvc: settingsSvc,
		renderer:    renderer,
	}
}

// GenerateForOrder builds and prints the receipt PDF for the given order.
// locale is one of "en" / "zh-Hant" (defaults to "en" for anything else).
// The order must be in a receiptable status; otherwise returns ErrNotReceiptable.
func (s *Service) GenerateForOrder(ctx context.Context, order *orders.Order, locale string) ([]byte, error) {
	if !receiptableStatuses[order.Status] {
		return nil, ErrNotReceiptable
	}
	locale = resolveLocale(locale)

	images, err := s.fetchOrderItemImages(ctx, order.ID)
	if err != nil {
		// Best-effort: a failure to look up thumbnails should never stop the
		// receipt from generating. The image cells will just render blank.
		images = map[string]string{}
	}

	view := s.buildView(ctx, order, images, locale)

	var buf bytes.Buffer
	if err := receiptTemplate.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("execute receipt template: %w", err)
	}

	pdf, err := s.renderer.Render(ctx, buf.String())
	if err != nil {
		return nil, fmt.Errorf("render PDF: %w", err)
	}
	return pdf, nil
}

// fetchOrderItemImages returns a map order_item_id → image URL using the same
// coalesce strategy as the cart so the receipt thumbnail matches the storefront
// (variant image, falling back to the primary product image, falling back to
// video thumbnail).
func (s *Service) fetchOrderItemImages(ctx context.Context, orderID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT oi.id,
		       COALESCE(
		           CASE WHEN vmf.mime_type LIKE 'video/%' THEN vmf.thumbnail_url END,
		           vmf.webp_url, vmf.url, vi.url,
		           CASE WHEN pmf.mime_type LIKE 'video/%' THEN pmf.thumbnail_url END,
		           pmf.webp_url, pmf.url, pi.url,
		           ''
		       ) AS image_url
		  FROM order_items oi
		  LEFT JOIN product_variants pv ON pv.id = oi.variant_id
		  LEFT JOIN product_images vi  ON vi.variant_id = oi.variant_id
		  LEFT JOIN media_files    vmf ON vmf.id = vi.media_file_id
		  LEFT JOIN product_images pi
		         ON pi.product_id = pv.product_id AND pi.is_primary = TRUE
		  LEFT JOIN media_files pmf ON pmf.id = pi.media_file_id
		 WHERE oi.order_id = $1`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var id, url string
		if err := rows.Scan(&id, &url); err != nil {
			return nil, err
		}
		if url != "" {
			out[id] = url
		}
	}
	return out, rows.Err()
}

type viewShop struct {
	Name           string
	LogoURL        string
	AddressBlock   string
	Phone          string
	Email          string
	RegistrationNo string
}

type viewParty struct {
	Name         string
	Email        string
	Phone        string
	AddressBlock string
}

type viewRow struct {
	Index        int
	IsChild      bool
	ImageURL     string
	ProductName  string
	SKU          string
	Attrs        string
	Quantity     int
	UnitPriceFmt string
	LineTotalFmt string
}

type viewModel struct {
	Locale      string
	L           map[string]string
	Shop        viewShop
	Order       *orders.Order
	ReceiptDate string
	PlacedOn    string
	BillTo      viewParty
	ShipTo      *viewParty
	Rows        []viewRow
	HasDiscount bool
	HasTax      bool
	SubtotalFmt string
	DiscountFmt string
	TaxFmt      string
	ShippingFmt string
	TotalFmt    string
	PaymentLine string
}

// buildView assembles the data passed into the HTML template. Pulling settings
// here (rather than from the handler) keeps the receipt route handler thin
// and makes the view model easy to test in isolation against fixtures.
func (s *Service) buildView(ctx context.Context, order *orders.Order, images map[string]string, locale string) viewModel {
	shop := s.loadShop(ctx)
	currency := s.settingValue(ctx, "currency", "HKD")

	bill := viewParty{}
	if order.CustomerName != nil {
		bill.Name = *order.CustomerName
	}
	if order.CustomerEmail != nil {
		bill.Email = *order.CustomerEmail
	}
	if order.CustomerPhone != nil {
		bill.Phone = *order.CustomerPhone
	}

	var ship *viewParty
	if order.ShippingAddress != nil {
		a := order.ShippingAddress
		name := strings.TrimSpace(a.FirstName + " " + a.LastName)
		if name == "" {
			name = bill.Name
		}
		phone := ""
		if a.Phone != nil {
			phone = *a.Phone
		}
		ship = &viewParty{
			Name:         name,
			Phone:        phone,
			AddressBlock: composeAddress(a.Line1, ptrToStr(a.Line2), a.City, ptrToStr(a.State), a.PostalCode, a.Country),
		}
	}

	rows := assembleRows(order.Items, images, currency)

	receiptDate := time.Now().UTC()
	if order.PaidAt != nil && *order.PaidAt != "" {
		if t, err := time.Parse(time.RFC3339, *order.PaidAt); err == nil {
			receiptDate = t
		} else if t, err := time.Parse(time.RFC3339Nano, *order.PaidAt); err == nil {
			receiptDate = t
		}
	}
	placedOn := receiptDate
	if t, err := time.Parse(time.RFC3339, order.CreatedAt); err == nil {
		placedOn = t
	} else if t, err := time.Parse(time.RFC3339Nano, order.CreatedAt); err == nil {
		placedOn = t
	}

	return viewModel{
		Locale:      locale,
		L:           labels[locale],
		Shop:        shop,
		Order:       order,
		ReceiptDate: formatDate(receiptDate, locale),
		PlacedOn:    formatDate(placedOn, locale),
		BillTo:      bill,
		ShipTo:      ship,
		Rows:        rows,
		HasDiscount: order.DiscountAmount > 0,
		HasTax:      order.TaxAmount > 0,
		SubtotalFmt: fmtMoney(order.Subtotal, currency),
		DiscountFmt: fmtMoney(order.DiscountAmount, currency),
		TaxFmt:      fmtMoney(order.TaxAmount, currency),
		ShippingFmt: fmtMoney(order.ShippingFee, currency),
		TotalFmt:    fmtMoney(order.Total, currency),
		PaymentLine: formatPayment(order),
	}
}

func (s *Service) loadShop(ctx context.Context) viewShop {
	get := func(key string) string { return s.settingValue(ctx, key, "") }
	name := get("site_name")
	if name == "" {
		name = "Gyeon"
	}
	address := composeAddress(
		get("company_address_line1"),
		get("company_address_line2"),
		get("company_city"),
		"", // no state field in site_settings
		get("company_postal_code"),
		get("company_country"),
	)
	return viewShop{
		Name:           name,
		LogoURL:        get("company_logo_url"),
		AddressBlock:   address,
		Phone:          get("company_phone"),
		Email:          get("contact_email"),
		RegistrationNo: get("company_registration_no"),
	}
}

func (s *Service) settingValue(ctx context.Context, key, fallback string) string {
	st, err := s.settingsSvc.Get(ctx, key)
	if err != nil || st == nil {
		return fallback
	}
	v := strings.TrimSpace(st.Value)
	if v == "" {
		return fallback
	}
	return v
}

// assembleRows flattens the bundle parent/child relationship and assigns a
// running 1-based index to top-level parent rows. Children inherit no index
// and render with the "↳" indent in the template.
func assembleRows(items []orders.OrderItem, images map[string]string, currency string) []viewRow {
	type idxed struct {
		item     orders.OrderItem
		original int
	}
	parents := make([]idxed, 0, len(items))
	childrenByParent := make(map[string][]orders.OrderItem)
	for i, it := range items {
		if it.ParentItemID != nil && *it.ParentItemID != "" {
			childrenByParent[*it.ParentItemID] = append(childrenByParent[*it.ParentItemID], it)
		} else {
			parents = append(parents, idxed{item: it, original: i})
		}
	}
	// Preserve the order of parents as returned by the order service.
	sort.SliceStable(parents, func(a, b int) bool { return parents[a].original < parents[b].original })

	rows := make([]viewRow, 0, len(items))
	rowIdx := 1
	for _, p := range parents {
		rows = append(rows, viewRow{
			Index:        rowIdx,
			IsChild:      false,
			ImageURL:     images[p.item.ID],
			ProductName:  p.item.ProductName,
			SKU:          p.item.VariantSKU,
			Attrs:        formatVariantAttrs(p.item.VariantAttrs),
			Quantity:     p.item.Quantity,
			UnitPriceFmt: fmtMoney(p.item.UnitPrice, currency),
			LineTotalFmt: fmtMoney(p.item.LineTotal, currency),
		})
		for _, c := range childrenByParent[p.item.ID] {
			rows = append(rows, viewRow{
				IsChild:     true,
				ImageURL:    images[c.ID],
				ProductName: c.ProductName,
				SKU:         c.VariantSKU,
				Attrs:       formatVariantAttrs(c.VariantAttrs),
				Quantity:    c.Quantity,
			})
		}
		rowIdx++
	}
	return rows
}

func formatVariantAttrs(attrs map[string]interface{}) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %v", k, attrs[k]))
	}
	return strings.Join(parts, " · ")
}

func formatPayment(order *orders.Order) string {
	if order.CardBrand != nil && order.CardLast4 != nil && *order.CardBrand != "" && *order.CardLast4 != "" {
		brand := *order.CardBrand
		switch strings.ToLower(brand) {
		case "visa":
			brand = "Visa"
		case "mastercard":
			brand = "Mastercard"
		case "amex":
			brand = "Amex"
		case "jcb":
			brand = "JCB"
		case "discover":
			brand = "Discover"
		case "diners":
			brand = "Diners"
		case "unionpay":
			brand = "UnionPay"
		}
		return fmt.Sprintf("%s •••• %s", brand, *order.CardLast4)
	}
	if order.PaymentMethod != nil && *order.PaymentMethod != "" {
		return *order.PaymentMethod
	}
	return ""
}

func composeAddress(line1, line2, city, state, postal, country string) string {
	parts := []string{}
	if line1 != "" {
		parts = append(parts, line1)
	}
	if line2 != "" {
		parts = append(parts, line2)
	}
	cityState := strings.TrimSpace(strings.Join(filterNonEmpty(city, state), ", "))
	if cityState != "" {
		parts = append(parts, cityState)
	}
	if postal != "" {
		parts = append(parts, postal)
	}
	if country != "" {
		parts = append(parts, country)
	}
	return strings.Join(parts, "\n")
}

func filterNonEmpty(ss ...string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func ptrToStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// fmtMoney renders amount as HKD currency: HK$X,XXX.XX. Currency code is
// accepted for future expansion but we currently only ship a HKD format —
// other currencies fall back to "<CODE> X,XXX.XX".
func fmtMoney(amount float64, currency string) string {
	if currency == "" || strings.EqualFold(currency, "HKD") {
		return "HK$" + formatThousands(amount)
	}
	return strings.ToUpper(currency) + " " + formatThousands(amount)
}

func formatThousands(amount float64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}
	s := fmt.Sprintf("%.2f", amount)
	dot := strings.IndexByte(s, '.')
	whole := s[:dot]
	frac := s[dot:]
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	out := b.String() + frac
	if negative {
		return "-" + out
	}
	return out
}

func formatDate(t time.Time, locale string) string {
	if locale == "zh-Hant" {
		return t.Format("2006年01月02日")
	}
	return t.Format("Jan 2, 2006")
}
