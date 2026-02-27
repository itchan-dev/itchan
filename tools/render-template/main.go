// render-template executes a Go text/template file with environment variables.
//
// Usage:
//
//	go run ./tools/render-template/ -t config/private.yaml.tmpl -o config/private.yaml
//
// Template functions:
//
//	required        — fail if the value is empty (error includes file:line)
//	default "val"   — fall back when the env var is empty
//	split   "sep"   — split a string into a list (trims whitespace, drops empties)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func main() {
	tmplPath := flag.String("t", "", "template file path")
	outPath := flag.String("o", "", "output file path (default: stdout)")
	flag.Parse()

	if *tmplPath == "" {
		log.Fatal("-t (template path) is required")
	}

	// Collect env vars into a map for the template context.
	env := make(map[string]string)
	for _, e := range os.Environ() {
		k, v, _ := strings.Cut(e, "=")
		env[k] = v
	}

	funcMap := template.FuncMap{
		// {{ .VAR | required }}  — abort when empty (error includes file:line).
		"required": func(val string) (string, error) {
			if val == "" {
				return "", fmt.Errorf("required value is empty")
			}
			return val, nil
		},
		// {{ .VAR | default "fallback" }}
		"default": func(def, val string) string {
			if val != "" {
				return val
			}
			return def
		},
		// {{ .VAR | split "," }}  — returns []string, nil when empty.
		"split": func(sep, s string) []string {
			if s == "" {
				return nil
			}
			parts := strings.Split(s, sep)
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					out = append(out, p)
				}
			}
			return out
		},
	}

	tmpl, err := template.New(filepath.Base(*tmplPath)).
		Funcs(funcMap).
		Option("missingkey=zero").
		ParseFiles(*tmplPath)
	if err != nil {
		log.Fatalf("failed to parse template %s: %v", *tmplPath, err)
	}

	// Render into a buffer first so a template error doesn't produce a partial file.
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, env); err != nil {
		log.Fatalf("failed to render template: %v", err)
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, buf.Bytes(), 0600); err != nil {
			log.Fatalf("failed to write %s: %v", *outPath, err)
		}
		fmt.Printf("✓ %s\n", *outPath)
	} else {
		buf.WriteTo(os.Stdout)
	}
}
