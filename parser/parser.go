package parser

import (
	"errors"
	"fmt"
	"strings"
)

type keyword string
type punct string
type tokenType uint

const (
	selectKeyword keyword = "select"
	whereKeyword  keyword = "where"
	fromKeyword   keyword = "from"
	asKeyword     keyword = "as"
	tableKeyword  keyword = "table"
	createKeyword keyword = "create"
	insertKeyword keyword = "insert"
	intoKeyword   keyword = "into"
	valuesKeyword keyword = "values"
	intKeyword    keyword = "int"
	textKeyword   keyword = "text"

	semicolonPunct  punct = ";"
	asteriskPunct   punct = "*"
	commaPunct      punct = ","
	leftparenPunct  punct = "("
	rightparenPunct punct = ")"

	keywordType tokenType = iota
	symbolType
	identifierType
	stringType
	numericType
)

type loc struct {
	line   uint
	column uint
}

type tok struct {
	value string
	tt    tokenType
	loc   loc
}

type cursor struct {
	ptr uint
	loc loc
}

func (t *tok) eq(rhs *tok) bool {
	return t.value == rhs.value && t.tt == rhs.tt
}

type lexer func(string, cursor) (*tok, cursor, bool)

func tokenize(src string) ([]*tok, error) {
	tokens := []*tok{}
	cur := cursor{}

lex:
	for cur.ptr < uint(len(src)) {
		lexers := []lexer{lexNum, lexString}

		for _, l := range lexers {
			if token, newcursor, ok := l(src, cur); ok {
				cur = newcursor
				if token != nil {
					tokens = append(tokens, token)
				}

				continue lex
			}
		}

		hint := ""
		if len(tokens) > 0 {
			hint = "after " + tokens[len(tokens)-1].value
		}
		return nil, fmt.Errorf(
			"unable to lex tokens%s, at %d:%d", hint, cur.loc.line, cur.loc.column)
	}

	return tokens, nil
}

func lexNum(src string, ic cursor) (*tok, cursor, bool) {
	cur := ic

	periodFound := false
	expMarkerFound := false

	for ; cur.ptr < uint(len(src)); cur.ptr++ {
		c := src[cur.ptr]
		cur.loc.column++

		isDigit := c >= '0' && c <= '9'
		isPeriod := c == '.'
		isExpMarker := c == 'e'

		// Must start with a digit or period
		if cur.ptr == ic.ptr {
			if !isDigit && !isPeriod {
				return nil, ic, false
			}

			periodFound = isPeriod
			continue
		}

		if isPeriod {
			if periodFound {
				return nil, ic, false
			}

			periodFound = true
			continue
		}

		if isExpMarker {
			if expMarkerFound {
				return nil, ic, false
			}

			// No periods allowed after expMarker
			periodFound = true
			expMarkerFound = true

			// expMarker must be followed by digits
			if cur.ptr == uint(len(src)-1) {
				return nil, ic, false
			}

			cNext := src[cur.ptr+1]
			if cNext == '-' || cNext == '+' {
				cur.ptr++
				cur.loc.column++
			}

			continue
		}

		if !isDigit {
			break
		}
	}

	// No characters accumulated
	if cur.ptr == ic.ptr {
		return nil, ic, false
	}

	return &tok{
		value: src[ic.ptr:cur.ptr],
		loc:   ic.loc,
		tt:    numericType,
	}, cur, true
}

func lexCharacterDelimited(src string, ic cursor, delimiter byte) (
	*tok, cursor, bool,
) {
	cur := ic
	if len(src[cur.ptr:]) == 0 {
		return nil, ic, false
	}

	if src[cur.ptr] != delimiter {
		return nil, ic, false
	}

	cur.loc.column++
	cur.ptr++

	var value []byte
	for ; cur.ptr < uint(len(src)); cur.ptr++ {
		c := src[cur.ptr]

		if c == delimiter {
			if cur.ptr+1 >= uint(len(src)) || src[cur.ptr+1] != delimiter {
				return &tok{
					value: string(value),
					loc:   ic.loc,
					tt:    stringType,
				}, cur, true
			} else {
				value = append(value, delimiter)
				cur.ptr++
				cur.loc.column++
			}
		}

		value = append(value, c)
		cur.loc.column++
	}

	return nil, ic, false
}

func lexString(src string, ic cursor) (*tok, cursor, bool) {
	return lexCharacterDelimited(src, ic, '\'')
}

func lexSymbol(src string, ic cursor) (*tok, cursor, bool) {
	c := src[ic.ptr]
	cur := ic
	// Will get overwritten later if not an ignored syntax
	cur.ptr++
	cur.loc.column++

	switch c {
	// Syntax that should be thrown away
	case '\n':
		cur.loc.line++
		cur.loc.column = 0
		fallthrough
	case '\t':
		fallthrough
	case ' ':
		return nil, cur, true
	}

	// Syntax that should be kept
	symbols := []punct{
		commaPunct,
		leftparenPunct,
		rightparenPunct,
		semicolonPunct,
		asteriskPunct,
	}

	var options []string
	for _, s := range symbols {
		options = append(options, string(s))
	}

	// Use `ic`, not `cur`
	match := longestMatch(src, ic, options)
	// Unknown character
	if match == "" {
		return nil, ic, false
	}

	cur.ptr = ic.ptr + uint(len(match))
	cur.loc.column = ic.loc.column + uint(len(match))

	return &tok{
		value: match,
		loc:   ic.loc,
		tt:    symbolType,
	}, cur, true
}

func lexKeyword(source string, ic cursor) (*tok, cursor, bool) {
	cur := ic
	keywords := []keyword{
		selectKeyword,
		insertKeyword,
		valuesKeyword,
		tableKeyword,
		createKeyword,
		whereKeyword,
		fromKeyword,
		intoKeyword,
		textKeyword,
	}

	var options []string
	for _, k := range keywords {
		options = append(options, string(k))
	}

	match := longestMatch(source, ic, options)
	if match == "" {
		return nil, ic, false
	}

	cur.ptr = ic.ptr + uint(len(match))
	cur.loc.column = ic.loc.column + uint(len(match))

	return &tok{
		value: match,
		tt:    keywordType,
		loc:   ic.loc,
	}, cur, true
}

func longestMatch(src string, ic cursor, options []string) string {
	var value []byte
	var skipList []int
	var match string

	cur := ic

	for cur.ptr < uint(len(src)) {
		value = append(value, strings.ToLower(string(src[cur.ptr]))...)
		cur.ptr++

	match:
		for i, option := range options {
			for _, skip := range skipList {
				if i == skip {
					continue match
				}
			}

			if option == string(value) {
				skipList = append(skipList, i)
				if len(option) > len(match) {
					match = option
				}

				continue
			}

			sharesPrefix := string(value) == option[:cur.ptr-ic.ptr]
			tooLong := len(value) > len(option)
			if tooLong || !sharesPrefix {
				skipList = append(skipList, i)
			}
		}

		if len(skipList) == len(options) {
			break
		}
	}

	return match
}

func lexIdentifier(src string, ic cursor) (*tok, cursor, bool) {
	if token, newCursor, ok := lexCharacterDelimited(src, ic, '"'); ok {
		return token, newCursor, true
	}

	cur := ic

	c := src[cur.ptr]
	isAlphabetical := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
	if !isAlphabetical {
		return nil, ic, false
	}
	cur.ptr++
	cur.loc.column++

	value := []byte{c}
	for ; cur.ptr < uint(len(src)); cur.ptr++ {
		c = src[cur.ptr]

		// Other characters count too, big ignoring non-ascii for now
		isAlphabetical := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
		isNumeric := c >= '0' && c <= '9'
		if isAlphabetical || isNumeric || c == '$' || c == '_' {
			value = append(value, c)
			cur.loc.column++
			continue
		}

		break
	}

	if len(value) == 0 {
		return nil, ic, false
	}

	return &tok{
		// Unquoted dentifiers are case-insensitive
		value: strings.ToLower(string(value)),
		loc:   ic.loc,
		tt:    identifierType,
	}, cur, true
}

type AST struct {
	Statements []*Statement
}

type ASTType uint

const (
	SelectType ASTType = iota
	CreateTableType
	InsertType
)

type Statement struct {
	SelectStatement      *SelectStatement
	CreateTableStatement *CreateTableStatement
	InsertStatement      *InsertStatement
	tt                   ASTType
}

type expressionType uint

const (
	literalType expressionType = iota
)

type expression struct {
	lit *tok
	tt  expressionType
}

type columnDefinition struct {
	name     tok
	datatype tok
}

type CreateTableStatement struct {
	name tok
	cols *[]*columnDefinition
}

type SelectStatement struct {
	item []*expression
	from tok
}

type InsertStatement struct {
	table  tok
	values *[]*expression
}

func tokenFromKeyword(k keyword) tok {
	return tok{
		tt:    keywordType,
		value: string(k),
	}
}

func tokenFromPunct(s punct) tok {
	return tok{
		tt:    symbolType,
		value: string(s),
	}
}

func expectToken(tokens []*tok, cursor uint, t tok) bool {
	if cursor >= uint(len(tokens)) {
		return false
	}

	return t.eq(tokens[cursor])
}

func helpMessage(tokens []*tok, cursor uint, msg string) {
	var c *tok
	if cursor < uint(len(tokens)) {
		c = tokens[cursor]
	} else {
		c = tokens[cursor-1]
	}

	fmt.Printf("[%d,%d]: %s, got: %s\n", c.loc.line, c.loc.column, msg, c.value)
}

func Parse(src string) (*AST, error) {
	tokens, err := tokenize(src)
	if err != nil {
		return nil, err
	}

	a := AST{}
	cursor := uint(0)
	for cursor < uint(len(tokens)) {
		stmt, newCursor, ok := parseStatement(tokens, cursor, tokenFromPunct(semicolonPunct))
		if !ok {
			helpMessage(tokens, cursor, "Expected statement")
			return nil, errors.New("Failed to parse, expected statement")
		}
		cursor = newCursor

		a.Statements = append(a.Statements, stmt)

		atLeastOneSemicolon := false
		for expectToken(tokens, cursor, tokenFromPunct(semicolonPunct)) {
			cursor++
			atLeastOneSemicolon = true
		}

		if !atLeastOneSemicolon {
			helpMessage(tokens, cursor, "Expected semi-colon delimiter between statements")
			return nil, errors.New("Missing semi-colon between statements")
		}
	}

	return &a, nil
}
