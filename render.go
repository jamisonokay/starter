package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const tmplSuffix = ".tmpl"

// render applies the template layers one after another into dst.
//
// Files ending in ".tmpl" are executed with text/template and lose the suffix;
// everything else is copied byte-for-byte (so .templ files, JS, images and
// anything with literal "{{" stay untouched). Generated .go files are gofmt-ed.
func render(src fs.FS, layerNames []string, dst string, data *Data) error {
	for _, layer := range layerNames {
		if st, err := fs.Stat(src, layer); err != nil || !st.IsDir() {
			return fmt.Errorf("template has no %q directory", layer)
		}
		sub, err := fs.Sub(src, layer)
		if err != nil {
			return err
		}
		err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return os.MkdirAll(filepath.Join(dst, filepath.FromSlash(p)), 0o755)
			}
			return renderFile(sub, p, d, dst, data)
		})
		if err != nil {
			return fmt.Errorf("layer %q: %w", layer, err)
		}
	}
	return nil
}

func renderFile(src fs.FS, p string, d fs.DirEntry, dst string, data *Data) error {
	content, err := fs.ReadFile(src, p)
	if err != nil {
		return err
	}
	info, err := d.Info()
	if err != nil {
		return err
	}

	outRel := p
	if strings.HasSuffix(p, tmplSuffix) {
		t, err := template.New(p).Option("missingkey=error").Parse(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, data); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		content = buf.Bytes()
		outRel = strings.TrimSuffix(p, tmplSuffix)
	}

	if strings.HasSuffix(outRel, ".go") {
		formatted, err := format.Source(content)
		if err != nil {
			return fmt.Errorf("%s: generated Go code does not parse: %w", outRel, err)
		}
		content = formatted
	}

	out := filepath.Join(dst, filepath.FromSlash(outRel))
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, content, info.Mode().Perm()|0o600)
}

// createDotEnv copies .env.example to .env (never overwrites an existing one).
func createDotEnv(dir string) error {
	example, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		return nil // template has no .env.example
	}
	target := filepath.Join(dir, ".env")
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	return os.WriteFile(target, example, 0o600)
}
