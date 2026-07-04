- create new parser in "./v2" directory like "gad/parser.Parser", using "gad/parser/source" for this template engine,
  and translates all nodes to gad.Node interface, preserving source.Pos values. For raw gad
  code, parses to related gad Node implementation.
- create an interface GiomCoder to regenerate initial formmatted code like gofmt and gad Coder,
  and take giom nodes to implements here.
- replaces all regexp match on "scanner.go" to per token scanner
- create giom command to: 1) render templates; 2) generate gad code; 3) format giom template.
  for subcommands use github.com/moisespsena-go/command-context.
- create expansive tests for parser, gad code generator, giom code generator, compiler and vm results.