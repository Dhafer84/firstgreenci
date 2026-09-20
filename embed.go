// Package firstgreenci embeds the assets that ship inside the binary.
//
// Templates and translations live at the repository root, outside the engine,
// so that contributors can propose a new pipeline template or a new language
// without touching Go code. A go:embed directive cannot reach above the
// package that declares it, so the directives live here, at the root, and the
// internal packages consume the file systems exported below.
package firstgreenci

import "embed"

// TemplatesFS holds the pipeline templates, rooted at "templates".
//
//go:embed templates
var TemplatesFS embed.FS

// LocalesFS holds the translation catalogs, rooted at "locales".
//
//go:embed locales
var LocalesFS embed.FS
