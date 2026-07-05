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
h1 #{= Model.Title}
p #{= "Hello " + User.Name}
```

Use Gad expressions inside `#{= ...}`.

## Raw HTML Values

If the application passes a `gad.RawStr`, Giom writes it without escaping.

```giom
article #{= Post.Body}
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
    h1 #{= title}
```

## Variables And Assignment

```giom
@main
    @assign total = len(Items)
    p #{= total + " items"}
```

Depending on parser form, assignment can also be represented by Gad code inside
`~~` blocks.

## Conditions

```giom
@if User
    p Welcome #{= User.Name}
@else
    p Welcome guest
```

## Loops

```giom
ul
    @for item in Items
        li #{= item.Title}
```

## Empty States

```giom
@if Posts
    div.grid
        @for post in Posts
            article.card #{= post.Title}
@else
    p No posts yet.
```

## Switch

```giom
@switch Status
    @case "draft"
        span.badge Draft
    @case "published"
        span.badge Published
    @default
        span.badge Unknown
```

## Imports

```giom
@import "components.giom"
```

The parser recognizes import lines. Applications commonly resolve imports before
compilation, as shown in `examples/cms/main.go`.

## Globals

Declare globals in Gad code sections or provide them through the Go symbol table.
The CMS example passes one global named `Model`.

```giom
~~
global (Model)
~~

@main
    h1 #{= Model.Title}
```
