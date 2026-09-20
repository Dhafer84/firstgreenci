// Package generate turns a detected project into a GitHub Actions workflow.
//
// The workflow is rendered from a text template rather than serialised from a
// data structure: a YAML marshaller drops comments, and the comments are what
// makes the generated file readable by a beginner. They are the point of the
// file, not decoration.
package generate

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/Dhafer84/firstgreenci/internal/detect"
	"github.com/Dhafer84/firstgreenci/internal/i18n"
)

// FileExistsError reports a workflow file that is already there. The user's
// files are never overwritten without a confirmation: design principle 6.
type FileExistsError struct {
	Path string
}

func (e *FileExistsError) Error() string {
	return fmt.Sprintf("%s already exists", e.Path)
}

// CreateDirError reports a directory that could not be created.
type CreateDirError struct {
	Path string
	Err  error
}

func (e *CreateDirError) Error() string {
	return fmt.Sprintf("create directory %s: %v", e.Path, e.Err)
}

func (e *CreateDirError) Unwrap() error { return e.Err }

// WriteError reports a file that could not be written.
type WriteError struct {
	Path string
	Err  error
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("write file %s: %v", e.Path, e.Err)
}

func (e *WriteError) Unwrap() error { return e.Err }

// WorkflowPath returns the path of the generated workflow inside a project,
// built with the separator of the running system.
func WorkflowPath() string {
	return filepath.Join(".github", "workflows", "ci.yml")
}

// Render produces the content of the workflow for project, in the language of
// catalog. templates is the embedded file system holding templates/.
func Render(templates fs.FS, catalog *i18n.Catalog, project *detect.Project) ([]byte, error) {
	// Embedded file systems always use forward slashes, whatever the system.
	name := path.Join("templates", string(project.Language)+".yml.tmpl")

	source, err := fs.ReadFile(templates, name)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}

	parsed, err := template.New(path.Base(name)).Funcs(functions(catalog)).Parse(string(source))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, project); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}

	return rendered.Bytes(), nil
}

// Write saves content as the workflow of the project rooted at root. It
// returns the path of the file it wrote, relative to root.
func Write(root string, content []byte, force bool) (string, error) {
	relative := WorkflowPath()
	absolute := filepath.Join(root, relative)

	if _, err := os.Stat(absolute); err == nil && !force {
		return relative, &FileExistsError{Path: relative}
	}

	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return relative, &CreateDirError{Path: filepath.Dir(relative), Err: err}
	}

	if err := os.WriteFile(absolute, content, 0o644); err != nil {
		return relative, &WriteError{Path: relative, Err: err}
	}

	return relative, nil
}

// Exists reports whether the project already carries a workflow file.
func Exists(root string) bool {
	_, err := os.Stat(filepath.Join(root, WorkflowPath()))
	return err == nil
}

// functions are the helpers available inside the templates. They are kept few
// and obvious, so that a contributor can write a template without reading the
// engine.
func functions(catalog *i18n.Catalog) template.FuncMap {
	return template.FuncMap{
		// t translates a key, so that a workflow generated in French is
		// commented in French.
		"t": catalog.T,

		// q writes a value as a quoted YAML scalar. Without it, a version
		// such as 3.10 would be read as a number and become 3.1.
		"q": func(value string) string { return strconv.Quote(value) },

		// indent shifts a possibly multi-line command under a YAML block
		// scalar, which keeps commands readable and free of quoting rules.
		"indent": func(value string, spaces int) string {
			padding := strings.Repeat(" ", spaces)
			lines := strings.Split(value, "\n")
			for i, line := range lines {
				if line == "" {
					continue
				}
				lines[i] = padding + line
			}
			return strings.Join(lines, "\n")
		},
	}
}
