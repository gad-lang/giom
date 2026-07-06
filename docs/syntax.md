# Template Syntax

Giom uses indentation to describe HTML, components, and Gad control flow.

## Document Type

```giom
!!! 5
```

Output:

```html
<!DOCTYPE html>
```

## Tags

```giom
section.hero
    h1 Welcome
    p Ship templates with less noise.
```

Output:

```html
<section class="hero"><h1>Welcome</h1><p>Ship templates with less noise.</p></section>
```

## Ids And Classes

```giom
main#content.page.shell
    h1.title Hello
```

Output:

```html
<main id="content" class="page shell"><h1 class="title">Hello</h1></main>
```

## Attributes

```giom
a.button[href="/docs"][target="_blank"] Read docs
img.cover[src=Post.CoverImage][alt=Post.Title]
input[type="email"][name="email"][required]
```

Attributes can use string literals, variables, or expressions.

## Text

Inline text:

```giom
p Hello world
```

Text block:

```giom
p
    | This is plain text.
    | It can span multiple lines.
```

## Expressions

```giom
h1 {= Model.Title}
p {= "Hello " + User.Name}
```

Use Gad expressions inside `{= ...}`.

## Raw HTML Values

If the application passes a `gad.RawStr`, Giom writes it without escaping.

```giom
article {= Post.Body}
```

Use raw values only for trusted HTML.

## Main Block

```giom
@main
    h1 Home
    p This template body is executed.
```

## Code Block

Use `~~` for Gad source sections.

```giom
~~
const title = "Hello"
~~

@main
    h1 {= title}
```

## Variables And Assignment

```giom
@main
    @assign total = len(Items)
    p {= total + " items"}
```

Depending on parser form, assignment can also be represented by Gad code inside
`~~` blocks.

## Conditions

```giom
@if User
    p Welcome {= User.Name}
@else
    p Welcome guest
```

## Loops

```giom
ul
    @for item in Items
        li {= item.Title}
```

## Empty States

```giom
@if Posts
    div.grid
        @for post in Posts
            article.card {= post.Title}
@else
    p No posts yet.
```

## Match

```giom
@match Status
    @case "draft"
        span.badge Draft
    @case "published"
        span.badge Published
    @else
        span.badge Unknown
```

## Imports

### Bare Import

```giom
@import "components.giom"
```

Imports the module for its side effects.

### Named Import

```giom
@import "components.giom" as comps
```

Makes the module available as the variable `comps`. Components or values from
the module are accessed via `+comps.name(...)`.

### Destructured Import

```giom
@import { page_wrapper, hero } from "components.giom"
```

Extracts specific named exports directly into scope. Components are then
available as `+page_wrapper(...)` and `+hero(...)` without a module prefix.

Supports Gad destructuring syntax including:

- Rename: `@import { page_wrapper: pw } from "components.giom"`
- Default value: `@import { page_wrapper = fallback } from "components.giom"`
- Rest pattern: `@import { page_wrapper, **rest } from "components.giom"`
- Mixed: `@import { a, b: bb, c = nil, **rest } from "modules.giom"`

All forms compile to Gad `import()` calls. Destructured imports generate a
curly-destructure assignment (`{...} := import("...")`), which is handled by
Gad's built-in destructuring compiler.

## Variable Declarations

Declare mutable variables with `@var`. Supports comma-separated declarations
and optional initializer expressions.

```giom
@var a, b = {name: "test"}, x
```

Compiles to Gad `var (a, b={name: "test"}, x)`.

## Globals

Declare globals using `@global` followed by space-separated names.

```giom
@global Model User
```

Compiles to Gad `global (Model, User)`. Globals can also be provided through
the Go symbol table — the CMS example passes one global named `Model`.
