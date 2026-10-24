# Go `html/template` Reference

## Part 1: The Go API (`html/template`)

Everything in `html/template` revolves around the `*template.Template` type. Below is the complete API surface you will use in Go code, organized by lifecycle stage.

### 1. Creating and Parsing Templates

You can load templates from raw strings, the OS filesystem, or an embedded filesystem (`io/fs`, such as `embed.FS`).

```go
// 1. From a raw string
tmpl, err := template.New("page").Parse("<h1>{{.Title}}</h1>")

// 2. From one or more specific files on disk
tmpl, err := template.ParseFiles("templates/base.html", "templates/home.html")

// 3. From a glob pattern on disk
tmpl, err := template.ParseGlob("templates/*.html")

// 4. From an io/fs.FS (e.g., //go:embed)
//go:embed templates/*.html
var tplFS embed.FS

tmpl, err := template.ParseFS(tplFS, "templates/base.html", "templates/home.html")
```

#### Wrapping with `template.Must`

At application startup, if a template has a syntax error, you want the program to panic immediately rather than boot in a broken state. `template.Must` takes `(*Template, error)` and panics if `err != nil`:

```go
var homeTmpl = template.Must(template.ParseFiles("templates/base.html", "templates/home.html"))
```

#### The #1 API Trap: `New()` + `ParseFiles()` Naming Mismatch

When you use `ParseFiles("dir/home.html")`, Go uses the **base filename** (`"home.html"`) as the template's name in the internal map.

If you write this, it **will fail** at runtime:

```go
// BUG: New("custom") creates an empty template named "custom".
// ParseFiles("dir/home.html") adds a SECOND template named "home.html" to the set.
// The returned pointer still points to the empty "custom" template!
t := template.Must(template.New("custom").Funcs(myFuncs).ParseFiles("dir/home.html"))
t.Execute(w, data) // Error: template: "custom" is an incomplete or empty template
```

**How to fix it (pick one):**

1. Match the name in `New()` to the exact base filename of the first file:

   ```go
   t := template.Must(template.New("home.html").Funcs(myFuncs).ParseFiles("dir/home.html"))
   t.Execute(w, data) // Works!
   ```

2. Or use `ExecuteTemplate` to explicitly target the filename or a `{{define "name"}}` block:

   ```go
   t.ExecuteTemplate(w, "home.html", data) // Works!
   ```

### 2. Executing Templates

Once parsed, you render a template into any `io.Writer` (`os.Stdout`, `*bytes.Buffer`, `http.ResponseWriter`). Both methods are safe to call concurrently from multiple goroutines.

```go
// Executes the primary template that `tmpl` points to
err := tmpl.Execute(w, data)

// Executes a specific named template (or {{define "name"}} block) in the set
err := tmpl.ExecuteTemplate(w, "base", data)
```

### 3. Registering Custom Functions (`FuncMap`)

To call custom Go functions inside a template, register a `template.FuncMap` via `.Funcs()`.

**Rules for custom functions:**

1. **Must be called BEFORE `.Parse*()`:** The parser verifies function names while building the AST. Calling `.Funcs()` after `.Parse()` does nothing.
2. **Return values:** A template function must return either **1 value** (the output to render/pipe) or **2 values** where the second value is `error`. If the function returns a non-nil `error` during execution, `Execute()` immediately stops and returns that error to your Go code.

```go
funcs := template.FuncMap{
	// 1 return value
	"formatCents": func(cents int) string {
		return fmt.Sprintf("$%.2f", float64(cents)/100.0)
	},

	// 2 return values (value, error)
	"safeDivide": func(a, b int) (int, error) {
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a / b, nil
	},
}

tmpl := template.Must(
	template.New("invoice.html").
		Funcs(funcs). // <-- MUST come before ParseFiles/Parse
		ParseFiles("invoice.html"),
)
```

### 4. Cloning, Delimiters, and Options

- **`tmpl.Clone()`**: Creates a deep copy of the template set so you can parse page-specific files onto a shared base layout without collisions:

  ```go
  pageClone, err := baseTmpl.Clone()
  _, err = pageClone.ParseFiles("pages/dashboard.html")
  ```

- **`tmpl.Delims(left, right)`**: Changes the action delimiters from `{{` and `}}` to something else. Useful if your HTML page uses a frontend script that also uses `{{ }}` (must be called before parsing):

  ```go
  tmpl := template.New("page").Delims("[[", "]]")
  ```

- **`tmpl.Option("missingkey=...")`**: Controls what happens when a template accesses a key in a `map[string]any` that does not exist:

  - `"missingkey=default"` or `"missingkey=invalid"`: Renders `<no value>` (default behavior).
  - `"missingkey=zero"`: Returns the zero value of the map value's type.
  - `"missingkey=error"`: Immediately stops `Execute()` and returns an error.

  ```go
  tmpl := template.New("page").Option("missingkey=error")
  ```

### 5. Safe Content Types (Bypassing Auto-Escaping)

When you intentionally want to inject pre-rendered, trusted strings without `html/template` escaping them, cast the string in Go to one of these types before passing it to `Execute`:

| Go Type | Context Where It Bypasses Escaping | Example Value |
| --- | --- | --- |
| `template.HTML` | HTML body / element content | `template.HTML("<strong>Bold</strong>")` |
| `template.HTMLAttr` | Inside an HTML tag as an attribute | ``template.HTMLAttr(`data-id="42"`)`` |
| `template.URL` | Inside `href="..."` or `src="..."` | `template.URL("tel:+8801700000000")` |
| `template.Srcset` | Inside `<img srcset="...">` | `template.Srcset("img-480w.jpg 480w, img-800w.jpg 800w")` |
| `template.JS` | Inside `<script>` as a raw JS expression | `template.JS("console.log('ready')")` |
| `template.JSStr` | Inside quotes in `<script>` (`var x = "{{.}}";`) | `template.JSStr("Hello\\nWorld")` |
| `template.CSS` | Inside `<style>` or `style="..."` | `template.CSS("color: #ff0000; font-weight: bold")` |

## Part 2: The Template Syntax

Everything inside `{{ ... }}` is called an **Action**. Outside `{{ ... }}` is raw text copied directly to the output stream.

### 1. Comments and Whitespace Trimming

**Comments:** Stripped completely during compilation; they never appear in the rendered HTML output (unlike `<!-- HTML comments -->`).

```html
{{/* This comment is completely invisible in the HTTP response */}}
```

**Whitespace Trimming (`{{-` and `-}}`):** By default, newlines and spaces outside `{{ ... }}` are preserved verbatim.

- `{{-` strips all spaces, tabs, and newlines immediately **before** the action.
- `-}}` strips all spaces, tabs, and newlines immediately **after** the action.

```html
<div>
  {{- .Name -}}
</div>
<!-- Renders as: <div>Alice</div> with zero newlines or spaces inside -->
```

*Note: Always put a space between the minus sign and your token: `{{- .Name -}}`, not `{{-.Name}}`, so it isn't parsed as a negative number.*

### 2. The Cursor (`.` and `$`) and Data Access

When you call `tmpl.Execute(w, data)`, `data` becomes the initial **cursor**, represented by a single dot `.`.

#### Accessing Struct Fields, Map Keys, and Methods

```html
<!-- 1. Exported Struct Field (must start with a capital letter in Go) -->
<p>{{.Username}}</p>

<!-- 2. Map Key (works with map[string]any, case-sensitive) -->
<p>{{. settings_theme}}</p>

<!-- 3. Nested Chaining -->
<p>{{.User.Profile.Bio}}</p>

<!-- 4. Exported Struct Methods! -->
<!-- You can call any exported method on your struct that returns 1 value (or 2 values where the 2nd is error) -->
<p>{{.User.FullName}}</p>

<!-- Calling a method that takes arguments (space-separated, no parentheses!) -->
{{if .User.HasPermission "write_post"}}
  <button>Publish</button>
{{end}}
```

#### The Root Cursor (`$`)

Blocks like `{{range}}` and `{{with}}` overwrite `.` to point to the inner item. To reach back out to the top-level data originally passed into `Execute()`, use `$`:

```html
{{range .Posts}}
  <!-- Here, `.` is an individual Post, while `$` is still the root page struct -->
  <article>
    <h2>{{.Title}}</h2>
    <small>Published on {{$.SiteTitle}} by {{$.CurrentUser.Name}}</small>
  </article>
{{end}}
```

### 3. Variables (`:=` and `=`)

You can create variables inside templates using a `$` prefix. Variables exist until the `{{end}}` of the block they were declared in.

```html
<!-- Declaration and initialization (:=) -->
{{$taxRate := 0.15}}
{{$greeting := .User.Name}}

<!-- Reassignment of an existing variable (=) -->
{{$greeting = "Anonymous"}}
```

Reassignment (`=`) is especially useful when you need to mutate state inside a `{{range}}` loop:

```html
{{$hasDiscount := false}}
{{range .CartItems}}
  {{if .OnSale}}
    {{$hasDiscount = true}}
  {{end}}
{{end}}
{{if $hasDiscount}}<p>Sale pricing applied!</p>{{end}}
```

### 4. Conditionals (`if`, `else if`, `else`)

```html
{{if .IsAdmin}}
  <a href="/admin">Admin Panel</a>
{{else if .IsModerator}}
  <a href="/mod">Mod Queue</a>
{{else}}
  <span>Standard User</span>
{{end}}
```

**What evaluates to `false` in `{{if .X}}`?** A value is false (empty) if it is:

- `false` (boolean)
- `0` (any numeric zero)
- `nil` (pointer, interface)
- `""` (empty string)
- An array, slice, or map with length `0` (`len == 0`)

Everything else evaluates to `true`.

### 5. Comparison and Boolean Operators (Prefix Notation)

In Go templates, `==`, `!=`, `&&`, and `||` do **not** exist as infix operators. Instead, they are **built-in functions**, which means the operator comes **first**, followed by its arguments separated by spaces: `operator arg1 arg2`.

#### Comparison Functions

- `eq arg1 arg2` (`arg1 == arg2`) — *Note: `eq` can take more than 2 arguments: `{{if eq .Role "admin" "editor" "owner"}}` checks if `.Role` equals ANY of the subsequent values!*
- `ne arg1 arg2` (`arg1 != arg2`)
- `lt arg1 arg2` (`arg1` less than `arg2`)
- `le arg1 arg2` (`arg1` less than or equal to `arg2`)
- `gt arg1 arg2` (`arg1` greater than `arg2`)
- `ge arg1 arg2` (`arg1` greater than or equal to `arg2`)

#### Boolean Logic Functions

- `not arg` (`!arg`)
- `and arg1 arg2 ...` (evaluates left-to-right, short-circuits on the first falsy value)
- `or arg1 arg2 ...` (evaluates left-to-right, short-circuits on the first truthy value)

#### Combining Them with Parentheses `()`

Use parentheses to nest function calls inside conditionals:

```html
<!-- Equivalent in Go: if user.Age >= 18 && !user.IsBanned -->
{{if and (ge .User.Age 18) (not .User.IsBanned)}}
  <p>Access Granted</p>
{{end}}

<!-- Equivalent in Go: if status == "paid" || status == "shipped" || status == "delivered" -->
{{if eq .Status "paid" "shipped" "delivered"}}
  <p>Order is confirmed.</p>
{{end}}
```

### 6. Loops (`range`, `else`, `break`, `continue`)

`{{range}}` iterates over slices, arrays, maps (sorted by key if keys are strings/ints), channels, or an integer count. Inside the loop, `.` becomes the current element.

#### Basic Loop

```html
<ul>
{{range .Items}}
  <li>{{.Name}} - ${{.Price}}</li>
{{end}}
</ul>
```

#### Capturing Index/Key and Value

```html
<!-- For Slices/Arrays: $i is index (0, 1, 2...), $item is the element -->
{{range $i, $item := .Items}}
  <p>#{{$i}}: {{$item.Name}} (or {{.Name}}, since `.` is also set to $item)</p>
{{end}}

<!-- For Maps: $k is the key, $v is the value -->
{{range $k, $v := .Headers}}
  <p><strong>{{$k}}:</strong> {{$v}}</p>
{{end}}
```

#### `range ... else` (Handling Empty Lists Cleanly)

Instead of wrapping a `{{range}}` inside an `{{if .Items}}`, use `{{else}}` directly on the `range` block. If the slice or map is empty or `nil`, the `{{else}}` branch runs:

```html
<ul>
{{range .Notifications}}
  <li>{{.Message}}</li>
{{else}}
  <li class="empty">No new notifications.</li>
{{end}}
</ul>
```

#### `{{break}}` and `{{continue}}`

You can exit a loop early or skip to the next iteration:

```html
{{range $i, $item := .Items}}
  {{if eq $i 5}}
    {{break}} {{/* Stop after 5 items */}}
  {{end}}
  {{if .IsHidden}}
    {{continue}} {{/* Skip hidden items */}}
  {{end}}
  <li>{{.Name}}</li>
{{end}}
```

### 7. Scoping with `with`

`{{with pipeline}}` checks if a value is non-empty. If it is truthy, it binds `.` to that value inside the block. If it is empty/nil, it skips the block (or executes `{{else}}`). This saves you from repeating long struct paths.

```html
<!-- Without `with`: repetitive and error-prone -->
{{if .User.Company}}
  <p>{{.User.Company.Name}} ({{.User.Company.TaxID}})</p>
{{end}}

<!-- With `with`: clean cursor shift -->
{{with .User.Company}}
  <p>{{.Name}} ({{.TaxID}})</p>
{{else}}
  <p>Freelancer / No Company Listed</p>
{{end}}
```

### 8. Pipelines (`|`) and Built-in Functions

A pipeline chains values and functions together using `|`. The output of each stage is passed as the **last argument** to the next function in the chain.

```html
<!-- Equivalent to: printf "$%.2f" .Price -->
<p>{{.Price | printf "$%.2f"}}</p>
```

#### Essential Built-in Functions

Aside from `eq`/`ne`/`and`/`or`/`not`, Go templates include these built-in functions out of the box:

| Function | Usage Example | Description |
| --- | --- | --- |
| **`index`** | `{{index .Items 0}}` | Accesses index `0` of a slice/array, or a key in a map (`{{index .Map "key"}}`). Can chain dimensions: `{{index .Matrix 1 2}}` (`Matrix[1][2]`). |
| **`slice`** | `{{slice .Items 1 4}}` | Equivalent to Go's `.Items[1:4]`. |
| **`len`** | `{{len .Items}}` | Returns the integer length of a string, slice, array, or map. |
| **`printf`** | `{{printf "%04d" .ID}}` | Alias for `fmt.Sprintf`. |
| **`print`** / **`println`** | `{{print .A .B}}` | Aliases for `fmt.Sprint` and `fmt.Sprintln`. |
| **`call`** | `{{call .MyFunc "arg1"}}` | Calls a `func` field stored directly on your data struct or map. |

### 9. Template Composition (`define`, `template`, `block`)

This is how you build layouts, partials (headers/footers), and reusable components.

#### 1. `{{define "name"}}` and `{{template "name" pipeline}}`

- `{{define "name"}}...{{end}}` declares a named sub-template.
- `{{template "name" .}}` invokes and renders that sub-template at that exact spot.

**Crucial Rule:** Sub-templates do **not** inherit variables or `.` automatically!

- `{{template "card"}}` passes `nil` as `.` inside `"card"`.
- `{{template "card" .}}` passes the current `.` cursor into `"card"`.
- `{{template "card" .User}}` passes `.User` as the `.` cursor inside `"card"`.

```html
{{define "userCard"}}
  <div class="card">
    <h3>{{.Name}}</h3>
    <p>{{.Email}}</p>
  </div>
{{end}}

<!-- Render it for a single user -->
{{template "userCard" .CurrentUser}}

<!-- Or reuse it inside a loop! -->
{{range .TeamMembers}}
  {{template "userCard" .}}
{{end}}
```

#### 2. `{{block "name" pipeline}}Default{{end}}` (Layout Inheritance)

`{{block}}` defines a placeholder slot in a base layout with default fallback content, and immediately renders it. Any child template parsed into the same set can override that slot using `{{define "name"}}`.

**`base.html`:**

```html
<!DOCTYPE html>
<html>
<head>
  <title>{{block "title" .}}Default Title{{end}}</title>
  {{block "extra_head" .}}{{end}}
</head>
<body>
  <main>
    {{block "main" .}}<p>Nothing here yet.</p>{{end}}
  </main>
</body>
</html>
```

**`invoice.html` (parsed on top of a `Clone()` of `base.html`):**

```html
{{define "title"}}Invoice #{{.InvoiceID}}{{end}}

{{define "extra_head"}}
  <style>.paid { color: green; }</style>
{{end}}

{{define "main"}}
  <h1>Invoice #{{.InvoiceID}}</h1>
  <p>Total: {{.Total}}</p>
{{end}}
```

## Part 3: Putting It All Together (End-to-End Reference)

Here is a single, self-contained program demonstrating structs, methods, custom `FuncMap`, safe HTML types, pipelines, `range`, `with`, `index`, `eq`, and template composition.

```go
package main

import (
	"fmt"
	"html/template"
	"os"
	"strings"
)

type Item struct {
	SKU      string
	Name     string
	Qty      int
	PriceBDT int
}

// Exported methods can be called directly inside templates: {{.Subtotal}}
func (i Item) Subtotal() int {
	return i.Qty * i.PriceBDT
}

type InvoicePage struct {
	ShopName   string
	Customer   string
	Status     string
	Notes      template.HTML // Trusted HTML that bypasses escaping
	Items      []Item
	Discounts  map[string]int
	FooterLink string
}

// Exported method taking an argument: {{.HasDiscount "SUMMER10"}}
func (p InvoicePage) HasDiscount(code string) bool {
	_, ok := p.Discounts[code]
	return ok
}

const invoiceTpl = `<!DOCTYPE html>
<html>
<body>
  <header>
    <h1>{{.ShopName | upper}}</h1>
    {{- with .Customer}}
    <p>Billed to: <strong>{{.}}</strong></p>
    {{- else}}
    <p>Billed to: <em>Walk-in Customer</em></p>
    {{- end}}
  </header>

  {{/* Conditional with prefix operators: eq and or */}}
  {{if eq .Status "PAID" "SETTLED"}}
    <div class="badge">[STATUS: PAID]</div>
  {{else}}
    <div class="badge">[STATUS: {{.Status}}]</div>
  {{end}}

  <table>
    <thead>
      <tr><th>#</th><th>Item</th><th>Qty</th><th>Price</th><th>Subtotal</th></tr>
    </thead>
    <tbody>
    {{$grandTotal := 0}}
    {{range $idx, $item := .Items}}
      {{$grandTotal = add $grandTotal $item.Subtotal}}
      <tr>
        <td>{{add $idx 1}}</td>
        <td>{{$item.Name}} ({{$item.SKU}})</td>
        <td>{{$item.Qty}}</td>
        <td>{{formatBDT $item.PriceBDT}}</td>
        <td>{{formatBDT $item.Subtotal}} ({{$.ShopName}})</td>
      </tr>
    {{else}}
      <tr><td colspan="5">No items in this invoice.</td></tr>
    {{end}}
    </tbody>
  </table>

  {{/* Calling an exported Go method with an argument */}}
  {{if .HasDiscount "STUDENT"}}
    <p>Student Discount Applied: -{{index .Discounts "STUDENT" | formatBDT}}</p>
  {{end}}

  {{/* Trusted HTML field (template.HTML) is not escaped */}}
  <div class="notes">{{.Notes}}</div>

  {{/* Invoking a sub-template and passing only the FooterLink string as '.' */}}
  {{template "footer" .FooterLink}}
</body>
</html>

{{- define "footer"}}
  <footer>
    <a href="{{.}}">Verify Invoice</a>
  </footer>
{{- end}}
`

func main() {
	// 1. Register custom functions BEFORE parsing
	funcs := template.FuncMap{
		"upper": strings.ToUpper,
		"add":   func(a, b int) int { return a + b },
		"formatBDT": func(amount int) string {
			return fmt.Sprintf("৳%d", amount)
		},
	}

	// 2. Parse template
	tmpl := template.Must(
		template.New("invoice").
			Funcs(funcs).
			Option("missingkey=error").
			Parse(invoiceTpl),
	)

	// 3. Prepare data
	data := InvoicePage{
		ShopName: "Bio Sun Care",
		Customer: "Robiul",
		Status:   "PAID",
		Notes:    template.HTML("<em>Thank you for shopping with us!</em>"),
		Items: []Item{
			{SKU: "CLN-01", Name: "Gentle Cleanser", Qty: 2, PriceBDT: 450},
			{SKU: "SPF-50", Name: "Matte Sunscreen", Qty: 1, PriceBDT: 850},
		},
		Discounts:  map[string]int{"STUDENT": 100},
		FooterLink: "https://example.com/verify?id=104",
	}

	// 4. Execute to stdout
	if err := tmpl.Execute(os.Stdout, data); err != nil {
		panic(err)
	}
}
```
