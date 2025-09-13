package goku

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed templates
var tmplEmbed embed.FS

var tmpls *template.Template

func init() {
	var err error
	tmpls, err = template.New("goku").Funcs(template.FuncMap{
		"join": strings.Join,
	}).ParseFS(tmplEmbed, "**/*.go.tmpl")

	if err != nil {
		panic(err)
	}
}
