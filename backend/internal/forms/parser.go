package forms

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ParseForm walks a CF7-style markup string and returns the canonical field
// list plus any parse errors. The pass is forgiving: malformed individual
// tags become errors but do not abort parsing — the admin can fix them one
// by one. Lines outside of `[...]` tags are ignored at this stage; the form
// renderer keeps them as labels/HTML in the editor preview.
//
// Supported phase-1 tag syntax:
//
//	[type    name attrs... "default"]              (regular field)
//	[type*   name attrs... "default"]              (required field)
//	[select  name attrs... "Apple" "Banana|banana"](pipe = label|value)
//	[checkbox/radio  name attrs... "Yes" "No"]
//	[submit "Send"]                                 (label as 1st quoted)
//
// Supported attrs: `id:foo`, `class:foo bar`, `size:30`, `maxlength:100`,
// `minlength:5`, `min:N`, `max:N`, `placeholder` (next quoted = text),
// `default` (next quoted = default value). Anything else is recorded in
// Raw and ignored during validation.
func ParseForm(markup string) ([]FormField, []ParseError) {
	var fields []FormField
	var errs []ParseError
	seen := make(map[string]bool)

	for _, tok := range scanTags(markup) {
		f, err := parseTag(tok.body, tok.start)
		if err != nil {
			errs = append(errs, *err)
			continue
		}
		// `submit` doesn't carry data so duplicate-name checks don't apply.
		if f.Type != FieldSubmit {
			if f.Name == "" {
				errs = append(errs, ParseError{
					Position: tok.start, Tag: tok.body,
					Message: "field name is required",
				})
				continue
			}
			if seen[f.Name] {
				errs = append(errs, ParseError{
					Position: tok.start, Tag: tok.body,
					Message: fmt.Sprintf("duplicate field name %q", f.Name),
				})
				continue
			}
			seen[f.Name] = true
		}
		fields = append(fields, *f)
	}
	return fields, errs
}

// rawTag is the bracketed body (without the brackets) and its byte offset
// inside the original markup.
type rawTag struct {
	body  string
	start int // byte offset of the opening `[`
}

// scanTags pulls every `[...]` that isn't escaped (`\[`). Tags inside
// fenced code blocks are NOT special-cased — admins are expected to drop
// raw CF7 syntax into the form markup, not arbitrary docs.
func scanTags(src string) []rawTag {
	var out []rawTag
	i := 0
	for i < len(src) {
		c := src[i]
		if c == '[' {
			if i > 0 && src[i-1] == '\\' {
				i++
				continue
			}
			// `[/...]` is a closing tag — CF7 has no closers; skip.
			if i+1 < len(src) && src[i+1] == '/' {
				i++
				continue
			}
			j := strings.IndexByte(src[i+1:], ']')
			if j == -1 {
				// Unterminated — bail; the rest is plain text.
				break
			}
			end := i + 1 + j
			body := src[i+1 : end]
			body = strings.TrimSpace(body)
			if body != "" {
				out = append(out, rawTag{body: body, start: i})
			}
			i = end + 1
			continue
		}
		i++
	}
	return out
}

func parseTag(body string, pos int) (*FormField, *ParseError) {
	toks, perr := tokenizeTag(body, pos)
	if perr != nil {
		return nil, perr
	}
	if len(toks) == 0 {
		return nil, &ParseError{Position: pos, Tag: body, Message: "empty tag"}
	}

	head := toks[0]
	required := false
	typeName := head.text
	if strings.HasSuffix(typeName, "*") {
		required = true
		typeName = strings.TrimSuffix(typeName, "*")
	}
	typeName = strings.ToLower(typeName)

	ft, ok := SupportedTypes[typeName]
	if !ok {
		return nil, &ParseError{
			Position: pos, Tag: body,
			Message: fmt.Sprintf("unsupported field type %q", typeName),
		}
	}

	f := FormField{Type: ft, Raw: "[" + body + "]"}
	if required {
		f.Required = true
	}

	// `submit` takes one quoted label and no name. Everything else expects a
	// field name as the next bare token.
	idx := 1
	if ft == FieldSubmit {
		if idx < len(toks) && toks[idx].quoted {
			f.Label = toks[idx].text
			idx++
		}
		// Drop any remaining tokens silently — phase 1 ignores extra attrs
		// on submit.
		return &f, nil
	}

	if idx >= len(toks) {
		return nil, &ParseError{Position: pos, Tag: body, Message: "field name is required"}
	}
	if toks[idx].quoted {
		return nil, &ParseError{
			Position: pos, Tag: body,
			Message: "field name must be the first token after the type (got a quoted string)",
		}
	}
	name := toks[idx].text
	if !isValidFieldName(name) {
		return nil, &ParseError{
			Position: pos, Tag: body,
			Message: fmt.Sprintf("invalid field name %q (use letters, digits, underscore, hyphen)", name),
		}
	}
	f.Name = name
	idx++

	// Walk remaining tokens applying attr semantics. Quoted strings either
	// follow a flag like `placeholder` / `default` (consumed there) or accumulate
	// as options for choice fields / default for plain inputs.
	for idx < len(toks) {
		t := toks[idx]
		idx++

		if t.quoted {
			// First trailing quoted on a non-choice field is taken as the
			// default value. Additional ones are silently dropped to match
			// CF7's behaviour.
			if isChoiceField(ft) {
				f.Options = append(f.Options, parseOption(t.text))
				continue
			}
			if f.Default == "" {
				f.Default = t.text
			}
			continue
		}

		// Bare token: either `key:value` attr, or a flag like `placeholder`
		// whose value is the next quoted token, or an unrecognised flag we
		// drop.
		if k, v, ok := splitKV(t.text); ok {
			applyKV(&f, k, v)
			continue
		}

		switch strings.ToLower(t.text) {
		case "placeholder":
			if idx < len(toks) && toks[idx].quoted {
				f.Placeholder = toks[idx].text
				idx++
			}
		case "default":
			if idx < len(toks) && toks[idx].quoted {
				f.Default = toks[idx].text
				idx++
			}
		case "required":
			f.Required = true
		default:
			// Unknown flag — ignore; admins will eventually see it in the
			// raw editor and can remove it.
		}
	}

	if f.Label == "" {
		f.Label = humanise(f.Name)
	}
	return &f, nil
}

func isChoiceField(t FieldType) bool {
	return t == FieldSelect || t == FieldCheckbox || t == FieldRadio
}

// parseOption splits "Label|value" into label and value. With no pipe, the
// value falls back to the label.
func parseOption(s string) FieldOption {
	if i := strings.IndexByte(s, '|'); i != -1 {
		return FieldOption{Label: s[:i], Value: s[i+1:]}
	}
	return FieldOption{Label: s, Value: s}
}

// tokenizeTag splits the tag body into bare words and quoted strings.
// Returns a ParseError if a quoted string is left unterminated.
func tokenizeTag(body string, pos int) ([]tagToken, *ParseError) {
	var out []tagToken
	i := 0
	for i < len(body) {
		c := body[i]
		if c == ' ' || c == '\t' || c == '\n' {
			i++
			continue
		}
		if c == '"' {
			j := strings.IndexByte(body[i+1:], '"')
			if j == -1 {
				return nil, &ParseError{
					Position: pos, Tag: body,
					Message: "unterminated quoted string",
				}
			}
			out = append(out, tagToken{text: body[i+1 : i+1+j], quoted: true})
			i = i + 1 + j + 1
			continue
		}
		// Bare token until whitespace or quote.
		start := i
		for i < len(body) {
			ch := body[i]
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '"' {
				break
			}
			i++
		}
		out = append(out, tagToken{text: body[start:i]})
	}
	return out, nil
}

type tagToken struct {
	text   string
	quoted bool
}

// splitKV recognises `key:value` (no spaces). `class:foo` is the canonical
// CF7 form; we accept it for any known key.
func splitKV(s string) (string, string, bool) {
	i := strings.IndexByte(s, ':')
	if i <= 0 || i == len(s)-1 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}

func applyKV(f *FormField, key, value string) {
	switch strings.ToLower(key) {
	case "id":
		f.ID = value
	case "class":
		if f.Class == "" {
			f.Class = value
		} else {
			f.Class = f.Class + " " + value
		}
	case "size":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			f.Size = n
		}
	case "maxlength":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			f.MaxLength = n
		}
	case "minlength":
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			f.MinLength = n
		}
	case "min":
		f.Min = value
	case "max":
		f.Max = value
	}
}

func isValidFieldName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
		case r == '_' || r == '-':
		default:
			return false
		}
		if i == 0 && unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// humanise turns "your-name" / "your_email" into "Your name" / "Your email"
// for use as the default field label.
func humanise(name string) string {
	s := strings.NewReplacer("-", " ", "_", " ").Replace(name)
	s = strings.TrimSpace(s)
	if s == "" {
		return name
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
