package pagestream

import (
	g "maragu.dev/gomponents"
	dsattr "maragu.dev/gomponents-datastar"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

type DocumentSpec struct {
	Title     string
	Language  string
	HTMLAttrs []g.Node
	Head      []g.Node
	MainAttrs []g.Node
	Signals   map[string]any
	Init      []string
	Body      []g.Node
}

func RenderDocument(spec DocumentSpec) g.Node {
	language := spec.Language
	if language == "" {
		language = "en"
	}
	mainChildren := []g.Node{}
	if spec.Signals != nil {
		mainChildren = append(mainChildren, dsattr.Signals(spec.Signals))
	}
	if init := InitExpression(spec.Init...); init != "" {
		mainChildren = append(mainChildren, dsattr.Init(init))
	}
	mainChildren = append(mainChildren, spec.Body...)
	mainAttrs := append([]g.Node{}, spec.MainAttrs...)
	mainAttrs = append(mainAttrs, mainChildren...)
	return c.HTML5(c.HTML5Props{
		Title:     spec.Title,
		Language:  language,
		HTMLAttrs: spec.HTMLAttrs,
		Head:      spec.Head,
		Body:      []g.Node{h.Main(mainAttrs...)},
	})
}

func InitExpression(expressions ...string) string {
	out := ""
	for _, expression := range expressions {
		if expression == "" {
			continue
		}
		if out != "" {
			out += "; "
		}
		out += expression
	}
	return out
}
