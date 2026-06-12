- create new parser like "gad/parser.Parser", using "gad/parser/source" for this template engine,
  and translates all nodes to gad.Node interface, preserving source.Pos values. For raw gad
  code, parses to related gad Node implementation