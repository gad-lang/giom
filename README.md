# giom

giom is an elegant templating engine for Go, inspired by HAML and Jade/Pug.
It compiles templates to [GAD](https://github.com/gad-lang/gad) (a Go-based scripting language) bytecode.

### Usage

```go
import "github.com/gad-lang/giom"
```

### Tags

A tag is simply a word:

    html

is converted to

```html
<html></html>
```

It is possible to add ID and CLASS attributes to tags:

    div#main
    span.time

are converted to

```html
<div id="main"></div>
<span class="time"></span>
```

Any arbitrary attribute name / value pair can be added this way:

    a[href="http://www.google.com"]

You can mix multiple attributes together

    a#someid[href="/"][title="Main Page"].main.link Click Link

gets converted to

```html
<a id="someid" class="main link" href="/" title="Main Page">Click Link</a>
```

It is also possible to define these attributes within the block of a tag

    a
        #someid
        [href="/"]
        [title="Main Page"]
        .main
        .link
        | Click Link

### Doctypes

To add a doctype, use `!!!` or `doctype` keywords:

    !!! transitional
    // <!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">

or use `doctype`

    doctype 5
    // <!DOCTYPE html>

Available options: `5`, `default`, `xml`, `transitional`, `strict`, `frameset`, `1.1`, `basic`, `mobile`

### Tag Content

For single line tag text, you can just append the text after tag name:

    p Testing!

would yield

    <p>Testing!</p>

For multi line tag text, or nested tags, use indentation:

    html
        head
            title Page Title
        body
            div#content
                p
                    | This is a long page content
                    | These lines are all part of the parent p

                    a[href="/"] Go To Main Page

### Data

Input template data can be reached by key names directly. For example, assuming the template has been
executed with following JSON data:

```json
{
  "Name": "Ekin",
  "LastName": "Koc",
  "Repositories": [
    "giom",
    "dateformat"
  ],
  "Avatar": "/images/ekin.jpg",
  "Friends": 17
}
```

It is possible to interpolate fields using `#{}`

    p Welcome #{Name}!

would print

```html
<p>Welcome Ekin!</p>
```

Attributes can have field names as well

    a[title=Name][href="/ekin.koc"]

would print

```html
<a title="Ekin" href="/ekin.koc"></a>
```

### Expressions

giom can expand basic expressions. For example, it is possible to concatenate strings with + operator:

    p Welcome #{Name + " " + LastName}

Arithmetic expressions are also supported:

    p You need #{50 - Friends} more friends to reach 50!

Expressions can be used within attributes

    img[alt=Name + " " + LastName][src=Avatar]

### Variables

It is possible to define dynamic variables within templates,
all variables must start with a $ character and can be assigned as in the following example:

    div
        $fullname = Name + " " + LastName
        p Welcome #{$fullname}

If you need to access the supplied data itself (i.e. the object containing Name, LastName etc fields.) you can use `$` variable

    p $.Name

### Conditions

For conditional blocks, it is possible to use `if <expression>`

    div
        if Friends > 10
            p You have more than 10 friends
        else if Friends > 5
            p You have more than 5 friends
        else
            p You need more friends

Again, it is possible to use arithmetic and boolean operators

    div
        if Name == "Ekin" && LastName == "Koc"
            p Hey! I know you..

There is a special syntax for conditional attributes. Only block attributes can have conditions;

    div
        .hasfriends ? Friends > 0

This would yield a div with `hasfriends` class only if the `Friends > 0` condition holds. It is
perfectly fine to use the same method for other types of attributes:

    div
        #foo ? Name == "Ekin"
        [bar=baz] ? len(Repositories) > 0

### Iterations

It is possible to iterate over arrays and maps using `each`:

    each $repo in Repositories
        p #{$repo}

would print

    p giom
    p dateformat

It is also possible to iterate over values and indexes at the same time

    each $i, $repo in Repositories
        p
            .even ? $i % 2 == 0
            .odd ? $i % 2 == 1

### Comps

Comps (reusable template blocks that accept arguments) can be defined:

    mixin surprise
        span Surprise!
    mixin link($href, $title, $text)
        a[href=$href][title=$title] #{$text}

and then called multiple times within a template (or even within another mixin definition):

    div
    	+surprise
    	+surprise
        +link("http://google.com", "Google", "Check out Google")

Template data, variables, expressions, etc., can all be passed as arguments:

    +link(GoogleUrl, $googleTitle, "Check out " + $googleTitle)

### Imports

A template can import other templates using `import`:

    a.giom
        p this is template a

    b.giom
        p this is template b

    c.giom
        div
            import a
            import b

gets compiled to

    div
        p this is template a
        p this is template b

### Inheritance

A template can inherit other templates. In order to inherit another template, an `extends` keyword should be used.
Parent template can define several named blocks and child template can modify the blocks.

    master.giom
        !!! 5
        html
            head
                block meta
                    meta[name="description"][content="This is a great website"]

                title
                    block title
                        | Default title
            body
                block content

    subpage.giom
        extends master

        block title
            | Some sub page!

        block append meta
            // This will be added after the description meta tag. It is also possible
            // to prepend someting to an existing block
            meta[name="keywords"][content="foo bar"]

        block content
            div#main
                p Some content here

### License
(The MIT License)

Copyright (c) 2012 Ekin Koc <ekin@eknkc.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the 'Software'), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED 'AS IS', WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

## Usage

### Compile to GAD

Use `CompileToGad` to compile a giom template to GAD source code:

```go
import (
    "bytes"
    "github.com/gad-lang/giom"
)

func main() {
    var out bytes.Buffer
    err := giom.CompileToGad(&out, []byte(`html
    head
        title Page Title
    body
        div#content
            p Hello #{name}
`), giom.Options{})
    // out.String() contains the compiled GAD source
}
```

### Build and Execute

Use `TemplateBuilder` to build and execute a compiled template:

```go
import (
    "bytes"
    "github.com/gad-lang/giom"
)

func main() {
    var gadBuf bytes.Buffer
    giom.CompileToGad(&gadBuf, []byte(`
@comp Greet(name)
    p Hello #{name}

@main
    +Greet("World")
`), giom.Options{})

    t, err := giom.NewTemplateBuilder(gadBuf.Bytes()).Build()
    if err != nil {
        panic(err)
    }

    var html bytes.Buffer
    _, err = t.Executor().Out(&html).ExecuteModule()
    // html.String() == "<p>Hello World</p>"
}
```

### Key Types

#### Options

```go
type Options struct {
    PrettyPrint bool   // Pretty print output HTML (default: true)
    LineNumbers bool   // Emit line number comments (default: false)
    PreCode     string // Prepended GAD source code
    FileName    string // Source file name for error traces
}
```

#### TemplateBuilder

Used to build a `*Template` from compiled GAD bytecode:

```go
builder := giom.NewTemplateBuilder(gadSource)
builder.WithContext(ctx)
builder.WithModule(module)
builder.WithModuleMap(moduleMap)
builder.WithBuiltins(builtins)
t, err := builder.Build()
```

#### TemplateExecutor

Executes a `*Template` and writes output:

```go
executor := t.Executor()
executor.Out(writer)         // Output writer
executor.Global(data)        // Global variables map
executor.Args(args...)       // Positional arguments
executor.NamedArgs(named)    // Named arguments
result, err := executor.Execute()
result, err := executor.ExecuteModule()  // Runs "main" export
```

### Comps (Components)

Comps are reusable template blocks with arguments:

```
@comp Greet(name)
    p Hello #{name}

@main
    +Greet("World")
```

### Exports

Comps can be exported to make them accessible from the module result:

```
@export comp Alert(message)
    div.alert #{message}
```

### Slots

Slots allow content injection into comps:

```
@comp Card
    @slot main
        | Default content

@main
    +Card()
        | Custom content
```
