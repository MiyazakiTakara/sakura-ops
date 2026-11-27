package licensecheck

import (
	"fmt"
	"strings"
)

// Verdict of a single licence identifier or of an expression.
type Verdict int

const (
	Allow    Verdict = iota // explicitly allowlisted
	Denied                  // on the deny list (always wins over the allowlist)
	Unlisted                // neither allowed nor denied, including unknown or NOASSERTION
)

func (v Verdict) String() string {
	switch v {
	case Allow:
		return "allowed"
	case Denied:
		return "denied"
	default:
		return "not on the allowlist"
	}
}

// classify evaluates one identifier. A trailing "+" ("or later") is not stripped:
// GPL-2.0+ is still denied by the GPL- prefix, and an "X+" form of an
// allowlisted X is Unlisted, so the allowlist is never silently widened.
func (p *Policy) classify(id string) Verdict {
	l := strings.ToLower(strings.TrimSpace(id))
	if l == "" {
		return Unlisted
	}
	if p.denied[l] || p.deniedByPrefix(l) {
		return Denied
	}
	if p.allowed[l] {
		return Allow
	}
	return Unlisted
}

// EvalExpression evaluates an SPDX licence expression. OR passes if any
// operand passes, AND requires every operand to pass. "X WITH Y" is treated as
// one identifier that must itself be allowlisted verbatim. If an OR fails, the
// worst operand verdict is reported (Denied over Unlisted).
func (p *Policy) EvalExpression(expr string) (Verdict, error) {
	toks, err := tokenize(expr)
	if err != nil {
		return Unlisted, err
	}
	ps := &parser{p: p, toks: toks}
	v, err := ps.parseOr()
	if err != nil {
		return Unlisted, err
	}
	if ps.pos != len(ps.toks) {
		return Unlisted, fmt.Errorf("unexpected %q in licence expression %q", ps.toks[ps.pos], expr)
	}
	return v, nil
}

func tokenize(s string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			toks = append(toks, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r == '(' || r == ')':
			flush()
			toks = append(toks, string(r))
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	if len(toks) == 0 {
		return nil, fmt.Errorf("empty licence expression")
	}
	return toks, nil
}

type parser struct {
	p    *Policy
	toks []string
	pos  int
}

func (ps *parser) peek() string {
	if ps.pos < len(ps.toks) {
		return ps.toks[ps.pos]
	}
	return ""
}

func isOp(t string, op string) bool { return strings.EqualFold(t, op) }

func (ps *parser) parseOr() (Verdict, error) {
	v, err := ps.parseAnd()
	if err != nil {
		return Unlisted, err
	}
	for isOp(ps.peek(), "OR") {
		ps.pos++
		r, err := ps.parseAnd()
		if err != nil {
			return Unlisted, err
		}
		v = orVerdict(v, r)
	}
	return v, nil
}

func (ps *parser) parseAnd() (Verdict, error) {
	v, err := ps.parseAtom()
	if err != nil {
		return Unlisted, err
	}
	for isOp(ps.peek(), "AND") {
		ps.pos++
		r, err := ps.parseAtom()
		if err != nil {
			return Unlisted, err
		}
		v = andVerdict(v, r)
	}
	return v, nil
}

func (ps *parser) parseAtom() (Verdict, error) {
	t := ps.peek()
	switch {
	case t == "":
		return Unlisted, fmt.Errorf("licence expression ends unexpectedly")
	case t == "(":
		ps.pos++
		v, err := ps.parseOr()
		if err != nil {
			return Unlisted, err
		}
		if ps.peek() != ")" {
			return Unlisted, fmt.Errorf("missing ) in licence expression")
		}
		ps.pos++
		return v, nil
	case t == ")" || isOp(t, "AND") || isOp(t, "OR") || isOp(t, "WITH"):
		return Unlisted, fmt.Errorf("unexpected %q in licence expression", t)
	}
	ps.pos++
	id := t
	if isOp(ps.peek(), "WITH") {
		ps.pos++
		exc := ps.peek()
		if exc == "" || exc == "(" || exc == ")" || isOp(exc, "AND") || isOp(exc, "OR") {
			return Unlisted, fmt.Errorf("WITH needs an exception identifier after %q", id)
		}
		ps.pos++
		// The base licence must not be denied, and the pair must be allowlisted verbatim.
		if ps.p.classify(id) == Denied {
			return Denied, nil
		}
		return ps.p.classify(id + " WITH " + exc), nil
	}
	return ps.p.classify(id), nil
}

// orVerdict: any Allow wins; otherwise Denied is reported over Unlisted.
func orVerdict(a, b Verdict) Verdict {
	if a == Allow || b == Allow {
		return Allow
	}
	if a == Denied || b == Denied {
		return Denied
	}
	return Unlisted
}

// andVerdict: all must be Allow; otherwise Denied is reported over Unlisted.
func andVerdict(a, b Verdict) Verdict {
	if a == Denied || b == Denied {
		return Denied
	}
	if a == Unlisted || b == Unlisted {
		return Unlisted
	}
	return Allow
}
