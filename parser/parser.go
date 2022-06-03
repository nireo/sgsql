package parser

import "fmt"

type keyword string
type punct string
type tokenType uint

const (
	selectKey     keyword = "select"
	fromKeyword   keyword = "from"
	asKeyword     keyword = "as"
	tableKeyword  keyword = "table"
	createKeyword keyword = "create"
	insertKeyword keyword = "insert"
	intoKeyword   keyword = "into"
	valuesKeyword keyword = "values"
	intKeyword    keyword = "int"
	textKeyword   keyword = "text"

	semicolonPunctg  punct = ";"
	asteriskPunctg   punct = "*"
	commaPunctg      punct = ","
	leftparenPunctg  punct = "("
	rightparenPunctg punct = ")"

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
