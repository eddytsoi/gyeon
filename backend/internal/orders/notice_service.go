package orders

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type NoticeRole string

const (
	NoticeRoleSystem   NoticeRole = "system"
	NoticeRoleAdmin    NoticeRole = "admin"
	NoticeRoleCustomer NoticeRole = "customer"
)

// Notice is one entry in an order's unified timeline of system events,
// admin-to-customer messages, and customer-to-admin replies.
type Notice struct {
	ID        string     `json:"id"`
	OrderID   string     `json:"order_id"`
	Role      NoticeRole `json:"role"`
	Status    *string    `json:"status,omitempty"` // only set for role='system' notices produced by a status transition
	Body      string     `json:"body"`
	AuthorID  *string    `json:"author_id,omitempty"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

var ErrNoticeNotAllowed = errors.New("notice role not permitted on this endpoint")
var ErrNoticeBodyEmpty = errors.New("notice body is required")

type NoticeService struct {
	db *sql.DB
}

func NewNoticeService(db *sql.DB) *NoticeService {
	return &NoticeService{db: db}
}

// List returns notices for an order in chronological order.
// When forCustomer is true, role='system' rows are filtered out.
func (s *NoticeService) List(ctx context.Context, orderID string, forCustomer bool) ([]Notice, error) {
	q := `SELECT id, order_id, role, status, body, author_id, read_at, created_at
	      FROM order_notices
	      WHERE order_id = $1`
	if forCustomer {
		q += ` AND role <> 'system'`
	}
	q += ` ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Notice
	for rows.Next() {
		var n Notice
		var status, authorID sql.NullString
		var readAt sql.NullTime
		if err := rows.Scan(&n.ID, &n.OrderID, &n.Role, &status, &n.Body, &authorID, &readAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		if status.Valid {
			s := status.String
			n.Status = &s
		}
		if authorID.Valid {
			a := authorID.String
			n.AuthorID = &a
		}
		if readAt.Valid {
			t := readAt.Time
			n.ReadAt = &t
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CreateSystemNoticeTx inserts a system notice within an existing transaction.
// Used by the order service so status transitions and the resulting notice
// land atomically.
func CreateSystemNoticeTx(ctx context.Context, tx *sql.Tx, orderID string, status *OrderStatus, body string) error {
	if body == "" {
		return ErrNoticeBodyEmpty
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO order_notices (order_id, role, status, body) VALUES ($1, 'system', $2, $3)`,
		orderID, status, body)
	return err
}

// CreateSystemNotice inserts an ad-hoc system notice (e.g. an admin's internal
// note unrelated to a status change). status is nil for these.
func (s *NoticeService) CreateSystemNotice(ctx context.Context, orderID, body string) (*Notice, error) {
	return s.insert(ctx, orderID, NoticeRoleSystem, nil, body, nil)
}

// CreateAdminMessage records an admin → customer message.
func (s *NoticeService) CreateAdminMessage(ctx context.Context, orderID, adminID, body string) (*Notice, error) {
	return s.insert(ctx, orderID, NoticeRoleAdmin, nil, body, &adminID)
}

// CreateCustomerMessage records a customer → admin reply.
func (s *NoticeService) CreateCustomerMessage(ctx context.Context, orderID, customerID, body string) (*Notice, error) {
	return s.insert(ctx, orderID, NoticeRoleCustomer, nil, body, &customerID)
}

func (s *NoticeService) insert(ctx context.Context, orderID string, role NoticeRole, status *OrderStatus, body string, authorID *string) (*Notice, error) {
	if body == "" {
		return nil, ErrNoticeBodyEmpty
	}
	var n Notice
	var statusOut, authorOut sql.NullString
	var readAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO order_notices (order_id, role, status, body, author_id)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, order_id, role, status, body, author_id, read_at, created_at`,
		orderID, role, status, body, authorID).
		Scan(&n.ID, &n.OrderID, &n.Role, &statusOut, &n.Body, &authorOut, &readAt, &n.CreatedAt)
	if err != nil {
		return nil, err
	}
	if statusOut.Valid {
		v := statusOut.String
		n.Status = &v
	}
	if authorOut.Valid {
		v := authorOut.String
		n.AuthorID = &v
	}
	if readAt.Valid {
		v := readAt.Time
		n.ReadAt = &v
	}
	return &n, nil
}

// MarkAdminNoticesRead stamps read_at on all unread role='admin' notices for an order.
// Called when a customer opens the order detail page.
func (s *NoticeService) MarkAdminNoticesRead(ctx context.Context, orderID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE order_notices SET read_at = NOW()
		 WHERE order_id = $1 AND role = 'admin' AND read_at IS NULL`, orderID)
	return err
}

// MarkCustomerNoticesRead stamps read_at on all unread role='customer' notices for an order.
// Called when an admin opens the order detail page.
func (s *NoticeService) MarkCustomerNoticesRead(ctx context.Context, orderID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE order_notices SET read_at = NOW()
		 WHERE order_id = $1 AND role = 'customer' AND read_at IS NULL`, orderID)
	return err
}

// UnreadCountsForCustomer returns a map of order_id → count of unread admin
// notices, scoped to orders owned by the given customer. Used to render
// per-order badges in /account/orders.
func (s *NoticeService) UnreadCountsForCustomer(ctx context.Context, customerID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT n.order_id, COUNT(*)
		 FROM order_notices n
		 JOIN orders o ON o.id = n.order_id
		 WHERE o.customer_id = $1
		   AND n.role = 'admin'
		   AND n.read_at IS NULL
		 GROUP BY n.order_id`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// UnreadCountsForAdmin returns a map of order_id → count of unread customer
// notices across all orders. Used to render per-order badges in /admin/orders.
func (s *NoticeService) UnreadCountsForAdmin(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT order_id, COUNT(*)
		 FROM order_notices
		 WHERE role = 'customer' AND read_at IS NULL
		 GROUP BY order_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// OrderOwnedByCustomer returns true iff the order belongs to the given customer.
// Used to enforce ownership on customer-side notice endpoints.
func (s *NoticeService) OrderOwnedByCustomer(ctx context.Context, orderID, customerID string) (bool, error) {
	var ownerID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT customer_id FROM orders WHERE id = $1`, orderID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return ownerID.Valid && ownerID.String == customerID, nil
}

// CustomerEmailForOrder fetches the customer email for the email notification flow.
// Returns "", "" if the order has no email on file.
func (s *NoticeService) CustomerEmailForOrder(ctx context.Context, orderID string) (email, name, orderNumber string, err error) {
	var emailNS, nameNS sql.NullString
	var numberNS sql.NullString
	err = s.db.QueryRowContext(ctx,
		`SELECT customer_email, customer_name, COALESCE(order_number, '')
		 FROM orders WHERE id = $1`, orderID).Scan(&emailNS, &nameNS, &numberNS)
	if err != nil {
		return "", "", "", err
	}
	if emailNS.Valid {
		email = emailNS.String
	}
	if nameNS.Valid {
		name = nameNS.String
	}
	if numberNS.Valid {
		orderNumber = numberNS.String
	}
	return
}
