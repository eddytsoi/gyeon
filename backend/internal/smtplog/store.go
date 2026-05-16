// Package smtplog records every outbound email attempt by the queue worker.
// Rows include the rendered subject/body so the admin "Resend" action can
// replay the exact payload without re-running the template engine.
package smtplog

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
)

type Row struct {
	ID                string  `json:"id"`
	QueueJobID        *string `json:"queue_job_id,omitempty"`
	TemplateKey       *string `json:"template_key,omitempty"`
	TriggerCondition  string  `json:"trigger_condition"`
	RelatedEntityType *string `json:"related_entity_type,omitempty"`
	RelatedEntityID   *string `json:"related_entity_id,omitempty"`
	Recipient         string  `json:"recipient"`
	FromEmail         string  `json:"from_email"`
	FromName          *string `json:"from_name,omitempty"`
	ReplyTo           *string `json:"reply_to,omitempty"`
	Subject           string  `json:"subject"`
	BodyHTML          string  `json:"body_html"`
	BodyText          string  `json:"body_text"`
	Status            string  `json:"status"`
	FailureReason     *string `json:"failure_reason,omitempty"`
	AttemptNumber     int     `json:"attempt_number"`
	ResentFromID      *string `json:"resent_from_id,omitempty"`
	CreatedAt         string  `json:"created_at"`
}

type InsertInput struct {
	QueueJobID        *string
	TemplateKey       *string
	TriggerCondition  string
	RelatedEntityType *string
	RelatedEntityID   *string
	Recipient         string
	FromEmail         string
	FromName          string
	ReplyTo           string
	Subject           string
	BodyHTML          string
	BodyText          string
	Status            string
	FailureReason     string
	AttemptNumber     int
	ResentFromID      *string
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

var ErrNotFound = errors.New("smtplog: row not found")

func (s *Store) Insert(ctx context.Context, in InsertInput) (string, error) {
	if in.AttemptNumber <= 0 {
		in.AttemptNumber = 1
	}
	var fromName, replyTo, failureReason any
	if in.FromName != "" {
		fromName = in.FromName
	}
	if in.ReplyTo != "" {
		replyTo = in.ReplyTo
	}
	if in.FailureReason != "" {
		failureReason = in.FailureReason
	}
	var id string
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO smtp_log
		   (queue_job_id, template_key, trigger_condition, related_entity_type, related_entity_id,
		    recipient, from_email, from_name, reply_to, subject, body_html, body_text,
		    status, failure_reason, attempt_number, resent_from_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		 RETURNING id`,
		in.QueueJobID, in.TemplateKey, in.TriggerCondition, in.RelatedEntityType, in.RelatedEntityID,
		in.Recipient, in.FromEmail, fromName, replyTo, in.Subject, in.BodyHTML, in.BodyText,
		in.Status, failureReason, in.AttemptNumber, in.ResentFromID,
	).Scan(&id)
	return id, err
}

type ListFilter struct {
	Status           string
	TemplateKey      string
	TriggerCondition string
	Recipient        string
	From             string
	To               string
	Limit            int
	Offset           int
}

func (s *Store) List(ctx context.Context, f ListFilter) ([]Row, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	args := []any{}
	where := []string{"TRUE"}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if f.Status != "" {
		add("status = ?", f.Status)
	}
	if f.TemplateKey != "" {
		add("template_key = ?", f.TemplateKey)
	}
	if f.TriggerCondition != "" {
		add("trigger_condition = ?", f.TriggerCondition)
	}
	if f.Recipient != "" {
		add("recipient ILIKE '%' || ? || '%'", f.Recipient)
	}
	if f.From != "" {
		add("created_at >= ?", f.From)
	}
	if f.To != "" {
		add("created_at <= ?", f.To)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM smtp_log WHERE `+whereSQL, args...).
		Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, f.Limit, f.Offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	q := `SELECT id, queue_job_id, template_key, trigger_condition, related_entity_type, related_entity_id,
	             recipient, from_email, from_name, reply_to, subject, body_html, body_text,
	             status, failure_reason, attempt_number, resent_from_id, created_at
	        FROM smtp_log
	       WHERE ` + whereSQL + `
	       ORDER BY created_at DESC
	       LIMIT $` + strconv.Itoa(limitIdx) + ` OFFSET $` + strconv.Itoa(offsetIdx)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Row, 0)
	for rows.Next() {
		var r Row
		if err := rows.Scan(&r.ID, &r.QueueJobID, &r.TemplateKey, &r.TriggerCondition,
			&r.RelatedEntityType, &r.RelatedEntityID, &r.Recipient, &r.FromEmail,
			&r.FromName, &r.ReplyTo, &r.Subject, &r.BodyHTML, &r.BodyText,
			&r.Status, &r.FailureReason, &r.AttemptNumber, &r.ResentFromID, &r.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (*Row, error) {
	var r Row
	err := s.db.QueryRowContext(ctx,
		`SELECT id, queue_job_id, template_key, trigger_condition, related_entity_type, related_entity_id,
		        recipient, from_email, from_name, reply_to, subject, body_html, body_text,
		        status, failure_reason, attempt_number, resent_from_id, created_at
		   FROM smtp_log WHERE id=$1`, id).
		Scan(&r.ID, &r.QueueJobID, &r.TemplateKey, &r.TriggerCondition,
			&r.RelatedEntityType, &r.RelatedEntityID, &r.Recipient, &r.FromEmail,
			&r.FromName, &r.ReplyTo, &r.Subject, &r.BodyHTML, &r.BodyText,
			&r.Status, &r.FailureReason, &r.AttemptNumber, &r.ResentFromID, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
