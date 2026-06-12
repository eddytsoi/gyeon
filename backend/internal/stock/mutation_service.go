// Package stock owns the Stock Management module: batch stock-change
// documents called "mutations". A mutation is a draft-first list of variant
// line items, all going the same direction (all "in" or all "out"). The
// draft is editable; on Execute it atomically updates product_variants
// stock levels and writes one inventory_history row per item, all in a
// single transaction. Executed mutations are immutable.
package stock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/shop"
)

// ── Domain types ──────────────────────────────────────────────────────────

type MutationType string

const (
	TypeIn  MutationType = "in"
	TypeOut MutationType = "out"
)

type MutationStatus string

const (
	StatusDraft    MutationStatus = "draft"
	StatusExecuted MutationStatus = "executed"
)

// Mutation is the parent document.
type Mutation struct {
	ID                 string         `json:"id"`
	Number             int64          `json:"number"`
	MutationNumber     string         `json:"mutation_number"`
	Type               MutationType   `json:"type"`
	Status             MutationStatus `json:"status"`
	Note               *string        `json:"note,omitempty"`
	CreatedByAdminID   *string        `json:"created_by_admin_id,omitempty"`
	CreatedByEmail     *string        `json:"created_by_email,omitempty"`
	ExecutedByAdminID  *string        `json:"executed_by_admin_id,omitempty"`
	ExecutedByEmail    *string        `json:"executed_by_email,omitempty"`
	CreatedAt          string         `json:"created_at"`
	UpdatedAt          string         `json:"updated_at"`
	ExecutedAt         *string        `json:"executed_at,omitempty"`
	Items              []MutationItem `json:"items"`
}

// MutationItem is one variant line. quantity is always positive; the signed
// stock delta is (quantity) for type=in or (-quantity) for type=out.
//
// Bundle support: a bundle product becomes one parent row (ParentItemID nil,
// Kind = "bundle", VariantID = bundle's parent variant) followed by one
// component row per bundle item (ParentItemID = parent's ID, Kind = "simple",
// Quantity = per-bundle qty × parent qty). Execute applies stock changes
// only to component rows + standalone simple rows; parent bundle rows have
// no stock impact and leave BeforeQty / AfterQty NULL.
type MutationItem struct {
	ID           string  `json:"id"`
	MutationID   string  `json:"mutation_id"`
	VariantID    string  `json:"variant_id"`
	ParentItemID *string `json:"parent_item_id,omitempty"`
	Quantity     int     `json:"quantity"`
	BeforeQty    *int    `json:"before_qty,omitempty"`
	AfterQty     *int    `json:"after_qty,omitempty"`
	Position     int     `json:"position"`
	// Display fields populated on read for UI convenience. Never persisted.
	ProductID    *string `json:"product_id,omitempty"`
	ProductName  *string `json:"product_name,omitempty"`
	VariantName  *string `json:"variant_name,omitempty"`
	VariantSKU   *string `json:"variant_sku,omitempty"`
	CurrentStock *int    `json:"current_stock,omitempty"`
	Kind         string  `json:"kind"` // "simple" | "bundle"; derived from products.kind
	// ImageURL resolves variant's own image first, falling back to the
	// parent product's primary image.
	ImageURL *string `json:"image_url,omitempty"`
}

// ── Errors ────────────────────────────────────────────────────────────────

var (
	ErrMutationNotFound      = errors.New("mutation not found")
	ErrMutationExecuted      = errors.New("mutation already executed and cannot be modified")
	ErrInvalidType           = errors.New("type must be 'in' or 'out'")
	ErrNoItems               = errors.New("at least one item is required")
	ErrInvalidQuantity       = errors.New("quantity must be a positive integer")
	ErrDuplicateVariant      = errors.New("the same variant cannot appear twice in one mutation")
	ErrVariantNotFound       = errors.New("variant not found")
	ErrInsufficientStock     = errors.New("insufficient stock to execute mutation")
)

// StockConflict surfaces per-variant shortfall info on a failed Execute.
type StockConflict struct {
	VariantID   string  `json:"variant_id"`
	ProductName *string `json:"product_name,omitempty"`
	VariantSKU  *string `json:"variant_sku,omitempty"`
	Requested   int     `json:"requested"`
	Available   int     `json:"available"`
}

// InsufficientStockError is returned by Execute when one or more variants
// don't have enough on-hand stock to satisfy a stock-out mutation. The HTTP
// layer renders this as 422 with the conflicts array.
type InsufficientStockError struct {
	Conflicts []StockConflict
}

func (e *InsufficientStockError) Error() string {
	return ErrInsufficientStock.Error()
}

// ── Audit ─────────────────────────────────────────────────────────────────

type AuditRecorder interface {
	Record(ctx context.Context, e AuditEntry)
}

type AuditEntry struct {
	Action     string
	EntityType string
	EntityID   string
	Before     any
	After      any
}

// ── Service ───────────────────────────────────────────────────────────────

type Service struct {
	db    *sql.DB
	audit AuditRecorder
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) SetAudit(rec AuditRecorder) { s.audit = rec }

func (s *Service) record(ctx context.Context, action, entityID string, before, after any) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, AuditEntry{
		Action: action, EntityType: "stock_mutation", EntityID: entityID,
		Before: before, After: after,
	})
}

// ── Input shapes ──────────────────────────────────────────────────────────

type CreateRequest struct {
	Type  MutationType         `json:"type"`
	Note  *string              `json:"note,omitempty"`
	Items []CreateRequestItem  `json:"items"`
}

type CreateRequestItem struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

type UpdateRequest struct {
	Type  MutationType         `json:"type"`
	Note  *string              `json:"note,omitempty"`
	Items []CreateRequestItem  `json:"items"`
}

type ListFilters struct {
	Status    string // "draft" | "executed" | ""
	Type      string // "in" | "out" | ""
	From      string
	To        string
	CreatedBy string // admin_users.id of the creator | ""
	Search    string // ILIKE on mutation_number / product name / variant sku
	Limit     int
	Offset    int
}

type ListResult struct {
	Items []MutationSummary `json:"items"`
	Total int               `json:"total"`
}

// MutationSummary is the row shape returned by list queries — parent fields
// + a small summary of items (count, total qty) without the full item array.
type MutationSummary struct {
	ID               string         `json:"id"`
	MutationNumber   string         `json:"mutation_number"`
	Type             MutationType   `json:"type"`
	Status           MutationStatus `json:"status"`
	ItemCount        int            `json:"item_count"`
	TotalQuantity    int            `json:"total_quantity"`
	Note             *string        `json:"note,omitempty"`
	CreatedByEmail   *string        `json:"created_by_email,omitempty"`
	ExecutedByEmail  *string        `json:"executed_by_email,omitempty"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
	ExecutedAt       *string        `json:"executed_at,omitempty"`
	// ConsumedByOrderID is set when this out-mutation has already been combined
	// into an order; the list UI disables its checkbox to prevent double-billing.
	ConsumedByOrderID *string `json:"consumed_by_order_id,omitempty"`
}

// ── Helpers ───────────────────────────────────────────────────────────────

// mutationNumberPrefix mirrors orders.OrderService.orderNumberPrefix — pulls
// the configurable prefix from site_settings and falls back to "MUT".
func (s *Service) mutationNumberPrefix(ctx context.Context) string {
	var v string
	_ = s.db.QueryRowContext(ctx,
		`SELECT value FROM site_settings WHERE key = 'mutation_number_prefix'`).Scan(&v)
	if v == "" {
		return "MUT"
	}
	return v
}

func validateType(t MutationType) error {
	if t != TypeIn && t != TypeOut {
		return ErrInvalidType
	}
	return nil
}

func validateItems(items []CreateRequestItem) error {
	if len(items) == 0 {
		return ErrNoItems
	}
	seen := make(map[string]struct{}, len(items))
	for _, it := range items {
		if it.VariantID == "" {
			return ErrVariantNotFound
		}
		if it.Quantity <= 0 {
			return ErrInvalidQuantity
		}
		if _, dup := seen[it.VariantID]; dup {
			return ErrDuplicateVariant
		}
		seen[it.VariantID] = struct{}{}
	}
	return nil
}

// ── CRUD ──────────────────────────────────────────────────────────────────

// Create persists a new draft mutation + items in one transaction. The
// caller's admin id (from ctx) is stamped onto created_by_admin_id.
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Mutation, error) {
	if err := validateType(req.Type); err != nil {
		return nil, err
	}
	if err := validateItems(req.Items); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var adminIDArg any
	if id, ok := auth.AdminIDFromContext(ctx); ok {
		adminIDArg = id
	}

	var notePtr any
	if req.Note != nil && strings.TrimSpace(*req.Note) != "" {
		n := strings.TrimSpace(*req.Note)
		notePtr = n
	}

	var (
		id        string
		number    int64
		createdAt time.Time
		updatedAt time.Time
	)
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO stock_mutations (mutation_number, type, status, note, created_by_admin_id)
		 VALUES ('', $1, 'draft', $2, $3)
		 RETURNING id, number, created_at, updated_at`,
		string(req.Type), notePtr, adminIDArg,
	).Scan(&id, &number, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	mutationNumber := fmt.Sprintf("%s-%04d", s.mutationNumberPrefix(ctx), number)
	if _, err := tx.ExecContext(ctx,
		`UPDATE stock_mutations SET mutation_number = $2 WHERE id = $1`,
		id, mutationNumber); err != nil {
		return nil, err
	}

	if err := insertItems(ctx, tx, id, req.Items); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	out, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "stock_mutation.create", id, nil, out)
	return out, nil
}

// insertItems inserts top-level rows and — for bundle products — their
// expanded component rows. Each top-level item is looked up against
// product_variants → products to detect kind. Bundle children are pulled
// from bundle_items and inserted with parent_item_id = parent.id, quantity
// pre-multiplied by the parent qty. Mirrors orders.admin_create's bundle
// expansion (admin_create.go:238–265).
func insertItems(ctx context.Context, tx *sql.Tx, mutationID string, items []CreateRequestItem) error {
	for i, it := range items {
		var kind string
		var productID string
		err := tx.QueryRowContext(ctx,
			`SELECT p.kind, p.id
			   FROM product_variants pv
			   JOIN products p ON p.id = pv.product_id
			  WHERE pv.id = $1`, it.VariantID,
		).Scan(&kind, &productID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrVariantNotFound
		}
		if err != nil {
			return err
		}

		var parentID string
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO stock_mutation_items (mutation_id, variant_id, quantity, position)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id`,
			mutationID, it.VariantID, it.Quantity, i,
		).Scan(&parentID); err != nil {
			if strings.Contains(err.Error(), "stock_mutation_items_variant_id_fkey") {
				return ErrVariantNotFound
			}
			if strings.Contains(err.Error(), "stock_mutation_items_unique_top_level") {
				return ErrDuplicateVariant
			}
			return err
		}

		if kind != "bundle" {
			continue
		}

		// Expand bundle children — same query as orders.admin_create:240.
		compRows, err := tx.QueryContext(ctx,
			`SELECT bi.component_variant_id, bi.quantity
			   FROM bundle_items bi
			  WHERE bi.bundle_product_id = $1
			  ORDER BY bi.sort_order ASC`, productID)
		if err != nil {
			return err
		}
		type childRow struct {
			variantID string
			qty       int
		}
		var children []childRow
		for compRows.Next() {
			var c childRow
			if err := compRows.Scan(&c.variantID, &c.qty); err != nil {
				compRows.Close()
				return err
			}
			children = append(children, c)
		}
		compRows.Close()
		if err := compRows.Err(); err != nil {
			return err
		}
		for j, c := range children {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO stock_mutation_items (mutation_id, variant_id, quantity, position, parent_item_id)
				 VALUES ($1, $2, $3, $4, $5)`,
				mutationID, c.variantID, c.qty*it.Quantity, j, parentID,
			); err != nil {
				if strings.Contains(err.Error(), "stock_mutation_items_variant_id_fkey") {
					return ErrVariantNotFound
				}
				return err
			}
		}
	}
	return nil
}

// Update replaces the draft's type + items in one transaction. Rejects if
// the mutation is already executed.
func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*Mutation, error) {
	if err := validateType(req.Type); err != nil {
		return nil, err
	}
	if err := validateItems(req.Items); err != nil {
		return nil, err
	}

	before, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if before.Status == StatusExecuted {
		return nil, ErrMutationExecuted
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var notePtr any
	if req.Note != nil && strings.TrimSpace(*req.Note) != "" {
		n := strings.TrimSpace(*req.Note)
		notePtr = n
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE stock_mutations
		    SET type = $2, note = $3, updated_at = NOW()
		  WHERE id = $1 AND status = 'draft'`,
		id, string(req.Type), notePtr,
	); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM stock_mutation_items WHERE mutation_id = $1`, id); err != nil {
		return nil, err
	}
	if err := insertItems(ctx, tx, id, req.Items); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	out, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "stock_mutation.update", id, before, out)
	return out, nil
}

// Delete removes a draft mutation. Rejects if already executed.
func (s *Service) Delete(ctx context.Context, id string) error {
	before, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if before.Status == StatusExecuted {
		return ErrMutationExecuted
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM stock_mutations WHERE id = $1 AND status = 'draft'`, id); err != nil {
		return err
	}
	s.record(ctx, "stock_mutation.delete", id, before, nil)
	return nil
}

// Duplicate clones a mutation (any status) into a brand new draft. Number is
// freshly assigned; created_by is the current admin; status = draft.
func (s *Service) Duplicate(ctx context.Context, id string) (*Mutation, error) {
	src, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	req := CreateRequest{Type: src.Type, Note: src.Note}
	for _, it := range src.Items {
		// Skip bundle component rows — Create re-expands them server-side
		// from the bundle definition.
		if it.ParentItemID != nil {
			continue
		}
		req.Items = append(req.Items, CreateRequestItem{VariantID: it.VariantID, Quantity: it.Quantity})
	}
	return s.Create(ctx, req)
}

// Execute atomically applies the mutation's deltas to product_variants and
// writes one inventory_history row per item. For type=out it pre-validates
// that every variant has enough stock; on shortfall returns
// *InsufficientStockError with the list of conflicts and no DB writes.
func (s *Service) Execute(ctx context.Context, id string) (*Mutation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock the parent row.
	var (
		mType   MutationType
		mStatus MutationStatus
		mNumber string
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT type, status, mutation_number
		   FROM stock_mutations WHERE id = $1 FOR UPDATE`, id,
	).Scan(&mType, &mStatus, &mNumber); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrMutationNotFound
		}
		return nil, err
	}
	if mStatus == StatusExecuted {
		return nil, ErrMutationExecuted
	}

	// Load items. Bundle parent rows (products.kind = 'bundle' AND no
	// parent_item_id) are display-only: stock is moved on their children.
	// Top-level simple rows + every component row are "leaf" rows that
	// actually update product_variants.stock_qty on execute.
	rows, err := tx.QueryContext(ctx,
		`SELECT si.id, si.variant_id, si.quantity, si.parent_item_id, p.kind
		   FROM stock_mutation_items si
		   JOIN product_variants v ON v.id = si.variant_id
		   JOIN products         p ON p.id = v.product_id
		  WHERE si.mutation_id = $1
		  ORDER BY si.parent_item_id NULLS FIRST, si.position, si.created_at`, id)
	if err != nil {
		return nil, err
	}
	type rawItem struct {
		id, variantID string
		qty           int
		parentID      *string
		kind          string
	}
	var items []rawItem
	for rows.Next() {
		var it rawItem
		if err := rows.Scan(&it.id, &it.variantID, &it.qty, &it.parentID, &it.kind); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrNoItems
	}

	// Filter to leaf rows: bundle parents are skipped entirely; everything
	// else (top-level simple, or any component row) gets the stock delta.
	leafs := items[:0:len(items)]
	for _, it := range items {
		if it.parentID == nil && it.kind == "bundle" {
			continue
		}
		leafs = append(leafs, it)
	}
	if len(leafs) == 0 {
		return nil, ErrNoItems
	}
	items = leafs

	// Lock each variant FOR UPDATE so concurrent mutations / checkouts can't
	// race the validation → apply window.
	variantStock := make(map[string]int, len(items))
	for _, it := range items {
		var stock int
		if err := tx.QueryRowContext(ctx,
			`SELECT stock_qty FROM product_variants WHERE id = $1 FOR UPDATE`,
			it.variantID,
		).Scan(&stock); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrVariantNotFound
			}
			return nil, err
		}
		variantStock[it.variantID] = stock
	}

	// For stock-out, pre-validate: every variant must have enough total
	// stock to cover all of its leaf rows summed (two bundles sharing a
	// component, or a top-level + component row of the same variant, etc.).
	if mType == TypeOut {
		needed := make(map[string]int, len(items))
		for _, it := range items {
			needed[it.variantID] += it.qty
		}
		var conflicts []StockConflict
		for varID, qty := range needed {
			avail := variantStock[varID]
			if avail < qty {
				c := StockConflict{
					VariantID: varID,
					Requested: qty,
					Available: avail,
				}
				// Enrich with product / sku for UI display.
				var pname, sku sql.NullString
				_ = tx.QueryRowContext(ctx,
					`SELECT p.name, v.sku
					   FROM product_variants v
					   LEFT JOIN products p ON p.id = v.product_id
					  WHERE v.id = $1`, varID,
				).Scan(&pname, &sku)
				if pname.Valid {
					n := pname.String
					c.ProductName = &n
				}
				if sku.Valid {
					n := sku.String
					c.VariantSKU = &n
				}
				conflicts = append(conflicts, c)
			}
		}
		if len(conflicts) > 0 {
			return nil, &InsufficientStockError{Conflicts: conflicts}
		}
	}

	// Apply each item: update variant stock + write inventory_history +
	// snapshot before/after on the item row. variantStock is updated in
	// place so successive leaf rows for the same variant see the running
	// total (e.g. two bundle components hitting the same SKU).
	for _, it := range items {
		before := variantStock[it.variantID]
		var after int
		if mType == TypeIn {
			after = before + it.qty
		} else {
			after = before - it.qty
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE product_variants SET stock_qty = $2 WHERE id = $1`,
			it.variantID, after); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE stock_mutation_items SET before_qty = $2, after_qty = $3 WHERE id = $1`,
			it.id, before, after); err != nil {
			return nil, err
		}
		mutID := id
		note := mNumber
		shop.RecordStockChange(ctx, tx, it.variantID, before, after,
			"mutation.execute", nil, &mutID, &note)
		variantStock[it.variantID] = after
	}

	// Mark executed.
	var executorArg any
	if aid, ok := auth.AdminIDFromContext(ctx); ok {
		executorArg = aid
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE stock_mutations
		    SET status = 'executed',
		        executed_at = NOW(),
		        executed_by_admin_id = $2,
		        updated_at = NOW()
		  WHERE id = $1`,
		id, executorArg); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	out, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.record(ctx, "stock_mutation.execute", id, nil, out)
	return out, nil
}

// ── Reads ─────────────────────────────────────────────────────────────────

// GetByID returns the mutation + all items + display fields for the admin UI.
func (s *Service) GetByID(ctx context.Context, id string) (*Mutation, error) {
	var m Mutation
	var note, createdByEmail, executedByEmail, executedAt sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT m.id, m.number, m.mutation_number, m.type, m.status, m.note,
		        m.created_by_admin_id, cb.email,
		        m.executed_by_admin_id, eb.email,
		        m.created_at, m.updated_at, m.executed_at
		   FROM stock_mutations m
		   LEFT JOIN admin_users cb ON cb.id = m.created_by_admin_id
		   LEFT JOIN admin_users eb ON eb.id = m.executed_by_admin_id
		  WHERE m.id = $1`, id,
	).Scan(&m.ID, &m.Number, &m.MutationNumber, &m.Type, &m.Status, &note,
		&m.CreatedByAdminID, &createdByEmail,
		&m.ExecutedByAdminID, &executedByEmail,
		&m.CreatedAt, &m.UpdatedAt, &executedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMutationNotFound
	}
	if err != nil {
		return nil, err
	}
	if note.Valid {
		v := note.String
		m.Note = &v
	}
	if createdByEmail.Valid {
		v := createdByEmail.String
		m.CreatedByEmail = &v
	}
	if executedByEmail.Valid {
		v := executedByEmail.String
		m.ExecutedByEmail = &v
	}
	if executedAt.Valid {
		v := executedAt.String
		m.ExecutedAt = &v
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT i.id, i.mutation_id, i.variant_id, i.parent_item_id, i.quantity, i.before_qty, i.after_qty, i.position,
		        v.product_id, p.name, v.name, v.sku, v.stock_qty, p.kind,
		        COALESCE(vmf.url, vpi.url, pmf.url, ppi.url) AS image_url
		   FROM stock_mutation_items i
		   LEFT JOIN product_variants v   ON v.id = i.variant_id
		   LEFT JOIN products         p   ON p.id = v.product_id
		   LEFT JOIN product_images   vpi ON vpi.variant_id = v.id
		   LEFT JOIN media_files      vmf ON vmf.id = vpi.media_file_id
		   LEFT JOIN product_images   ppi ON ppi.product_id = v.product_id AND ppi.is_primary = TRUE
		   LEFT JOIN media_files      pmf ON pmf.id = ppi.media_file_id
		  WHERE i.mutation_id = $1
		  ORDER BY i.parent_item_id NULLS FIRST, i.position, i.created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m.Items = []MutationItem{}
	for rows.Next() {
		var it MutationItem
		var beforeQty, afterQty, stockQty sql.NullInt64
		var prodName, varName, sku, imageURL, kind sql.NullString
		if err := rows.Scan(&it.ID, &it.MutationID, &it.VariantID, &it.ParentItemID, &it.Quantity,
			&beforeQty, &afterQty, &it.Position,
			&it.ProductID, &prodName, &varName, &sku, &stockQty, &kind, &imageURL); err != nil {
			return nil, err
		}
		if kind.Valid {
			it.Kind = kind.String
		} else {
			it.Kind = "simple"
		}
		if beforeQty.Valid {
			v := int(beforeQty.Int64)
			it.BeforeQty = &v
		}
		if afterQty.Valid {
			v := int(afterQty.Int64)
			it.AfterQty = &v
		}
		if prodName.Valid {
			composed := prodName.String
			// For bundle parent rows the variant is just a wrapper (e.g.
			// "Default") and appending it muddies the display — mirror
			// orders.admin_create's handling.
			if varName.Valid && it.Kind != "bundle" {
				composed = shop.ProductDisplayName(prodName.String, varName.String)
			}
			it.ProductName = &composed
		}
		if varName.Valid {
			v := varName.String
			it.VariantName = &v
		}
		if sku.Valid {
			v := sku.String
			it.VariantSKU = &v
		}
		if stockQty.Valid {
			v := int(stockQty.Int64)
			it.CurrentStock = &v
		}
		if imageURL.Valid {
			v := imageURL.String
			it.ImageURL = &v
		}
		m.Items = append(m.Items, it)
	}
	return &m, rows.Err()
}

// List returns paginated mutation summaries with filters.
func (s *Service) List(ctx context.Context, f ListFilters) (ListResult, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	var where []string
	var args []any
	add := func(clause string, val any) {
		args = append(args, val)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if f.Status != "" {
		add("m.status = $%d", f.Status)
	}
	if f.Type != "" {
		add("m.type = $%d", f.Type)
	}
	if f.From != "" {
		add("m.created_at >= $%d", f.From)
	}
	if f.To != "" {
		// Inclusive of the whole `to` day: a date-only value like 2026-06-30
		// must include rows created any time that day, so compare against the
		// start of the next day rather than midnight of `to`.
		add("m.created_at < ($%d::date + INTERVAL '1 day')", f.To)
	}
	if f.CreatedBy != "" {
		add("m.created_by_admin_id = $%d", f.CreatedBy)
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		idx := len(args)
		where = append(where, fmt.Sprintf(
			`(m.mutation_number ILIKE $%d
			   OR EXISTS (SELECT 1 FROM stock_mutation_items si
			               LEFT JOIN product_variants v ON v.id = si.variant_id
			               LEFT JOIN products p ON p.id = v.product_id
			              WHERE si.mutation_id = m.id
			                AND (p.name ILIKE $%d OR v.sku ILIKE $%d)))`,
			idx, idx, idx))
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	out := ListResult{Items: []MutationSummary{}}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stock_mutations m `+whereSQL, args...,
	).Scan(&out.Total); err != nil {
		return out, err
	}

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, limit, offset)
	listSQL := `
		SELECT m.id, m.mutation_number, m.type, m.status, m.note,
		       cb.email, eb.email,
		       m.created_at, m.updated_at, m.executed_at,
		       COALESCE(agg.item_count, 0), COALESCE(agg.total_qty, 0),
		       m.consumed_by_order_id
		  FROM stock_mutations m
		  LEFT JOIN admin_users cb ON cb.id = m.created_by_admin_id
		  LEFT JOIN admin_users eb ON eb.id = m.executed_by_admin_id
		  LEFT JOIN (
		      -- Bundle parent rows are display-only; the real stock movement
		      -- (and so the meaningful count + total qty) lives on the leaves:
		      -- standalone simple rows + every component row.
		      SELECT si.mutation_id,
		             COUNT(*)::int        AS item_count,
		             SUM(si.quantity)::int AS total_qty
		        FROM stock_mutation_items si
		        LEFT JOIN product_variants pv ON pv.id = si.variant_id
		        LEFT JOIN products         p  ON p.id  = pv.product_id
		       WHERE NOT (si.parent_item_id IS NULL AND p.kind = 'bundle')
		       GROUP BY si.mutation_id
		  ) agg ON agg.mutation_id = m.id
		` + whereSQL + fmt.Sprintf(`
		 ORDER BY m.created_at DESC
		 LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)

	rows, err := s.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var r MutationSummary
		var note, cbEmail, ebEmail, executedAt, consumedBy sql.NullString
		if err := rows.Scan(&r.ID, &r.MutationNumber, &r.Type, &r.Status, &note,
			&cbEmail, &ebEmail, &r.CreatedAt, &r.UpdatedAt, &executedAt,
			&r.ItemCount, &r.TotalQuantity, &consumedBy); err != nil {
			return out, err
		}
		if consumedBy.Valid {
			v := consumedBy.String
			r.ConsumedByOrderID = &v
		}
		if note.Valid {
			v := note.String
			r.Note = &v
		}
		if cbEmail.Valid {
			v := cbEmail.String
			r.CreatedByEmail = &v
		}
		if ebEmail.Valid {
			v := ebEmail.String
			r.ExecutedByEmail = &v
		}
		if executedAt.Valid {
			v := executedAt.String
			r.ExecutedAt = &v
		}
		out.Items = append(out.Items, r)
	}
	return out, rows.Err()
}

// Creator is a distinct admin who has authored at least one stock mutation —
// the option set for the list page's "created by" filter.
type Creator struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// ListCreators returns the distinct admins who have created any stock mutation,
// ordered by email. Drives the creator filter dropdown.
func (s *Service) ListCreators(ctx context.Context) ([]Creator, error) {
	out := []Creator{}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT au.id, au.email, au.name
		  FROM stock_mutations m
		  JOIN admin_users au ON au.id = m.created_by_admin_id
		 ORDER BY au.email`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Creator
		if err := rows.Scan(&c.ID, &c.Email, &c.Name); err != nil {
			return out, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
