package parser

import "fmt"

type ParseError struct {
	Filename    string
	Line        int
	Column      int
	TokenLength int
	Err         error
}

func (p *ParseError) Error() string {
	return fmt.Sprintf("giom Error in <%s:%d:%d:%d>: %v", p.Filename, p.Line, p.Column, p.TokenLength, p.Err)
}

func (p *ParseError) Unrap() error {
	return p.Err
}
