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

	gp "github.com/gad-lang/gad/parser"
	"github.com/gad-lang/gad/parser/source"
	gt "github.com/gad-lang/gad/token"
)

type Kind int8

func (k Kind) String() string {
	if k > 0 && k <= tokDefault {
		return tokNames[k]
	}
	return fmt.Sprintf("Kind(%d)", k)
}

const (
	tokEOF Kind = iota + 1
	tokDoctype
	tokComment
	tokIndent
	tokOutdent
	tokBlank
	tokId
	tokClassName
	tokTag
	tokText
	tokAttribute
	tokIf
	tokElseIf
	tokElse
	tokFor
	tokAssignment
	tokCode
	tokImportModule
	tokFunc
	tokSlot
	tokSlotPass
	tokComp
	tokCompCall
	tokSwitch
	tokCase
	tokDefault
	tokExport
)

var tokNames = [...]string{
	tokEOF:          "EOF",
	tokDoctype:      "DOCTYPE",
	tokComment:      "COMENT",
	tokIndent:       "INDENT",
	tokOutdent:      "OUTDENT",
	tokBlank:        "BLANK",
	tokId:           "ID",
	tokClassName:    "CLASS_NAME",
	tokTag:          "TAG",
	tokText:         "TEXT",
	tokAttribute:    "ATTRIBUTE",
	tokIf:           "IF",
	tokElseIf:       "ELSE_IF",
	tokElse:         "ELSE",
	tokFor:          "FOR",
	tokAssignment:   "ASSIGNMENT",
	tokCode:         "CODE",
	tokImportModule: "IMPORT_MODULE",
	tokFunc:         "FUNC",
	tokSlot:         "SLOT",
	tokSlotPass:     "SLOT_PASS",
	tokComp:         "COMP",
	tokCompCall:     "COMP_CALL",
	tokSwitch:       "SWITCH",
	tokCase:         "CASE",
	tokDefault:      "DEFAULT",
	tokExport:       "EXPORT",
}

const (
	scnNewLine = iota
	scnLine
	scnEOF
)

type scanner struct {
	reader      *bufio.Reader
	indentStack *list.List
	stash       *list.List

	state  int32
	buffer string

	curPos        int
	line          int
	col           int
	lastTokenLine int
	lastTokenCol  int
	lastTokenSize int

	readRaw bool
}

type token struct {
	Kind     Kind
	Value    string
	Data     map[string]string
	Values   []string
	AnyValue any
}

func newScanner(r io.Reader) *scanner {
	s := new(scanner)
	s.reader = bufio.NewReader(r)
	s.indentStack = list.New()
	s.stash = list.New()
	s.state = scnNewLine
	s.line = -1
	s.col = 0

	return s
}

func (s *scanner) Pos() SourcePosition {
	return SourcePosition{
		s.curPos,
		s.lastTokenLine + 1,
		s.lastTokenCol + 1,
		s.lastTokenSize,
		""}
}

// Returns next token found in buffer
func (s *scanner) Next() *token {
	if s.readRaw {
		s.readRaw = false
		return s.NextRaw()
	}

	s.ensureBuffer()

	if stashed := s.stash.Front(); stashed != nil {
		tok := stashed.Value.(*token)
		s.stash.Remove(stashed)
		return tok
	}

do:
	switch s.state {
	case scnEOF:
		if outdent := s.indentStack.Back(); outdent != nil {
			s.indentStack.Remove(outdent)
			return &token{Kind: tokOutdent}
		}

		return &token{Kind: tokEOF}
	case scnNewLine:
		s.state = scnLine

		if tok := s.scanIndent(); tok != nil {
			return tok
		}

		return s.Next()
	case scnLine:
		if tok := s.scanExit(); tok != nil {
			s.state = scnEOF
			goto do
		}

		if tok := s.scanExport(); tok != nil {
			return tok
		}

		if tok := s.scanFunc(); tok != nil {
			return tok
		}

		if tok := s.scanComp(); tok != nil {
			return tok
		}

		if tok := s.scanCompCall(); tok != nil {
			return tok
		}

		if tok := s.scanSwitch(); tok != nil {
			return tok
		}

		if tok := s.scanCase(); tok != nil {
			return tok
		}

		if tok := s.scanDefault(); tok != nil {
			return tok
		}

		if tok := s.scanDoctype(); tok != nil {
			return tok
		}

		if tok := s.scanCondition(); tok != nil {
			return tok
		}

		if tok := s.scanFor(); tok != nil {
			return tok
		}

		if tok := s.scanImportModule(); tok != nil {
			return tok
		}

		if tok := s.scanSlot(); tok != nil {
			return tok
		}

		if tok := s.scanSlotPass(); tok != nil {
			return tok
		}

		if tok := s.scanAssignment(); tok != nil {
			return tok
		}

		if tok := s.scanCode(); tok != nil {
			return tok
		}

		if tok := s.scanMCode(); tok != nil {
			return tok
		}

		if tok := s.scanTag(); tok != nil {
			return tok
		}

		if tok := s.scanId(); tok != nil {
			return tok
		}

		if tok := s.scanClassName(); tok != nil {
			return tok
		}

		if tok := s.scanAttribute(); tok != nil {
			return tok
		}

		if tok := s.scanComment(); tok != nil {
			return tok
		}

		if tok := s.scanText(); tok != nil {
			return tok
		}
	}

	return nil
}

func (s *scanner) Indentation() string {
	var b strings.Builder
	if s.indentStack != nil {
		for e := s.indentStack.Front(); e != nil; e = e.Next() {
			// do something with e.Value
			b.WriteString(e.Value.(*regexp.Regexp).String())
		}
	}
	return b.String()
}

func (s *scanner) NextRaw() *token {
	result := ""
	level := 0

	for {
		s.ensureBuffer()

		switch s.state {
		case scnEOF:
			return &token{Kind: tokText, Value: result, Data: map[string]string{"Mode": "raw"}}
		case scnNewLine:
			s.state = scnLine

			if tok := s.scanIndent(); tok != nil {
				if tok.Kind == tokIndent {
					level++
				} else if tok.Kind == tokOutdent {
					level--
				} else {
					result = result + "\n"
					continue
				}

				if level < 0 {
					s.stash.PushBack(&token{Kind: tokOutdent})

					if len(result) > 0 && result[len(result)-1] == '\n' {
						result = result[:len(result)-1]
					}

					return &token{Kind: tokText, Value: result, Data: map[string]string{"Mode": "raw"}}
				}
			}
		case scnLine:
			if len(result) > 0 {
				result = result + "\n"
			}
			for i := 0; i < level; i++ {
				result += "\t"
			}
			result = result + s.buffer
			s.consume(len(s.buffer))
		}
	}

	return nil
}

func (s *scanner) NextRawCode(eof string) (lines []string) {
	var (
		ind = s.Indentation()
	)
	for {
		s.ensureBuffer()

		switch s.state {
		case scnEOF:
			return
		case scnNewLine:
			if s.buffer == ind {
				lines = append(lines, "")
				s.consume(len(s.buffer))
			} else {
				var (
					br   = []rune(s.buffer)
					indr = []rune(ind)
				)
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
				} else {
					lines = append(lines, b)
					s.consume(len(s.buffer))
				}
			}
		}
	}
}

var rgxIndent = regexp.MustCompile(`^(\s+)`)

func (s *scanner) scanIndent() *token {
	if len(s.buffer) == 0 {
		return &token{Kind: tokBlank}
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
		return &token{Kind: tokIndent, Value: newIndent}
	}

	if len(newIndent) == 0 && head != nil {
		for head != nil {
			next := head.Next()
			s.indentStack.Remove(head)
			if next == nil {
				return &token{Kind: tokOutdent}
			} else {
				s.stash.PushBack(&token{Kind: tokOutdent})
			}
			head = next
		}
	}

	if len(newIndent) != 0 && head != nil {
		panic("Mismatching indentation. Please use a coherent indent schema.")
	}

	return nil
}

var rgxDoctype = regexp.MustCompile(`^(!!!|@doctype)\s*(.*)`)

func (s *scanner) scanDoctype() *token {
	if sm := rgxDoctype.FindStringSubmatch(s.buffer); len(sm) != 0 {
		if len(sm[2]) == 0 {
			sm[2] = "html"
		}

		s.consume(len(sm[0]))
		return &token{Kind: tokDoctype, Value: sm[2]}
	}

	return nil
}

var rgxIf = regexp.MustCompile(`^@if\s+(.+)$`)
var rgxElse = regexp.MustCompile(`^@else(\s*|\s+if\s+(.+))$`)

func (s *scanner) scanCondition() *token {
	if sm := rgxIf.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokIf, Value: sm[1]}
	}

	if sm := rgxElse.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		if strings.Contains(strings.TrimSpace(sm[0][4:]), "if") {
			return &token{Kind: tokElseIf, Value: sm[2]}
		}
		return &token{Kind: tokElse}
	}

	return nil
}

var rgxEach = regexp.MustCompile(`^@for\s+(.+)$`)

func (s *scanner) scanFor() *token {
	if sm := rgxEach.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokFor, Value: sm[0][1:]}
	}

	return nil
}

var rgxAssignment = regexp.MustCompile(`^(\$[\w0-9\-_]*)?\s*([+-/*:]?)=\s*(.+)$`)

func (s *scanner) scanAssignment() *token {
	if sm := rgxAssignment.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokAssignment, Value: sm[3], Data: map[string]string{"X": sm[1], "Op": sm[2]}}
	}

	return nil
}

var rgxCode = regexp.MustCompile(`^\s*~\s+(.+)$`)

func (s *scanner) scanCode() *token {
	if sm := rgxCode.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokCode, Values: []string{sm[1]}}
	}

	return nil
}

var rgxMCode = regexp.MustCompile(`^\s*~~\s*$`)

func (s *scanner) scanMCode() *token {
	if sm := rgxMCode.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		code := s.NextRawCode("~~")
		return &token{Kind: tokCode, Values: code}
	}

	return nil
}

var rgxComment = regexp.MustCompile(`^\/\/(-)?\s*(.*)$`)

func (s *scanner) scanComment() *token {
	if sm := rgxComment.FindStringSubmatch(s.buffer); len(sm) != 0 {
		mode := "embed"
		if len(sm[1]) != 0 {
			mode = "silent"
		}

		s.consume(len(sm[0]))
		return &token{Kind: tokComment, Value: sm[2], Data: map[string]string{"Mode": mode}}
	}

	return nil
}

var rgxId = regexp.MustCompile(`^#([\w-]+)(?:\s*\?\s*(.*)$)?`)

func (s *scanner) scanId() *token {
	if sm := rgxId.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokId, Value: sm[1], Data: map[string]string{"Condition": sm[2]}}
	}

	return nil
}

var rgxClassName = regexp.MustCompile(`^\.([\w-]+)(?:\s*\?\s*(.*)$)?`)

func (s *scanner) scanClassName() *token {
	if sm := rgxClassName.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokClassName, Value: sm[1], Data: map[string]string{"Condition": sm[2]}}
	}

	return nil
}

var rgxAttribute = regexp.MustCompile(`^\[([\w\-:@\.]+)\s*(?:=\s*(\"([^\"\\]*)\"|([^\]]+)))?\](?:\s*\?\s*(.*)$)?`)

func (s *scanner) scanAttribute() *token {
	if s.buffer[0] == '[' {
		fs := source.NewFileSet()
		sf := fs.AddFileData("-", -1, []byte(s.buffer))
		p := gp.NewParser(sf, nil)
		lbrack := p.Expect(gt.LBrack)
		ret := p.ParseKeyValueArrayLitAt(lbrack, gt.RBrack)
		if p.Errors.Err() == nil {
			s.consume(int(ret.RBrace))
			return &token{Kind: tokAttribute, AnyValue: ret.Elements}
		}

		// value := s.buffer[:i]
		// s.consume(len(value))
		// return &token{Kind: tokAttribute, Value: value, Data: map[string]string{"Content": sm[4], "Mode": "expression", "Condition": sm[5]}}
	}

	if sm := rgxAttribute.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))

		if len(sm[3]) != 0 || sm[2] == "" {
			var flag string
			if sm[2] == "" {
				flag = "true"
			}
			return &token{Kind: tokAttribute, Value: sm[1], Data: map[string]string{
				"Content":   sm[3],
				"Mode":      "raw",
				"Condition": sm[5],
				"Flag":      flag,
			}}
		}

		if sm[2] != `""` {
			return &token{Kind: tokAttribute, Value: sm[1], Data: map[string]string{"Content": sm[4], "Mode": "expression", "Condition": sm[5]}}
		}
	}

	return nil
}

var rgxImportModule = regexp.MustCompile(`^@import\s+("[0-9a-zA-Z_\-\. \/][0-9a-zA-Z_\-\. \/]*")(\s+as\s+([a-zA-Z$_]\w*))?$`)

func (s *scanner) scanImportModule() *token {
	if sm := rgxImportModule.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokImportModule, Value: sm[1], Data: map[string]string{
			"ident": sm[3],
		}}
	}

	return nil
}

var rgxSlot = regexp.MustCompile(`^@slot\s+([a-zA-Z_-]+\w*)(\((.*)\))?$`)

func (s *scanner) scanSlot() *token {
	if sm := rgxSlot.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokSlot, Value: sm[1], Data: map[string]string{"Args": sm[3]}}
	}

	return nil
}

var rgxSlotPass = regexp.MustCompile(`^@slot\s+#(.+)$`)

func (s *scanner) scanSlotPass() *token {
	if sm := rgxSlotPass.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokSlotPass, Value: sm[1], Data: map[string]string{"Header": sm[1]}}
	}

	return nil
}

var rgxInit = regexp.MustCompile(`^~~~\s*$`)

var rgxTag = regexp.MustCompile(`^(\w[-:/\w]*)`)

func (s *scanner) scanTag() *token {
	if sm := rgxTag.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokTag, Value: sm[1]}
	}

	return nil
}

var rgxExit = regexp.MustCompile(`^@return\s*?$`)

func (s *scanner) scanExit() *token {
	if sm := rgxExit.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: scnEOF}
	}

	return nil
}

var rgxExport = regexp.MustCompile(`^@export\s+([a-zA-Z_]\w*)(\s*=\s*(.+))?$`)

func (s *scanner) scanExport() *token {
	if sm := rgxExport.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokExport, Value: sm[1], Data: map[string]string{"Name": sm[1], "Value": sm[3]}}
	}

	return nil
}

var rgxFunc = regexp.MustCompile(`^@(export\s+)?func ([a-zA-Z_-]+\w*)(\((.*)\))?$`)

func (s *scanner) scanFunc() *token {
	if sm := rgxFunc.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokFunc, Value: sm[2], Data: map[string]string{"Args": sm[4], "Exported": fmt.Sprint(len(sm[1]) > 0)}}
	}
	return nil
}

var rgxComp = regexp.MustCompile(`^@(export\s+)?comp ([a-zA-Z_-]+\w*)(\((.*)\))?$`)
var rgxMainComp = regexp.MustCompile(`^@main\s*(\((.*)\))?$`)

func (s *scanner) scanComp() *token {
	if sm := rgxComp.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokComp, Value: sm[2], Data: map[string]string{"Args": sm[4], "Exported": fmt.Sprint(len(sm[1]) > 0)}}
	}
	if sm := rgxMainComp.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokComp, Value: "main", Data: map[string]string{"Args": sm[2], "Exported": "true"}}
	}
	return nil
}

var rgxSwitch = regexp.MustCompile(`^@switch\s+(\S+)\s*$`)

func (s *scanner) scanSwitch() *token {
	if sm := rgxSwitch.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokSwitch, Value: sm[1]}
	}

	return nil
}

var rgxCase = regexp.MustCompile(`^@case\s+(.+)\s*$`)

func (s *scanner) scanCase() *token {
	if sm := rgxCase.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokCase, Value: sm[1]}
	}

	return nil
}

var rgxDefault = regexp.MustCompile(`^@default\s*$`)

func (s *scanner) scanDefault() *token {
	if sm := rgxDefault.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{Kind: tokDefault}
	}

	return nil
}

var rgxCompCall = regexp.MustCompile(`^\+([A-Za-z_-]+[.\w]*)(\((.*)\)\s*(~?))?$`)

func (s *scanner) scanCompCall() *token {
	if sm := rgxCompCall.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))
		return &token{
			Kind:  tokCompCall,
			Value: sm[1],
			Data: map[string]string{
				"Args":     sm[3],
				"WithCode": fmt.Sprint(sm[4] == "~"),
			},
		}
	}

	return nil
}

var rgxText = regexp.MustCompile(`^(\|)? ?(.*)$`)

func (s *scanner) scanText() *token {
	if sm := rgxText.FindStringSubmatch(s.buffer); len(sm) != 0 {
		s.consume(len(sm[0]))

		mode := "inline"
		if sm[1] == "|" {
			mode = "piped"
		}

		return &token{Kind: tokText, Value: sm[2], Data: map[string]string{"Mode": mode}}
	}

	return nil
}

// Moves position forward, and removes beginning of s.buffer (len bytes)
func (s *scanner) consume(runes int) {
	if len(s.buffer) < runes {
		panic(fmt.Sprintf("Unable to consume %d runes from buffer.", runes))
	}

	s.lastTokenLine = s.line
	s.lastTokenCol = s.col
	s.lastTokenSize = runes

	s.buffer = s.buffer[runes:]
	s.col += runes
}

// Reads string into s.buffer
func (s *scanner) ensureBuffer() {
	if len(s.buffer) > 0 {
		return
	}

	buf, err := s.reader.ReadString('\n')
	s.curPos += len(buf)
	var lq int

process:
	if err != nil && err != io.EOF {
		panic(err)
	} else if err != nil && len(buf) == 0 {
		s.state = scnEOF
	} else {
		if buf[len(buf)-1] == '\n' {
			buf = buf[:len(buf)-1]
		}

		if lq = lineQuote(buf); lq >= 0 {
			var tmp string
			if tmp, err = s.reader.ReadString('\n'); err == nil || err == io.EOF {
				s.line += 1
				buf = buf[0:lq] + trimLeftSpace(tmp)
			}
			s.curPos += len(buf)
			goto process
		}

		s.state = scnNewLine
		s.buffer = buf
		s.line += 1
		s.col = 0
	}
}

func trimLeftSpace(s string) string {
	start := 0
	for ; start < len(s); start++ {
		c := s[start]
		if c >= utf8.RuneSelf {
			// If we run into a non-ASCII byte, fall back to the
			// slower unicode-aware method on the remaining bytes
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
	var l = len(s)
	if l == 0 {
		return -1
	}
	if s[l-1] == '\\' {
		return l - 1
	}
	return -1
}
