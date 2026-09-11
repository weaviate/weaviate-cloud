package guide

import _ "embed"

//go:embed guide.md
var markdown string

// Markdown returns the canonical agent guide as the embedded Markdown text.
func Markdown() string { return markdown }
