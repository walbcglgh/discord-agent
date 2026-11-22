package tools

import (
	"fmt"
	"math"
	"strings"
)

// Small recursive-descent parser: no eval, no arbitrary code execution.
type exprParser struct {
	s   string
	pos int
}

func evalExpr(input string) (float64, error) {
	s := strings.ReplaceAll(strings.TrimSpace(input), " ", "")
	if s == "" {
		return 0, fmt.Errorf("運算式為空")
	}
	if len(s) > 200 {
		return 0, fmt.Errorf("運算式過長")
	}
	p := &exprParser{s: s}
	v, err := p.expr()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos != len(p.s) {
		return 0, fmt.Errorf("無法解析的字元: %q", p.s[p.pos:])
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, fmt.Errorf("結果不是有效數字")
	}
	return v, nil
}

func (p *exprParser) skipSpace() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

func (p *exprParser) expr() (float64, error) {
	v, err := p.term()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) || (p.s[p.pos] != '+' && p.s[p.pos] != '-') {
			return v, nil
		}
		op := p.s[p.pos]
		p.pos++
		right, err := p.term()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			v += right
		} else {
			v -= right
		}
	}
}

func (p *exprParser) term() (float64, error) {
	v, err := p.factor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.s) || (p.s[p.pos] != '*' && p.s[p.pos] != '/') {
			return v, nil
		}
		op := p.s[p.pos]
		p.pos++
		right, err := p.factor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			v *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("除數不可為零")
			}
			v /= right
		}
	}
}

func (p *exprParser) factor() (float64, error) {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return 0, fmt.Errorf("運算式不完整")
	}
	ch := p.s[p.pos]
	if ch == '+' || ch == '-' {
		p.pos++
		v, err := p.factor()
		if err != nil {
			return 0, err
		}
		if ch == '-' {
			return -v, nil
		}
		return v, nil
	}
	if ch == '(' {
		p.pos++
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if p.pos >= len(p.s) || p.s[p.pos] != ')' {
			return 0, fmt.Errorf("缺少右括號")
		}
		p.pos++
		return v, nil
	}
	start := p.pos
	for p.pos < len(p.s) && ((p.s[p.pos] >= '0' && p.s[p.pos] <= '9') || p.s[p.pos] == '.') {
		p.pos++
	}
	if p.pos == start {
		return 0, fmt.Errorf("非預期字元 %q", ch)
	}
	var v float64
	if _, err := fmt.Sscan(p.s[start:p.pos], &v); err != nil {
		return 0, err
	}
	return v, nil
}
