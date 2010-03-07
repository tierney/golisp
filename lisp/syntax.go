package lisp

import (
	"io"
	"os"
	"fmt"
	"strings"
	"strconv"
	"./lexer"
	"./peg"
)

const (
	_DOT peg.Terminal = iota
	_LSTART
	_LEND
	_LSTART2
	_LEND2
	_INT
	_FLOAT
	_STR
	_COMMENT
	_WS
	_QUOTE
	_SYMBOL
	_HASH
)

var lex = lexer.RegexSet {
	int(_DOT):			"\\.",
	int(_LSTART): 		"\\(",
	int(_LEND):			"\\)",
	int(_LSTART2): 		"\\[",
	int(_LEND2):		"\\]",
	int(_INT):			"\\d+",
	int(_FLOAT):		"\\d+(\\.\\d+)?",
	int(_STR):			"\"([^\"]|\\.)*\"",
	int(_COMMENT):		";[^\n]*",
	int(_WS):			"\\s+",
	int(_QUOTE):		"'|`|,|,@",
	int(_SYMBOL):		"[^#\\(\\)\"\n\r\t\\[\\]'`,@ ]+",
	int(_HASH):			"#.",
}

func listExpr(start, rec, end peg.Expr) peg.Expr {
	tail := peg.Bind(
		peg.Option(peg.Select(peg.And { _DOT, rec }, 1)),
		func(x interface{}) interface{} {
			o := x.([]interface{})
			if len(o) != 0 {
				return o[0]
			}
			return EMPTY_LIST
		},
	)
	inner := peg.Bind(
		peg.Option(peg.And { peg.Multi(rec), tail }),
		func(x interface{}) interface{} {
			o := x.([]interface{})
			if len(o) == 0 {
				return EMPTY_LIST
			}
			expr := o[0].([]interface{})
			ls := expr[0].([]interface{})
			res := expr[1]
			if Failed(res) { return res }
			for i := len(ls) - 1; i >= 0; i-- {
				x := ls[i]
				if Failed(x) { return x }
				res = Cons(x, res)
			}
			return res
		},
	)
	return peg.Select(peg.And { start, inner, end }, 1)
}

var syntax = func() *peg.ExtensibleExpr {
	expr := peg.Extensible()
	expr.Add(peg.Or {
		peg.Bind(_INT, func(x interface{}) interface{} { 
			res, err := strconv.Atoi(x.(string))
			if err != nil { return SystemError(err) }
			return res
		}),
		peg.Bind(_FLOAT, func(x interface{}) interface{} { 
			res, err := strconv.Atof(x.(string))
			if err != nil { return SystemError(err) }
			return res
		}),
		peg.Bind(_STR, func(x interface{}) interface{} { 
			res, err := strconv.Unquote(x.(string))
			if err != nil { return SystemError(err) }
			return res
		}),
		listExpr(_LSTART, expr, _LEND),
		listExpr(_LSTART2, expr, _LEND2),
		peg.Bind(peg.And { _QUOTE, expr }, func(x interface{}) interface{} {
			qu := x.([]interface{})
			switch qu[0].(string) {
				case "'": return List(Symbol("quote"), qu[1])
				case "`": return List(Symbol("quasiquote"), qu[1])
				case ",": return List(Symbol("unquote"), qu[1])
			}
			return List(Symbol("unquote-splicing"), qu[1])
		}),
		peg.Bind(_SYMBOL, func(x interface{}) interface{} {
			return Symbol(x.(string))
		}),
		peg.Bind(_HASH, func(x interface{}) interface{} {
			s := x.(string)
			switch s[1] {
				case 'v': return nil
				case 'f': return false
				case 't': return true
			}
			return SyntaxError("unknown hash syntax: " + s)
		}),
	})
	return expr
}()

func ReadLine(port Any) (string, os.Error) {
	pt, ok := port.(io.Reader)
	if !ok { return "", TypeError("input-port", pt) }
	buf := []byte{0}
	res := ""
	for {
		_, err := pt.Read(buf)
		if err == os.EOF { return "", EOF_OBJECT.(os.Error) }
		if err != nil { return "", SystemError(err) }
		if buf[0] == '\n' { break }
		res += string(buf)
	}
	return res, nil
}

func Read(port Any) Any {
	pt, ok := port.(io.Reader)
	if !ok { return TypeError("input-port", pt) }
	l := lexer.New()
	l.Regexes(nil, lex)
	src := peg.NewLex(pt, l, func(id int) bool { return id != int(_WS) && id != int(_COMMENT) })
	m, d := peg.Or { 
		syntax, 
		peg.Bind(peg.Eof, func(x interface{}) interface{} { return EOF_OBJECT }),
	}.Match(src)
	if m.Failed() { return Throw(Symbol("syntax-error"), "failed to parse") }
	return d	
}

func ReadString(s string) Any {
	return Read(strings.NewReader(s))
}

func toWrite(def string, obj Any) string {
	if obj == nil { return "#v" }
	if b, ok := obj.(bool); ok {
		if b {
			return "#t"
		} else {
			return "#f"
		}
	}
	return fmt.Sprintf(def, obj)
}

func Write(obj, port Any) Any {
	p, ok := port.(io.Writer)
	if !ok { return TypeError("output-port", port) }
	io.WriteString(p, toWrite("%#v", obj))
	return nil
}

func Display(obj, port Any) Any {
	p, ok := port.(io.Writer)
	if !ok { return TypeError("output-port", port) }
	io.WriteString(p, toWrite("%v", obj))
	return nil
}

