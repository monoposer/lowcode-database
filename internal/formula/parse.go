package formula

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xuri/efp"
)

const colIdentPrefix = "__lc_"

// Node is an application-layer formula AST produced from efp tokens.
type Node interface {
	node()
}

type NumberNode struct{ Value float64 }
type StringNode struct{ Value string }
type BoolNode struct{ Value bool }
type RefNode struct{ Name string }
type UnaryNode struct {
	Op string
	X  Node
}
type BinaryNode struct {
	Op string
	L  Node
	R  Node
}
type CallNode struct {
	Name string
	Args []Node
}

func (NumberNode) node() {}
func (StringNode) node() {}
func (BoolNode) node()   {}
func (RefNode) node()    {}
func (UnaryNode) node()  {}
func (BinaryNode) node() {}
func (CallNode) node()   {}

// Parse tokenizes an Excel-style expression with efp and builds an AST.
// Column refs use {{name}} and are rewritten to Excel identifiers before parse.
func Parse(expr string) (Node, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}
	rewritten, err := rewriteColumnRefsToIdents(expr)
	if err != nil {
		return nil, err
	}
	rewritten = normalizeSingleQuotedStrings(rewritten)
	ps := efp.ExcelParser()
	toks := ps.Parse(rewritten)
	if len(toks) == 0 {
		return nil, fmt.Errorf("formula: empty token stream")
	}
	p := &tokParser{toks: toks}
	n, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if !p.done() {
		t := p.peek()
		return nil, fmt.Errorf("formula: unexpected token %q", t.TValue)
	}
	return n, nil
}

func rewriteColumnRefsToIdents(expr string) (string, error) {
	var err error
	out := columnRefRe.ReplaceAllStringFunc(expr, func(m string) string {
		if err != nil {
			return m
		}
		sub := columnRefRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			err = fmt.Errorf("invalid formula reference %q", m)
			return m
		}
		return colIdentPrefix + sub[1]
	})
	return out, err
}

func identToCol(s string) (string, bool) {
	if strings.HasPrefix(s, colIdentPrefix) {
		name := strings.TrimPrefix(s, colIdentPrefix)
		if name != "" {
			return name, true
		}
	}
	return "", false
}

func normalizeSingleQuotedStrings(expr string) string {
	var b strings.Builder
	i := 0
	for i < len(expr) {
		if i+1 < len(expr) && expr[i:i+2] == "{{" {
			if close := strings.Index(expr[i+2:], "}}"); close >= 0 {
				end := i + 2 + close + 2
				b.WriteString(expr[i:end])
				i = end
				continue
			}
		}
		if expr[i] == '\'' {
			j := i + 1
			for j < len(expr) && expr[j] != '\'' {
				j++
			}
			if j < len(expr) {
				b.WriteByte('"')
				b.WriteString(expr[i+1 : j])
				b.WriteByte('"')
				i = j + 1
				continue
			}
		}
		b.WriteByte(expr[i])
		i++
	}
	return b.String()
}

type tokParser struct {
	toks []efp.Token
	i    int
}

func (p *tokParser) done() bool { return p.i >= len(p.toks) }

func (p *tokParser) peek() efp.Token {
	if p.done() {
		return efp.Token{}
	}
	return p.toks[p.i]
}

func (p *tokParser) next() efp.Token {
	t := p.peek()
	if !p.done() {
		p.i++
	}
	return t
}

func (p *tokParser) parseExpr() (Node, error) { return p.parseCompare() }

func (p *tokParser) parseCompare() (Node, error) {
	n, err := p.parseConcat()
	if err != nil {
		return nil, err
	}
	for !p.done() {
		t := p.peek()
		if t.TType != efp.TokenTypeOperatorInfix {
			break
		}
		op := t.TValue
		switch op {
		case "=", "<>", "<", ">", "<=", ">=":
		default:
			return n, nil
		}
		p.next()
		r, err := p.parseConcat()
		if err != nil {
			return nil, err
		}
		n = BinaryNode{Op: op, L: n, R: r}
	}
	return n, nil
}

func (p *tokParser) parseConcat() (Node, error) {
	n, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for !p.done() && p.peek().TType == efp.TokenTypeOperatorInfix && p.peek().TValue == "&" {
		p.next()
		r, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		n = BinaryNode{Op: "&", L: n, R: r}
	}
	return n, nil
}

func (p *tokParser) parseAdd() (Node, error) {
	n, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for !p.done() && p.peek().TType == efp.TokenTypeOperatorInfix {
		op := p.peek().TValue
		if op != "+" && op != "-" {
			break
		}
		p.next()
		r, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		n = BinaryNode{Op: op, L: n, R: r}
	}
	return n, nil
}

func (p *tokParser) parseMul() (Node, error) {
	n, err := p.parsePow()
	if err != nil {
		return nil, err
	}
	for !p.done() && p.peek().TType == efp.TokenTypeOperatorInfix {
		op := p.peek().TValue
		if op != "*" && op != "/" {
			break
		}
		p.next()
		r, err := p.parsePow()
		if err != nil {
			return nil, err
		}
		n = BinaryNode{Op: op, L: n, R: r}
	}
	return n, nil
}

func (p *tokParser) parsePow() (Node, error) {
	n, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	if !p.done() && p.peek().TType == efp.TokenTypeOperatorInfix && p.peek().TValue == "^" {
		p.next()
		r, err := p.parsePow()
		if err != nil {
			return nil, err
		}
		n = BinaryNode{Op: "^", L: n, R: r}
	}
	return n, nil
}

func (p *tokParser) parseUnary() (Node, error) {
	if p.done() {
		return nil, fmt.Errorf("formula: unexpected end")
	}
	t := p.peek()
	if t.TType == efp.TokenTypeOperatorPrefix {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return UnaryNode{Op: t.TValue, X: x}, nil
	}
	return p.parsePostfix()
}

func (p *tokParser) parsePostfix() (Node, error) {
	n, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for !p.done() && p.peek().TType == efp.TokenTypeOperatorPostfix && p.peek().TValue == "%" {
		p.next()
		n = UnaryNode{Op: "%", X: n}
	}
	return n, nil
}

func (p *tokParser) parsePrimary() (Node, error) {
	if p.done() {
		return nil, fmt.Errorf("formula: unexpected end")
	}
	t := p.next()
	switch t.TType {
	case efp.TokenTypeOperand:
		return operandNode(t)
	case efp.TokenTypeFunction:
		if t.TSubType != efp.TokenSubTypeStart {
			return nil, fmt.Errorf("formula: unexpected function token %q", t.TValue)
		}
		args, err := p.parseCallArgs(efp.TokenTypeFunction)
		if err != nil {
			return nil, err
		}
		return CallNode{Name: strings.ToUpper(strings.TrimSpace(t.TValue)), Args: args}, nil
	case efp.TokenTypeSubexpression:
		if t.TSubType != efp.TokenSubTypeStart {
			return nil, fmt.Errorf("formula: unexpected parenthesis")
		}
		n, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.done() || p.peek().TType != efp.TokenTypeSubexpression || p.peek().TSubType != efp.TokenSubTypeStop {
			return nil, fmt.Errorf("formula: missing closing parenthesis")
		}
		p.next()
		return n, nil
	default:
		return nil, fmt.Errorf("formula: unexpected token %q <%s>", t.TValue, t.TType)
	}
}

func (p *tokParser) parseCallArgs(stopType string) ([]Node, error) {
	var args []Node
	if p.done() {
		return nil, fmt.Errorf("formula: unclosed function")
	}
	if p.peek().TType == stopType && p.peek().TSubType == efp.TokenSubTypeStop {
		p.next()
		return args, nil
	}
	for {
		n, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, n)
		if p.done() {
			return nil, fmt.Errorf("formula: unclosed function")
		}
		t := p.peek()
		if t.TType == efp.TokenTypeArgument {
			p.next()
			continue
		}
		if t.TType == stopType && t.TSubType == efp.TokenSubTypeStop {
			p.next()
			return args, nil
		}
		return nil, fmt.Errorf("formula: expected ',' or ')' in function, got %q", t.TValue)
	}
}

func operandNode(t efp.Token) (Node, error) {
	switch t.TSubType {
	case efp.TokenSubTypeNumber:
		v, err := strconv.ParseFloat(t.TValue, 64)
		if err != nil {
			return nil, fmt.Errorf("formula: number %q: %w", t.TValue, err)
		}
		return NumberNode{Value: v}, nil
	case efp.TokenSubTypeText:
		return StringNode{Value: t.TValue}, nil
	case efp.TokenSubTypeLogical:
		u := strings.ToUpper(t.TValue)
		return BoolNode{Value: u == "TRUE"}, nil
	case efp.TokenSubTypeRange, "":
		name := strings.TrimSpace(t.TValue)
		if col, ok := identToCol(name); ok {
			return RefNode{Name: col}, nil
		}
		if name == "" {
			return nil, fmt.Errorf("formula: empty operand")
		}
		return RefNode{Name: name}, nil
	case efp.TokenSubTypeError:
		return nil, fmt.Errorf("formula: %s", t.TValue)
	default:
		if col, ok := identToCol(t.TValue); ok {
			return RefNode{Name: col}, nil
		}
		return RefNode{Name: t.TValue}, nil
	}
}

func walkRefs(n Node, fn func(string)) {
	if n == nil {
		return
	}
	switch t := n.(type) {
	case RefNode:
		fn(t.Name)
	case UnaryNode:
		walkRefs(t.X, fn)
	case BinaryNode:
		walkRefs(t.L, fn)
		walkRefs(t.R, fn)
	case CallNode:
		for _, a := range t.Args {
			walkRefs(a, fn)
		}
	}
}

// Validate parses expr and checks {{refs}} against known column names (nil known = syntax only).
func Validate(expr string, known map[string]struct{}) error {
	n, err := Parse(expr)
	if err != nil {
		return err
	}
	if n == nil || known == nil {
		return nil
	}
	var first error
	seen := map[string]struct{}{}
	walkRefs(n, func(name string) {
		if first != nil {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		if _, ok := known[name]; !ok {
			first = fmt.Errorf("formula references unknown column %q", name)
		}
	})
	return first
}
