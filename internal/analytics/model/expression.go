package model

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Expression struct {
	root  expressionNode
	refs  []string
	calls []string
}

func ParseExpression(input string) (Expression, error) {
	p := expressionParser{input: strings.TrimSpace(input)}
	if p.input == "" {
		return Expression{}, fmt.Errorf("expression is required")
	}
	root, err := p.parseAdditive()
	if err != nil {
		return Expression{}, err
	}
	p.skipSpace()
	if p.pos != len(p.input) {
		return Expression{}, fmt.Errorf("unexpected token at position %d", p.pos+1)
	}
	return Expression{root: root, refs: append([]string{}, p.refs...), calls: append([]string{}, p.calls...)}, nil
}

func (e Expression) References() []string {
	return append([]string{}, e.refs...)
}

func (e Expression) Functions() []string {
	return append([]string{}, e.calls...)
}

func (e Expression) SQL(resolve func(string) (string, error)) (string, error) {
	if e.root == nil {
		return "", fmt.Errorf("expression is not parsed")
	}
	return e.root.sql(resolve)
}

type expressionNode interface {
	sql(func(string) (string, error)) (string, error)
}

type expressionRef string

func (n expressionRef) sql(resolve func(string) (string, error)) (string, error) {
	return resolve(string(n))
}

type expressionNumber string

func (n expressionNumber) sql(_ func(string) (string, error)) (string, error) {
	return string(n), nil
}

type expressionUnary struct {
	op   byte
	node expressionNode
}

func (n expressionUnary) sql(resolve func(string) (string, error)) (string, error) {
	value, err := n.node.sql(resolve)
	if err != nil {
		return "", err
	}
	return "(" + string(n.op) + value + ")", nil
}

type expressionBinary struct {
	op          byte
	left, right expressionNode
}

func (n expressionBinary) sql(resolve func(string) (string, error)) (string, error) {
	left, err := n.left.sql(resolve)
	if err != nil {
		return "", err
	}
	right, err := n.right.sql(resolve)
	if err != nil {
		return "", err
	}
	return "(" + left + " " + string(n.op) + " " + right + ")", nil
}

type expressionCall struct {
	name string
	args []expressionNode
}

func (n expressionCall) sql(resolve func(string) (string, error)) (string, error) {
	args := make([]string, 0, len(n.args))
	for _, arg := range n.args {
		value, err := arg.sql(resolve)
		if err != nil {
			return "", err
		}
		args = append(args, value)
	}
	if n.name == "safe_divide" {
		return "(" + args[0] + " / NULLIF(" + args[1] + ", 0))", nil
	}
	return strings.ToUpper(n.name) + "(" + strings.Join(args, ", ") + ")", nil
}

type expressionParser struct {
	input     string
	pos       int
	refs      []string
	calls     []string
	seen      map[string]struct{}
	seenCalls map[string]struct{}
}

func (p *expressionParser) parseAdditive() (expressionNode, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if !p.peek('+') && !p.peek('-') {
			return left, nil
		}
		op := p.input[p.pos]
		p.pos++
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = expressionBinary{op: op, left: left, right: right}
	}
}

func (p *expressionParser) parseMultiplicative() (expressionNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if !p.peek('*') && !p.peek('/') {
			return left, nil
		}
		op := p.input[p.pos]
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = expressionBinary{op: op, left: left, right: right}
	}
}

func (p *expressionParser) parseUnary() (expressionNode, error) {
	p.skipSpace()
	if p.peek('+') || p.peek('-') {
		op := p.input[p.pos]
		p.pos++
		node, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return expressionUnary{op: op, node: node}, nil
	}
	return p.parsePrimary()
}

func (p *expressionParser) parsePrimary() (expressionNode, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of expression")
	}
	if p.peek('(') {
		p.pos++
		node, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.peek(')') {
			return nil, fmt.Errorf("missing closing parenthesis at position %d", p.pos+1)
		}
		p.pos++
		return node, nil
	}
	if strings.HasPrefix(p.input[p.pos:], "${") {
		return p.parseReference()
	}
	if isDigit(p.input[p.pos]) || p.input[p.pos] == '.' {
		return p.parseNumber()
	}
	if isIdentifierStart(rune(p.input[p.pos])) {
		return p.parseCall()
	}
	return nil, fmt.Errorf("unexpected character %q at position %d", p.input[p.pos], p.pos+1)
}

func (p *expressionParser) parseReference() (expressionNode, error) {
	start := p.pos + 2
	end := strings.IndexByte(p.input[start:], '}')
	if end < 0 {
		return nil, fmt.Errorf("unterminated reference at position %d", p.pos+1)
	}
	end += start
	ref := strings.TrimSpace(p.input[start:end])
	if ref == "" {
		return nil, fmt.Errorf("empty reference at position %d", p.pos+1)
	}
	for _, part := range strings.Split(ref, ".") {
		if err := validateSemanticIdentifier(part); err != nil {
			return nil, fmt.Errorf("invalid reference %q: %w", ref, err)
		}
	}
	p.pos = end + 1
	if p.seen == nil {
		p.seen = map[string]struct{}{}
	}
	if _, ok := p.seen[ref]; !ok {
		p.seen[ref] = struct{}{}
		p.refs = append(p.refs, ref)
	}
	return expressionRef(ref), nil
}

func (p *expressionParser) parseNumber() (expressionNode, error) {
	start := p.pos
	seenDot := false
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if isDigit(ch) {
			p.pos++
			continue
		}
		if ch == '.' && !seenDot {
			seenDot = true
			p.pos++
			continue
		}
		break
	}
	value := p.input[start:p.pos]
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		return nil, fmt.Errorf("invalid number %q", value)
	}
	return expressionNumber(value), nil
}

func (p *expressionParser) parseCall() (expressionNode, error) {
	start := p.pos
	for p.pos < len(p.input) && isIdentifierPart(rune(p.input[p.pos])) {
		p.pos++
	}
	name := strings.ToLower(p.input[start:p.pos])
	p.skipSpace()
	if !p.peek('(') {
		return nil, fmt.Errorf("bare identifier %q is not allowed; use ${%s}", name, name)
	}
	allowed := map[string][2]int{
		"coalesce":    {2, 8},
		"nullif":      {2, 2},
		"abs":         {1, 1},
		"round":       {1, 2},
		"safe_divide": {2, 2},
	}
	bounds, ok := allowed[name]
	if !ok {
		return nil, fmt.Errorf("unsupported expression function %q", name)
	}
	if p.seenCalls == nil {
		p.seenCalls = map[string]struct{}{}
	}
	if _, ok := p.seenCalls[name]; !ok {
		p.seenCalls[name] = struct{}{}
		p.calls = append(p.calls, name)
	}
	p.pos++
	args := []expressionNode{}
	for {
		p.skipSpace()
		if p.peek(')') {
			p.pos++
			break
		}
		arg, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		p.skipSpace()
		if p.peek(',') {
			p.pos++
			continue
		}
		if !p.peek(')') {
			return nil, fmt.Errorf("expected comma or closing parenthesis at position %d", p.pos+1)
		}
		p.pos++
		break
	}
	if len(args) < bounds[0] || len(args) > bounds[1] {
		return nil, fmt.Errorf("function %q expects %d..%d arguments", name, bounds[0], bounds[1])
	}
	return expressionCall{name: name, args: args}, nil
}

func (p *expressionParser) skipSpace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *expressionParser) peek(ch byte) bool {
	return p.pos < len(p.input) && p.input[p.pos] == ch
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func isIdentifierStart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

func isIdentifierPart(ch rune) bool {
	return isIdentifierStart(ch) || unicode.IsDigit(ch)
}
