package database

import (
	"fmt"
	"strings"
)

const (
	modeNormal = iota
	modeLine
	modeQuote
	modeDollar
)

type parser struct {
	sql   string
	i     int
	mode  int
	b     strings.Builder
	stmts []string
}

func splitSQL(sql string) ([]string, error) {
	p := &parser{sql: sql}
	if err := p.run(); err != nil {
		return nil, err
	}
	return p.stmts, nil
}

func (p *parser) run() error {
	for p.i < len(p.sql) {
		if err := p.step(); err != nil {
			return err
		}
	}
	return p.finish()
}

func (p *parser) finish() error {
	if p.mode != modeNormal {
		return fmt.Errorf("migrate: unterminated literal")
	}
	p.flush()
	return nil
}

func (p *parser) step() error {
	if p.mode == modeLine {
		return p.stepLine()
	}
	if p.mode == modeQuote {
		return p.stepQuote()
	}
	if p.mode == modeDollar {
		return p.stepDollar()
	}
	return p.stepNormal()
}

func (p *parser) stepNormal() error {
	if p.hasPrefix("--") {
		p.mode = modeLine
		p.writePrefix(2)
		return nil
	}
	if p.hasPrefix("$$") {
		p.mode = modeDollar
		p.writePrefix(2)
		return nil
	}
	return p.stepPlain()
}

func (p *parser) stepPlain() error {
	if p.sql[p.i] == '\'' {
		p.mode = modeQuote
		p.writeByte()
		return nil
	}
	if p.sql[p.i] == ';' {
		p.flush()
		p.i++
		return nil
	}
	p.writeByte()
	return nil
}

func (p *parser) stepLine() error {
	ch := p.sql[p.i]
	p.writeByte()
	if ch == '\n' {
		p.mode = modeNormal
	}
	return nil
}

func (p *parser) stepQuote() error {
	if p.hasPrefix("''") {
		p.writePrefix(2)
		return nil
	}
	ch := p.sql[p.i]
	p.writeByte()
	if ch == '\'' {
		p.mode = modeNormal
	}
	return nil
}

func (p *parser) stepDollar() error {
	if p.hasPrefix("$$") {
		p.writePrefix(2)
		p.mode = modeNormal
		return nil
	}
	p.writeByte()
	return nil
}

func (p *parser) hasPrefix(prefix string) bool {
	return strings.HasPrefix(p.sql[p.i:], prefix)
}

func (p *parser) writeByte() {
	p.b.WriteByte(p.sql[p.i])
	p.i++
}

func (p *parser) writePrefix(n int) {
	p.b.WriteString(p.sql[p.i : p.i+n])
	p.i += n
}

func (p *parser) flush() {
	stmt := strings.TrimSpace(p.b.String())
	p.b.Reset()
	if stmt == "" {
		return
	}
	p.stmts = append(p.stmts, stmt)
}
