package parser

import (
	"bufio"
	"container/list"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	gadparser "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/node"
	"github.com/gad-lang/gad/parser/source"
	"github.com/gad-lang/gad/token"

	giomtoken "github.com/gad-lang/giom/token"
)

// =============================================================================
// scanner — implements gad's ScannerInterface for giom template syntax
// =============================================================================

type scanner struct {
	file        *source.File
	reader      *bufio.Reader
	indentStack *list.List
	stash       *list.List

	state  int32
	buffer string

	offset        int // current byte offset within file
	line          int // current line number (0-based)
	col           int // current column
	lastTokenPos  source.Pos
	lastTokenSize int

	readRaw        bool
	mode           gadparser.ScanMode
	mixedDelimiter gadparser.MixedDelimiter
	errorHandler   []source.ScannerErrorHandler
}

// newScanner creates a scanner that reads from r using file for position tracking.
func newScanner(file *source.File, r io.Reader) *scanner {
	s := &scanner{
		file:        file,
		reader:      bufio.NewReader(r),
		indentStack: list.New(),
		stash:       list.New(),
		state:       giomtoken.ScnNewLine,
		line:        -1,
		col:         0,
		mixedDelimiter: gadparser.MixedDelimiter{
			Start: []rune("{"),
			End:   []rune("}"),
		},
	}
	return s
}

// =============================================================================
// ScannerInterface implementation
// =============================================================================

func (s *scanner) Scan() (t gadparser.PToken) {
	if s.readRaw {
		s.readRaw = false
		return s.NextRaw()
	}

	s.ensureBuffer()

	if stashed := s.stash.Front(); stashed != nil {
		tok := stashed.Value.(gadparser.PToken)
		s.stash.Remove(stashed)
		return tok
	}

	switch s.state {
	case giomtoken.ScnEOF:
		if outdent := s.indentStack.Back(); outdent != nil {
			s.indentStack.Remove(outdent)
			return s.newToken(giomtoken.Outdent, "", "")
		}
		return s.newToken(giomtoken.EOF, "", "")

	case giomtoken.ScnNewLine:
		s.state = giomtoken.ScnLine
		if tok := s.scanIndent(); tok.Valid() {
			return tok
		}
		return s.Scan()

	case giomtoken.ScnLine:
		if tok := s.scanExport(); tok.Valid() {
			return tok
		}
		if tok := s.scanGlobal(); tok.Valid() {
			return tok
		}
		if tok := s.scanFunc(); tok.Valid() {
			return tok
		}
		if tok := s.scanComp(); tok.Valid() {
			return tok
		}
		if tok := s.scanCompCall(); tok.Valid() {
			return tok
		}
		if tok := s.scanMatch(); tok.Valid() {
			return tok
		}
		if tok := s.scanCase(); tok.Valid() {
			return tok
		}
		if tok := s.scanDoctype(); tok.Valid() {
			return tok
		}
		if tok := s.scanCondition(); tok.Valid() {
			return tok
		}
		if tok := s.scanFor(); tok.Valid() {
			return tok
		}
		if tok := s.scanImportModule(); tok.Valid() {
			return tok
		}
		if tok := s.scanSlot(); tok.Valid() {
			return tok
		}
		if tok := s.scanSlotPass(); tok.Valid() {
			return tok
		}
		if tok := s.scanAssignment(); tok.Valid() {
			return tok
		}
		if tok := s.scanCode(); tok.Valid() {
			return tok
		}
		if tok := s.scanMCode(); tok.Valid() {
			return tok
		}
		if tok := s.scanTag(); tok.Valid() {
			return tok
		}
		if tok := s.scanId(); tok.Valid() {
			return tok
		}
		if tok := s.scanClassName(); tok.Valid() {
			return tok
		}
		if tok := s.scanAttribute(); tok.Valid() {
			return tok
		}
		if tok := s.scanComment(); tok.Valid() {
			return tok
		}
		if tok := s.scanText(); tok.Valid() {
			return tok
		}
	}

	return s.newToken(token.Illegal, "", "")
}

func (s *scanner) Mode() gadparser.ScanMode     { return s.mode }
func (s *scanner) SetMode(m gadparser.ScanMode) { s.mode = m }
func (s *scanner) SourceFile() *source.File     { return s.file }
func (s *scanner) Source() []byte               { return s.file.Data.Bytes() }

func (s *scanner) ErrorHandler(h ...source.ScannerErrorHandler) {
	s.errorHandler = append(s.errorHandler, h...)
}

func (s *scanner) GetMixedDelimiter() *gadparser.MixedDelimiter {
	return &s.mixedDelimiter
}

// =============================================================================
// Token construction
// =============================================================================

func (s *scanner) newToken(kind token.Token, literal, value string) gadparser.PToken {
	pt := gadparser.PToken{
		TokenLit: node.TokenLit{
			Pos:     s.lastTokenPos,
			Token:   kind,
			Literal: literal,
		},
	}
	if value != "" {
		pt.Set("value", value)
	}
	return pt
}

func (s *scanner) newTokenWithData(kind token.Token, literal string, data map[string]string) gadparser.PToken {
	pt := gadparser.PToken{
		TokenLit: node.TokenLit{
			Pos:     s.lastTokenPos,
			Token:   kind,
			Literal: literal,
		},
	}
	for k, v := range data {
		pt.Set(k, v)
	}
	return pt
}

// =============================================================================
// Indentation scanning
// =============================================================================

var rgxIndent = regexp.MustCompile(`^(\s+)`)

func (s *scanner) scanIndent() gadparser.PToken {
	if len(s.buffer) == 0 {
		s.consume(0)
		return s.newToken(giomtoken.Blank, "", "")
	}

	var head *list.Element
	for head = s.indentStack.Front(); head != nil; head = head.Next() {
		value := head.Value.(*regexp.Regexp)
		if match := value.FindString(s.buffer); len(match) != 0 {
			s.consume(len(match))
		} else {
			break
		}
	}

	newIndent := rgxIndent.FindString(s.buffer)

	if len(newIndent) != 0 && head == nil {
		s.indentStack.PushBack(regexp.MustCompile(regexp.QuoteMeta(newIndent)))
		s.consume(len(newIndent))
		return s.newToken(giomtoken.Indent, newIndent, newIndent)
	}

	if len(newIndent) == 0 && head != nil {
		for head != nil {
			next := head.Next()
			s.indentStack.Remove(head)
			if next == nil {
				return s.newToken(giomtoken.Outdent, "", "")
			} else {
				t := s.newToken(giomtoken.Outdent, "", "")
				s.stash.PushBack(t)
			}
			head = next
		}
	}

	if len(newIndent) != 0 && head != nil {
		panic("Mismatching indentation. Please use a coherent indent schema.")
	}

	return gadparser.PToken{}
}

func (s *scanner) Indentation() string {
	var b strings.Builder
	for e := s.indentStack.Front(); e != nil; e = e.Next() {
		b.WriteString(e.Value.(*regexp.Regexp).String())
	}
	return b.String()
}

// =============================================================================
// Scan methods — regex-based line matching
// =============================================================================

var rgxDoctype = regexp.MustCompile(`^(!!!|@doctype)\s*(.*)`)

func (s *scanner) scanDoctype() gadparser.PToken {
	if sm := rgxDoctype.FindStringSubmatch(s.buffer); len(sm) != 0 {
		val := sm[2]
		if val == "" {
			val = "html"
		}
		s.consume(len(sm[0]))
		return s.newToken(giomtoken.Doctype, sm[0], val)
	}
	return gadparser.PToken{}
}

var rgxIf = regexp.MustCompile(`^@if\s+(.+)$`)
var rgxElse = regexp.MustCompile(`^@else(\s*|\s+if\s+(.+))$`)

func (s *scanner) scanCondition() gadparser.PToken {
	if sm := rgxIf.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return s.newToken(giomtoken.If, sm[0], sm[1])
	}
	if sm := rgxElse.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		if strings.Contains(strings.TrimSpace(sm[0][4:]), "if") {
			return s.newToken(giomtoken.ElseIf, sm[0], sm[2])
		}
		return s.newToken(giomtoken.Else, sm[0], "")
	}
	return gadparser.PToken{}
}

var rgxFor = regexp.MustCompile(`^@for\s+(.+)$`)

func (s *scanner) scanFor() gadparser.PToken {
	if sm := rgxFor.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return s.newToken(giomtoken.For, sm[0], strings.TrimSpace(sm[1]))
	}
	return gadparser.PToken{}
}

var rgxAssignment = regexp.MustCompile(`^(\$[\w0-9\-_]*)?\s*([+-/*:]?)=\s*(.+)$`)

func (s *scanner) scanAssignment() gadparser.PToken {
	if sm := rgxAssignment.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		pt := s.newToken(giomtoken.Assignment, sm[0], sm[3])
		pt.Set("x", sm[1])
		pt.Set("op", sm[2])
		return pt
	}
	return gadparser.PToken{}
}

var rgxCode = regexp.MustCompile(`^\s*~\s+(.+)$`)

func (s *scanner) scanCode() gadparser.PToken {
	if sm := rgxCode.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		pt := s.newToken(giomtoken.Code, sm[0], "")
		pt.Set("values", []string{sm[1]})
		return pt
	}
	return gadparser.PToken{}
}

var rgxMCode = regexp.MustCompile(`^\s*~~\s*$`)

func (s *scanner) scanMCode() gadparser.PToken {
	if sm := rgxMCode.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		code := s.NextRawCode("~~")
		pt := s.newToken(giomtoken.Code, "", "")
		pt.Set("values", code)
		return pt
	}
	return gadparser.PToken{}
}

var rgxComment = regexp.MustCompile(`^\/\/(-)?\s*(.*)$`)

func (s *scanner) scanComment() gadparser.PToken {
	if sm := rgxComment.FindStringSubmatch(s.buffer); len(sm) != 0 {
		mode := "embed"
		if len(sm[1]) != 0 {
			mode = "silent"
		}
		s.consume(len(sm[0]))
		pt := s.newToken(giomtoken.Comment, sm[0], sm[2])
		pt.Set("mode", mode)
		return pt
	}
	return gadparser.PToken{}
}

var rgxId = regexp.MustCompile(`^#([\w-]+)(?:\s*\?\s*(.*)$)?`)

func (s *scanner) scanId() gadparser.PToken {
	if sm := rgxId.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		pt := s.newToken(giomtoken.Id, sm[0], sm[1])
		pt.Set("condition", sm[2])
		return pt
	}
	return gadparser.PToken{}
}

var rgxClassName = regexp.MustCompile(`^\.([\w-]+)(?:\s*\?\s*(.*)$)?`)

func (s *scanner) scanClassName() gadparser.PToken {
	if sm := rgxClassName.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		pt := s.newToken(giomtoken.ClassName, sm[0], sm[1])
		pt.Set("condition", sm[2])
		return pt
	}
	return gadparser.PToken{}
}

var rgxAttribute = regexp.MustCompile(`^\[([\w\-:@\.]+)\s*(?:=\s*(\"([^\"\\]*)\"|([^\]]+)))?\](?:\s*\?\s*(.*)$)?`)

func (s *scanner) scanAttribute() gadparser.PToken {
	if sm := rgxAttribute.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))

		if len(sm[3]) != 0 || sm[2] == "" {
			var flag string
			if sm[2] == "" {
				flag = "true"
			}
			pt := s.newToken(giomtoken.Attribute, sm[0], sm[1])
			pt.Set("content", sm[3])
			pt.Set("mode", "raw")
			pt.Set("condition", sm[5])
			pt.Set("flag", flag)
			return pt
		}

		if sm[2] != `""` {
			pt := s.newToken(giomtoken.Attribute, sm[0], sm[1])
			pt.Set("content", sm[4])
			pt.Set("mode", "expression")
			pt.Set("condition", sm[5])
			return pt
		}
	}
	return gadparser.PToken{}
}

var rgxImportModule = regexp.MustCompile(`^@import\s+("[0-9a-zA-Z_\-\. \/][0-9a-zA-Z_\-\. \/]*")(\s+as\s+([a-zA-Z$_]\w*))?$`)
var rgxImportDestructure = regexp.MustCompile(`^@import\s*\{([^}]*)\}\s+from\s+("[0-9a-zA-Z_\-\. \/][0-9a-zA-Z_\-\. \/]*")$`)

func (s *scanner) scanImportModule() gadparser.PToken {
	if strings.HasPrefix(s.buffer, "@import") {
		if sm := rgxImportDestructure.FindStringSubmatch(s.buffer); len(sm) != 0 {
			s.consume(len(sm[0]))
			pt := s.newToken(giomtoken.ImportModule, sm[0], sm[2])
			pt.Set("destructure", strings.TrimSpace(sm[1]))
			return pt
		}
		if sm := rgxImportModule.FindStringSubmatch(s.buffer); len(sm) != 0 {
			s.consume(len(sm[0]))
			pt := s.newToken(giomtoken.ImportModule, sm[0], sm[1])
			pt.Set("ident", sm[3])
			return pt
		}
	}
	return gadparser.PToken{}
}

var rgxSlot = regexp.MustCompile(`^@slot\s+([a-zA-Z_-]+\w*)(\((.*)\))?$`)

func (s *scanner) readBalanced(start int, open, close byte) (string, int, bool) {
	if start >= len(s.buffer) || s.buffer[start] != open {
		return "", start, false
	}
	depth := 0
	inString := byte(0)
	escaped := false
	for i := start; i < len(s.buffer); i++ {
		c := s.buffer[i]
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == inString {
				inString = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			inString = c
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s.buffer[start : i+1], i + 1, true
			}
		}
	}
	return "", start, false
}

func (s *scanner) scanSlot() gadparser.PToken {
	if strings.TrimSpace(s.buffer) == "@wrap" {
		line := s.buffer
		s.consume(len(line))
		return s.newToken(giomtoken.Wrap, line, "")
	}
	if !strings.HasPrefix(s.buffer, "@slot ") || strings.HasPrefix(s.buffer, "@slot #") {
		return gadparser.PToken{}
	}
	line := s.buffer
	i := len("@slot ")
	j := i
	for j < len(line) {
		c := line[j]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			j++
			continue
		}
		break
	}
	if j == i {
		return gadparser.PToken{}
	}
	name := line[i:j]
	rest := strings.TrimSpace(line[j:])
	args := ""
	consumed := j
	if rest != "" {
		if j >= len(line) || line[j] != '(' {
			return gadparser.PToken{}
		}
		balanced, end, ok := s.readBalanced(j, '(', ')')
		if !ok {
			return gadparser.PToken{}
		}
		args = balanced[1 : len(balanced)-1]
		if strings.TrimSpace(line[end:]) != "" {
			return gadparser.PToken{}
		}
		consumed = len(line)
	}
	lit := line[:consumed]
	s.consume(consumed)
	pt := s.newToken(giomtoken.Slot, lit, name)
	pt.Set("args", args)
	return pt
}

var rgxSlotPass = regexp.MustCompile(`^@slot\s+#(.+)$`)

func (s *scanner) scanSlotPass() gadparser.PToken {
	if sm := rgxSlotPass.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		pt := s.newToken(giomtoken.SlotPass, sm[0], sm[1])
		pt.Set("header", sm[1])
		return pt
	}
	return gadparser.PToken{}
}

var rgxTag = regexp.MustCompile(`^(\w[-:/\w]*)`)

func (s *scanner) scanTag() gadparser.PToken {
	if sm := rgxTag.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return s.newToken(giomtoken.Tag, sm[0], sm[1])
	}
	return gadparser.PToken{}
}

var rgxExport = regexp.MustCompile(`^@export\s+([a-zA-Z_]\w*)(\s*=\s*(.+))?$`)

func (s *scanner) scanExport() gadparser.PToken {
	if sm := rgxExport.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		pt := s.newToken(giomtoken.Export, sm[0], sm[1])
		pt.Set("name", sm[1])
		pt.Set("value", sm[3])
		return pt
	}
	return gadparser.PToken{}
}

var rgxGlobal = regexp.MustCompile(`^@global\s+(.+)$`)

func (s *scanner) scanGlobal() gadparser.PToken {
	if sm := rgxGlobal.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return s.newToken(giomtoken.Global, sm[0], sm[1])
	}
	return gadparser.PToken{}
}

var rgxFunc = regexp.MustCompile(`^@(export\s+)?func ([a-zA-Z_-]+\w*)(\((.*)\))?$`)

func (s *scanner) scanFunc() gadparser.PToken {
	line := s.buffer
	exported := false
	prefix := "@func "
	if strings.HasPrefix(line, "@export func ") {
		exported = true
		prefix = "@export func "
	} else if !strings.HasPrefix(line, prefix) {
		return gadparser.PToken{}
	}

	i := len(prefix)
	j := i
	for j < len(line) {
		c := line[j]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			j++
			continue
		}
		break
	}
	if j == i {
		return gadparser.PToken{}
	}
	name := line[i:j]
	rest := strings.TrimSpace(line[j:])
	args := ""
	consumed := j
	if rest != "" {
		if j >= len(line) || line[j] != '(' {
			return gadparser.PToken{}
		}
		balanced, end, ok := s.readBalanced(j, '(', ')')
		if !ok {
			return gadparser.PToken{}
		}
		args = balanced[1 : len(balanced)-1]
		if strings.TrimSpace(line[end:]) != "" {
			return gadparser.PToken{}
		}
		consumed = len(line)
	}
	lit := line[:consumed]
	s.consume(consumed)
	pt := s.newToken(giomtoken.Func, lit, name)
	pt.Set("args", args)
	pt.Set("exported", fmt.Sprint(exported))
	return pt
}

var rgxComp = regexp.MustCompile(`^@(export\s+)?comp ([a-zA-Z_-]+\w*)(\((.*)\))?$`)
var rgxMainComp = regexp.MustCompile(`^@main\s*(\((.*)\))?$`)

func (s *scanner) scanComp() gadparser.PToken {
	line := s.buffer
	if strings.HasPrefix(line, "@main") {
		rest := strings.TrimSpace(line[len("@main"):])
		args := ""
		consumed := len("@main")
		if rest != "" {
			if consumed >= len(line) || line[consumed] != ' ' {
				return gadparser.PToken{}
			}
			start := consumed
			for start < len(line) && line[start] == ' ' {
				start++
			}
			if start >= len(line) || line[start] != '(' {
				return gadparser.PToken{}
			}
			balanced, end, ok := s.readBalanced(start, '(', ')')
			if !ok {
				return gadparser.PToken{}
			}
			args = balanced[1 : len(balanced)-1]
			if strings.TrimSpace(line[end:]) != "" {
				return gadparser.PToken{}
			}
			consumed = len(line)
		}
		lit := line[:consumed]
		s.consume(consumed)
		pt := s.newToken(giomtoken.Comp, lit, "main")
		pt.Set("args", args)
		pt.Set("exported", "true")
		return pt
	}

	exported := false
	prefix := "@comp "
	if strings.HasPrefix(line, "@export comp ") {
		exported = true
		prefix = "@export comp "
	} else if !strings.HasPrefix(line, prefix) {
		return gadparser.PToken{}
	}

	i := len(prefix)
	j := i
	for j < len(line) {
		c := line[j]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			j++
			continue
		}
		break
	}
	if j == i {
		return gadparser.PToken{}
	}
	name := line[i:j]
	rest := strings.TrimSpace(line[j:])
	args := ""
	consumed := j
	if rest != "" {
		if j >= len(line) || line[j] != '(' {
			return gadparser.PToken{}
		}
		balanced, end, ok := s.readBalanced(j, '(', ')')
		if !ok {
			return gadparser.PToken{}
		}
		args = balanced[1 : len(balanced)-1]
		if strings.TrimSpace(line[end:]) != "" {
			return gadparser.PToken{}
		}
		consumed = len(line)
	}
	lit := line[:consumed]
	s.consume(consumed)
	pt := s.newToken(giomtoken.Comp, lit, name)
	pt.Set("args", args)
	pt.Set("exported", fmt.Sprint(exported))
	return pt
}

var rgxMatch = regexp.MustCompile(`^@match\s+(\S+)\s*$`)

func (s *scanner) scanMatch() gadparser.PToken {
	if sm := rgxMatch.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return s.newToken(giomtoken.Match, sm[0], sm[1])
	}
	return gadparser.PToken{}
}

var rgxCase = regexp.MustCompile(`^@case\s+(.+)\s*$`)

func (s *scanner) scanCase() gadparser.PToken {
	if sm := rgxCase.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return s.newToken(giomtoken.Case, sm[0], sm[1])
	}
	return gadparser.PToken{}
}

var rgxCompCall = regexp.MustCompile(`^\+([@\$A-Za-z_-]+[.\w]*)(\((.*)\)\s*(~?))?$`)

func (s *scanner) scanCompCall() gadparser.PToken {
	if !strings.HasPrefix(s.buffer, "+") {
		return gadparser.PToken{}
	}
	line := s.buffer
	i := 1
	j := i
	for j < len(line) {
		c := line[j]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '$' || c == '@' || c == '.' {
			j++
			continue
		}
		break
	}
	if j == i {
		return gadparser.PToken{}
	}
	name := line[i:j]
	rest := strings.TrimSpace(line[j:])
	args := ""
	withCode := false
	consumed := j
	if rest != "" {
		if j >= len(line) || line[j] != '(' {
			return gadparser.PToken{}
		}
		balanced, end, ok := s.readBalanced(j, '(', ')')
		if !ok {
			return gadparser.PToken{}
		}
		args = balanced[1 : len(balanced)-1]
		tail := strings.TrimSpace(line[end:])
		switch tail {
		case "":
		case "~":
			withCode = true
		default:
			return gadparser.PToken{}
		}
		consumed = len(line)
	}
	lit := line[:consumed]
	s.consume(consumed)
	pt := s.newToken(giomtoken.CompCall, lit, name)
	pt.Set("args", args)
	pt.Set("withCode", fmt.Sprint(withCode))
	return pt
}

var rgxText = regexp.MustCompile(`^(\|)? ?(.*)$`)

func (s *scanner) scanText() gadparser.PToken {
	if sm := rgxText.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		mode := "inline"
		if sm[1] == "|" {
			mode = "piped"
		}
		pt := s.newToken(giomtoken.Text, sm[0], sm[2])
		pt.Set("mode", mode)
		return pt
	}
	return gadparser.PToken{}
}

// =============================================================================
// Raw text mode (for <script>, <style> content)
// =============================================================================

func (s *scanner) NextRaw() gadparser.PToken {
	result := ""
	level := 0

	for {
		s.ensureBuffer()

		switch s.state {
		case giomtoken.ScnEOF:
			pt := s.newToken(giomtoken.Text, result, result)
			pt.Set("mode", "raw")
			return pt

		case giomtoken.ScnNewLine:
			s.state = giomtoken.ScnLine

			if tok := s.scanIndent(); tok.Valid() {
				if tok.Token == giomtoken.Indent {
					level++
				} else if tok.Token == giomtoken.Outdent {
					level--
				} else {
					result += "\n"
					continue
				}

				if level < 0 {
					s.stash.PushBack(s.newToken(giomtoken.Outdent, "", ""))
					if len(result) > 0 && result[len(result)-1] == '\n' {
						result = result[:len(result)-1]
					}
					pt := s.newToken(giomtoken.Text, result, result)
					pt.Set("mode", "raw")
					return pt
				}
			}

		case giomtoken.ScnLine:
			if len(result) > 0 {
				result += "\n"
			}
			for i := 0; i < level; i++ {
				result += "\t"
			}
			result += s.buffer
			s.consume(len(s.buffer))
		}
	}
}

func (s *scanner) NextRawCode(eof string) (lines []string) {
	ind := s.Indentation()
	for {
		s.ensureBuffer()

		switch s.state {
		case giomtoken.ScnEOF:
			return
		case giomtoken.ScnNewLine:
			if s.buffer == ind {
				lines = append(lines, "")
				s.consume(len(s.buffer))
			} else {
				br := []rune(s.buffer)
				indr := []rune(ind)
				for len(indr) > 0 && len(br) > 0 {
					if br[0] == indr[0] {
						br = br[1:]
						indr = indr[1:]
					} else {
						break
					}
				}
				b := string(br)
				if b == eof {
					s.consume(len(s.buffer))
					return
				}
				lines = append(lines, b)
				s.consume(len(s.buffer))
			}
		}
	}
}

// =============================================================================
// Position tracking
// =============================================================================

func (s *scanner) consume(runes int) {
	if len(s.buffer) < runes {
		panic(fmt.Sprintf("Unable to consume %d runes from buffer.", runes))
	}
	s.lastTokenPos = source.Pos(s.file.Base + s.offset - len(s.buffer) - 1)
	s.lastTokenSize = runes
	s.buffer = s.buffer[runes:]
	s.col += runes
}

func (s *scanner) ensureBuffer() {
	if len(s.buffer) > 0 {
		return
	}

	buf, err := s.reader.ReadString('\n')
	s.offset += len(buf)

process:
	if err != nil && err != io.EOF {
		panic(err)
	} else if err != nil && len(buf) == 0 {
		s.state = giomtoken.ScnEOF
	} else {
		if len(buf) > 0 && buf[len(buf)-1] == '\n' {
			buf = buf[:len(buf)-1]
		}

		if lq := lineQuote(buf); lq >= 0 {
			var tmp string
			if tmp, err = s.reader.ReadString('\n'); err == nil || err == io.EOF {
				s.line++
				buf = buf[0:lq] + trimLeftSpace(tmp)
			}
			s.offset += len(buf)
			goto process
		}

		s.state = giomtoken.ScnNewLine
		s.buffer = buf
		s.line++
		s.col = 0
	}
}

func trimLeftSpace(s string) string {
	start := 0
	for ; start < len(s); start++ {
		c := s[start]
		if c >= utf8.RuneSelf {
			return strings.TrimFunc(s[start:], unicode.IsSpace)
		}
		if asciiSpace[c] == 0 {
			break
		}
	}
	return s[start:]
}

var asciiSpace = [256]uint8{'\t': 1, '\n': 1, '\v': 1, '\f': 1, '\r': 1, ' ': 1}

func lineQuote(s string) (start int) {
	l := len(s)
	if l == 0 {
		return -1
	}
	if s[l-1] == '\\' {
		return l - 1
	}
	return -1
}

// =============================================================================
// Compile-time interface check
// =============================================================================

var _ gadparser.ScannerInterface = (*scanner)(nil)
