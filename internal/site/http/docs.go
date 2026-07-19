package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"net/url"
	"regexp"
	"strings"

	content "github.com/Yacobolo/libredash/docs"
	docsearch "github.com/Yacobolo/libredash/internal/site/search/sqlite"
	"github.com/Yacobolo/libredash/internal/visualdocs"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

var markdownRenderer = goldmark.New(goldmark.WithExtensions(extension.GFM))

type siteDocument struct {
	slug               string
	title              string
	breadcrumb         string
	breadcrumbRoot     string
	breadcrumbRootHref string
	summary            string
	markdown           string
	sectionID          string
	groupID            string
	source             string
	navigationTitle    string
	generated          bool
}

type siteCatalogDocument struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	NavigationTitle string `json:"navigationTitle"`
	Summary         string `json:"summary"`
	Source          string `json:"source"`
	Breadcrumb      string `json:"breadcrumb"`
	Generated       bool   `json:"generated"`
}

type siteCatalogGroup struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Summary   string                `json:"summary"`
	Href      string                `json:"href"`
	Documents []siteCatalogDocument `json:"documents"`
}

type siteCatalogSection struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Summary   string                `json:"summary"`
	Href      string                `json:"href"`
	Documents []siteCatalogDocument `json:"documents"`
	Groups    []siteCatalogGroup    `json:"groups"`
}

type siteDocumentationCatalog struct {
	Sections []siteCatalogSection `json:"sections"`
}

type loadedDocumentation struct {
	catalog   siteDocumentationCatalog
	documents []siteDocument
	bySlug    map[string]siteDocument
}

var documentation = loadDocumentation()
var siteCatalog = documentation.catalog
var siteDocuments = documentation.documents
var siteDocumentsBySlug = documentation.bySlug
var documentationSearchIndex = loadDocumentationSearchIndex()
var visualDocuments = documentsInCatalogGroup("reference", "visuals", true)
var visualDocumentationCatalogValidated = validateVisualDocumentationCatalog()

func loadDocumentationSearchIndex() *docsearch.Index {
	index, err := docsearch.Open(content.Files, docsearch.Filename)
	if err != nil {
		panic(fmt.Sprintf("open documentation search index: %v", err))
	}
	slugs, err := index.Slugs(context.Background())
	if err != nil {
		panic(fmt.Sprintf("read documentation search index: %v", err))
	}
	if len(slugs) != len(siteDocuments) {
		panic(fmt.Sprintf("documentation search index contains %d documents, catalog contains %d", len(slugs), len(siteDocuments)))
	}
	for _, slug := range slugs {
		if _, exists := siteDocumentsBySlug[slug]; !exists {
			panic(fmt.Sprintf("documentation search index contains unknown slug %q", slug))
		}
	}
	return index
}

func loadDocumentation() loadedDocumentation {
	catalogContents, err := content.Files.ReadFile("catalog.json")
	if err != nil {
		panic(fmt.Sprintf("read documentation catalog: %v", err))
	}
	var catalog siteDocumentationCatalog
	if err := json.Unmarshal(catalogContents, &catalog); err != nil {
		panic(fmt.Sprintf("decode documentation catalog: %v", err))
	}
	loaded := loadedDocumentation{catalog: catalog, bySlug: make(map[string]siteDocument)}
	for _, section := range catalog.Sections {
		for _, document := range section.Documents {
			loaded.add(section, siteCatalogGroup{}, document)
		}
		for _, group := range section.Groups {
			for _, document := range group.Documents {
				loaded.add(section, group, document)
			}
		}
	}
	return loaded
}

func (loaded *loadedDocumentation) add(section siteCatalogSection, group siteCatalogGroup, document siteCatalogDocument) {
	markdown, err := content.Files.ReadFile(document.Source)
	if err != nil {
		panic(fmt.Sprintf("read documentation source %q: %v", document.Source, err))
	}
	if _, exists := loaded.bySlug[document.Slug]; exists {
		panic(fmt.Sprintf("duplicate documentation slug %q", document.Slug))
	}
	rootTitle, rootHref := section.Title, section.Href
	if group.ID != "" {
		rootTitle, rootHref = group.Title, group.Href
	}
	if rootHref == "/docs/"+document.Slug {
		rootTitle, rootHref = "Documentation", "/docs"
	}
	entry := siteDocument{
		slug:               document.Slug,
		title:              document.Title,
		breadcrumb:         firstNonEmpty(document.Breadcrumb, document.Title),
		breadcrumbRoot:     rootTitle,
		breadcrumbRootHref: rootHref,
		summary:            document.Summary,
		markdown:           string(markdown),
		sectionID:          section.ID,
		groupID:            group.ID,
		source:             document.Source,
		navigationTitle:    document.NavigationTitle,
		generated:          document.Generated,
	}
	loaded.documents = append(loaded.documents, entry)
	loaded.bySlug[entry.slug] = entry
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func documentsInCatalogGroup(sectionID, groupID string, skipFirst bool) []siteDocument {
	for _, section := range siteCatalog.Sections {
		if section.ID != sectionID {
			continue
		}
		for _, group := range section.Groups {
			if group.ID != groupID {
				continue
			}
			documents := make([]siteDocument, 0, len(group.Documents))
			for index, document := range group.Documents {
				if skipFirst && index == 0 {
					continue
				}
				documents = append(documents, siteDocumentsBySlug[document.Slug])
			}
			return documents
		}
	}
	return nil
}

func allSiteDocuments() []siteDocument {
	return siteDocuments
}

func siteDocumentBySlug(slug string) (siteDocument, bool) {
	document, ok := siteDocumentsBySlug[strings.Trim(slug, "/")]
	return document, ok
}

func searchSiteDocuments(query string) []siteDocument {
	matches, err := documentationSearchIndex.Search(context.Background(), query, len(siteDocuments))
	if err != nil {
		panic(fmt.Sprintf("search documentation: %v", err))
	}
	results := make([]siteDocument, 0, len(matches))
	for _, match := range matches {
		document, exists := siteDocumentsBySlug[match.Slug]
		if !exists {
			panic(fmt.Sprintf("documentation search returned unknown slug %q", match.Slug))
		}
		results = append(results, document)
	}
	return results
}

func siteConfigurationSchema(name string) ([]byte, bool) {
	if strings.Contains(name, "/") || !strings.HasSuffix(name, ".schema.json") {
		return nil, false
	}
	schema, err := content.Files.ReadFile("reference/config/schemas/" + name)
	return schema, err == nil
}

func siteOpenAPISpecification() []byte {
	specification, err := content.Files.ReadFile("api/openapi.yaml")
	if err != nil {
		panic(fmt.Sprintf("read generated OpenAPI specification: %v", err))
	}
	return specification
}

var docsChartShortcode = regexp.MustCompile(`\{\{<\s*chart\s+id="([a-z0-9_]+)"\s*>}}`)

func siteDocsArticle(document siteDocument) g.Node {
	source := document.markdown
	shortcodes := docsChartShortcode.FindAllStringSubmatch(source, -1)
	for index, shortcode := range shortcodes {
		id := shortcode[1]
		if !documentHasVisualExample(document.slug, id) {
			panic(fmt.Sprintf("chart shortcode %q is not generated for documentation %s", id, document.slug))
		}
		placeholder := fmt.Sprintf("LIBREDASH_DOCS_CHART_PLACEHOLDER_%d", index)
		source = strings.Replace(source, shortcode[0], placeholder, 1)
	}
	if strings.Contains(source, "{{< chart") {
		panic(fmt.Sprintf("invalid chart shortcode in documentation: %s", document.slug))
	}

	var rendered bytes.Buffer
	if err := markdownRenderer.Convert([]byte(source), &rendered); err != nil {
		panic(fmt.Sprintf("render documentation Markdown: %v", err))
	}
	renderedHTML := rendered.String()
	if reference, ok := visualReferenceForDocument(document.slug); ok {
		firstHeading := strings.Index(renderedHTML, "<h2")
		if firstHeading < 0 {
			panic(fmt.Sprintf("visual documentation has no variation headings: %s", document.slug))
		}
		renderedHTML = renderedHTML[:firstHeading] + renderVisualAPIReference(reference) + renderedHTML[firstHeading:]
	}
	for index, shortcode := range shortcodes {
		placeholderID := fmt.Sprintf("LIBREDASH_DOCS_CHART_PLACEHOLDER_%d", index)
		placeholder := "<p>" + placeholderID + "</p>\n"
		payload, ok := visualExampleForDocument(document.slug, shortcode[1])
		if !ok {
			panic(fmt.Sprintf("generated visual example %q is missing for documentation: %s", shortcode[1], document.slug))
		}
		kind := ""
		if payload.Type == "kpi" {
			kind = ` kind="kpi"`
		}
		exampleReference, ok := visualExampleReferenceForDocument(document.slug, shortcode[1])
		if !ok {
			panic(fmt.Sprintf("generated visual example reference %q is missing for documentation: %s", shortcode[1], document.slug))
		}
		component := fmt.Sprintf("<ld-site-visual-example example-id=\"%s\"%s></ld-site-visual-example>\n%s", shortcode[1], kind, renderVisualKeyFields(exampleReference.KeyFields))
		if !strings.Contains(renderedHTML, placeholder) {
			panic(fmt.Sprintf("render chart shortcode %q for documentation: %s", shortcode[1], document.slug))
		}
		renderedHTML = strings.Replace(renderedHTML, placeholder, component, 1)
	}

	return h.Article(
		h.ID("main-content"),
		h.Class("site-docs-article"),
		h.Div(h.Class("site-docs-article-actions"), g.El("ld-site-markdown-copy", g.Attr("markdown", document.markdown))),
		g.Raw(renderedHTML),
		siteDocsArticleFooter(document),
	)
}

func renderVisualAPIReference(reference visualdocs.DocumentReference) string {
	return `<section class="site-visual-api-summary" aria-labelledby="site-visual-api-summary">` +
		`<h2 id="site-visual-api-summary">API at a glance</h2>` +
		`<dl>` +
		`<dt>Kind</dt><dd>` + renderVisualCodeValues(strings.Split(reference.Kind, ", ")) + `</dd>` +
		`<dt>Renderer</dt><dd>` + renderVisualCodeValues(strings.Split(reference.Renderer, ", ")) + `</dd>` +
		`<dt>Shapes</dt><dd>` + renderVisualCodeValues(reference.Shapes) + `</dd>` +
		`<dt>Query fields</dt><dd>` + renderVisualCodeValues(reference.QueryFields) + `</dd>` +
		`<dt>Options</dt><dd>` + renderVisualCodeValues(reference.Options) + `</dd>` +
		`</dl>` +
		`<p><strong>Accessibility.</strong> ` + stdhtml.EscapeString(reference.Accessibility) + `</p>` +
		`</section>`
}

func renderVisualKeyFields(fields []string) string {
	encoded, err := json.Marshal(fields)
	if err != nil {
		panic(fmt.Sprintf("encode visual key fields: %v", err))
	}
	return `<div class="site-visual-key-fields" aria-label="Key fields" data-key-fields="` + stdhtml.EscapeString(string(encoded)) + `"><strong>Key fields</strong>` + renderVisualCodeValues(fields) + `</div>`
}

func renderVisualCodeValues(values []string) string {
	if len(values) == 0 {
		return `<span>None</span>`
	}
	var rendered strings.Builder
	for _, value := range values {
		rendered.WriteString(`<code>`)
		rendered.WriteString(stdhtml.EscapeString(value))
		rendered.WriteString(`</code>`)
		rendered.WriteByte(' ')
	}
	return strings.TrimSpace(rendered.String())
}

func siteDocsArticleFooter(document siteDocument) g.Node {
	sourceLabel, sourceHref := documentationSourceLink(document)
	return h.Footer(h.Class("site-docs-article-footer"),
		h.Section(h.Class("site-docs-page-meta"), g.Attr("aria-labelledby", "site-docs-about-this-page"),
			h.H2(h.ID("site-docs-about-this-page"), g.Text("About this page")),
			h.Ul(
				h.Li(h.A(h.Href(documentationIssueLink(document)), g.Attr("rel", "external"), g.Text("Report content issue"))),
				h.Li(h.A(h.Href(documentationMarkdownLink(document)), g.Attr("rel", "external"), g.Text("See this page as Markdown"))),
				h.Li(h.A(h.Href(sourceHref), g.Attr("rel", "external"), g.Text(sourceLabel))),
			),
		),
	)
}

func documentationIssueLink(document siteDocument) string {
	query := url.Values{}
	query.Set("title", "Docs: "+document.title)
	query.Set("labels", "documentation")
	query.Set("body", "Page: /docs/"+document.slug+"\n\nDescribe the content issue or suggested improvement.")
	return "https://github.com/Yacobolo/libredash/issues/new?" + query.Encode()
}

func documentationMarkdownLink(document siteDocument) string {
	return "https://raw.githubusercontent.com/Yacobolo/libredash/main/docs/" + document.source
}

func documentationSourceLink(document siteDocument) (string, string) {
	const repository = "https://github.com/Yacobolo/libredash/"
	if !strings.HasPrefix(document.markdown, "<!-- Code generated") {
		return "Edit this page on GitHub", repository + "edit/main/docs/" + document.source
	}
	switch {
	case document.source == "configuration.md":
		return "View source contract on GitHub", repository + "blob/main/internal/configspec/spec.go"
	case strings.HasPrefix(document.source, "reference/config/"):
		return "View source contract on GitHub", repository + "blob/main/internal/configschema/contracts/contracts.cue"
	case strings.HasPrefix(document.source, "reference/cli/"):
		return "View source contract on GitHub", repository + "tree/main/internal/cli"
	case strings.HasPrefix(document.source, "api/"):
		return "View source contract on GitHub", repository + "tree/main/api/typespec"
	default:
		return "View source on GitHub", repository + "blob/main/docs/" + document.source
	}
}
