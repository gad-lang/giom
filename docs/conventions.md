# Conventions

## Go ↔ Gad value conversion

**Use `gad.MustNewReflectValue` instead of manually constructing `gad.Dict`.**

Pass Go structs directly through `gad.MustNewReflectValue` — the function uses reflection to convert exported struct fields into Gad values automatically. This eliminates the boilerplate of mapping each field by hand.

```go
// good — reflection-based
gad.MustNewReflectValue(page)

// bad — manual dict construction
gad.Dict{"ID": gad.Uint(p.ID), "Title": gad.Str(p.Title), …}
```

For computed fields (e.g., `URL` derived from `Slug`), pre-populate the struct before reflection rather than building a separate dict.

```go
p.URL = "/posts/" + p.Slug
gad.MustNewReflectValue(p)
```

Use `gorm:"-"` tags on model fields that should not be persisted but are needed during reflection (e.g., computed `URL` fields).

## Template output — raw HTML

**Use `#{=raw expr}` for template expressions that contain HTML markup.**

The `raw` directive outputs the value without HTML escaping. Use it for `Body`, `RightBody`, or any field that contains pre-rendered HTML.

| Expression | Escaping | Use case |
|---|---|---|
| `#{= expr}` | Escaped | Plain text, user-controlled strings |
| `#{=raw expr}` | Unescaped | HTML content (Body, RightBody) |

```giom
section.page-body
    #{=raw Model.Page.Body}
```
