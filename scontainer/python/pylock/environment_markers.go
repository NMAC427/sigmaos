// https://packaging.python.org/en/latest/specifications/dependency-specifiers/#environment-markers

package pylock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type tokenType int

const (
	tokenVar tokenType = iota
	tokenStr
	tokenOp
	tokenAnd
	tokenOr
	tokenLParen
	tokenRParen
)

type token struct {
	typ tokenType
	val string
}

// ---------------- TOKENIZER ----------------

func tokenize(s string) ([]token, error) {
	s = strings.TrimSpace(s)
	toks := []token{}
	for len(s) > 0 {
		switch {
		case strings.HasPrefix(s, "("):
			toks = append(toks, token{tokenLParen, "("})
			s = s[1:]
		case strings.HasPrefix(s, ")"):
			toks = append(toks, token{tokenRParen, ")"})
			s = s[1:]
		case strings.HasPrefix(s, "and "):
			toks = append(toks, token{tokenAnd, "and"})
			s = s[3:]
		case strings.HasPrefix(s, "or "):
			toks = append(toks, token{tokenOr, "or"})
			s = s[2:]
		case strings.HasPrefix(s, "==") || strings.HasPrefix(s, "!=") ||
			strings.HasPrefix(s, "<=") || strings.HasPrefix(s, ">=") ||
			strings.HasPrefix(s, "<") || strings.HasPrefix(s, ">"):
			op := string(s[0])
			if len(s) > 1 && (s[1] == '=' || s[1] == '~') {
				op = s[:2]
				s = s[2:]
			} else {
				s = s[1:]
			}
			toks = append(toks, token{tokenOp, op})
		case s[0] == '"' || s[0] == '\'':
			q := s[0]
			s = s[1:]
			i := strings.IndexByte(s, q)
			if i == -1 {
				return nil, errors.New("unterminated string")
			}
			str := s[:i]
			toks = append(toks, token{tokenStr, str})
			s = s[i+1:]
		default:
			// variable or identifier
			end := strings.IndexFunc(s, func(r rune) bool {
				return r == ' ' || r == '(' || r == ')' || r == '"' || r == '\'' || r == '<' || r == '>' || r == '='
			})
			if end == -1 {
				end = len(s)
			}
			word := strings.TrimSpace(s[:end])
			if word != "" {
				toks = append(toks, token{tokenVar, word})
			}
			s = s[end:]
		}
		s = strings.TrimLeft(s, " \t")
	}
	return toks, nil
}

// ---------------- PARSER ----------------

type node interface{}

type binaryNode struct {
	op    string
	left  node
	right node
}

type valueNode struct {
	typ tokenType
	val string
}

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() *token {
	if p.pos >= len(p.toks) {
		return nil
	}
	return &p.toks[p.pos]
}

func (p *parser) consume() *token {
	if p.pos >= len(p.toks) {
		return nil
	}
	t := &p.toks[p.pos]
	p.pos++
	return t
}

// marker := orExpr
func (p *parser) parseExpr() (node, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() != nil && p.peek().typ == tokenOr {
		p.consume()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &binaryNode{"or", left, right}
	}
	return left, nil
}

func (p *parser) parseAnd() (node, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	for p.peek() != nil && p.peek().typ == tokenAnd {
		p.consume()
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		left = &binaryNode{"and", left, right}
	}
	return left, nil
}

func (p *parser) parseAtom() (node, error) {
	tok := p.peek()
	if tok == nil {
		return nil, errors.New("unexpected end")
	}
	switch tok.typ {
	case tokenLParen:
		p.consume()
		n, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek() == nil || p.peek().typ != tokenRParen {
			return nil, errors.New("missing closing parenthesis")
		}
		p.consume()
		return n, nil
	case tokenVar, tokenStr:
		left := p.consume()
		if p.peek() != nil && p.peek().typ == tokenOp {
			op := p.consume()
			right := p.consume()
			if right == nil {
				return nil, errors.New("missing right operand")
			}
			return &binaryNode{op.val, &valueNode{left.typ, left.val}, &valueNode{right.typ, right.val}}, nil
		}
		return &valueNode{left.typ, left.val}, nil
	default:
		return nil, fmt.Errorf("unexpected token %v", tok.val)
	}
}

// ---------------- EVALUATOR ----------------

func eval(n node, env map[string]string) (bool, error) {
	switch v := n.(type) {
	case *binaryNode:
		switch v.op {
		case "and":
			l, _ := eval(v.left, env)
			if !l {
				return false, nil
			}
			r, _ := eval(v.right, env)
			return r, nil
		case "or":
			l, _ := eval(v.left, env)
			if l {
				return true, nil
			}
			r, _ := eval(v.right, env)
			return r, nil
		case "==", "!=", "<", "<=", ">", ">=":
			leftVal := resolveValue(v.left, env)
			rightVal := resolveValue(v.right, env)
			return compare(leftVal, rightVal, v.op), nil
		default:
			return false, fmt.Errorf("unknown operator %q", v.op)
		}
	default:
		return false, fmt.Errorf("unexpected node type")
	}
}

func resolveValue(n node, env map[string]string) string {
	switch v := n.(type) {
	case *valueNode:
		if v.typ == tokenVar {
			if val, ok := env[v.val]; ok {
				return val
			}
			return ""
		}
		return v.val
	default:
		return ""
	}
}

func compare(a, b, op string) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	}
	// version comparison (lexical, numeric-aware)
	cmp := versionCompare(a, b)
	switch op {
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	default:
		return false
	}
}

func versionCompare(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		ai, bi := 0, 0
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
	}
	return 0
}

func LoadPythonEnvironmentMarkers(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]string
	err = json.Unmarshal(data, &m)
	return m, err
}

func EvaluateMarker(expr string, env map[string]string) (bool, error) {
	if expr == "" {
		return true, nil
	}

	toks, err := tokenize(expr)
	if err != nil {
		return false, err
	}
	p := parser{toks: toks}
	ast, err := p.parseExpr()
	if err != nil {
		return false, err
	}
	return eval(ast, env)
}
