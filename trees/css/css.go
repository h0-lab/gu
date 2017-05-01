package css

import (
	"bytes"
	"strings"
	"text/template"

	bcss "github.com/aymerick/douceur/css"
	"github.com/aymerick/douceur/parser"
)

var (
	helpers = template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"multiply": func(a, b int) int {
			return a * b
		},
		"subtract": func(a, b int) int {
			return b - a
		},
	}
)

// Rule defines the a single css rule which will be transformed and
// converted into a usable stylesheet during rendering.
type Rule struct {
	template  *template.Template
	feed      *Rule
	feedStyle *bcss.Stylesheet
	depends   []*Rule
}

// New returns a new instance of a Rule which provides capability to parse
// and extrapolate the giving content using the provided binding.
// - Arguments:
// 		- rules : text containing css values.
//		- extension: A instance of a Rule, that may contain certain styles which can be extended into current rule styles using the `extend` template function.
// 		- rules: A slice of rules which should be built with this, they will also inherit this rules parents, a nice way to
// 				extend a rule sets property.
func New(rules string, extension *Rule, rs ...*Rule) *Rule {
	rsc := &Rule{depends: rs, feed: extension}

	tmp, err := template.New("css").Funcs(helpers).Funcs(template.FuncMap{
		"extend": rsc.extend,
	}).Parse(rules)

	if err != nil {
		panic(err)
	}

	rsc.template = tmp
	return rsc
}

// extend attempts to pull a giving set of classes and assigns into
// a target class.
func (r *Rule) extend(item string) string {
	if r.feedStyle == nil {
		return ""
	}

	var attrs []string

	for _, rule := range r.feedStyle.Rules {
		if rule.Prelude != item {
			continue
		}

		for _, prop := range rule.Declarations {
			if prop.Important {
				attrs = append(attrs, prop.StringWithImportant(prop.Important))
			} else {
				attrs = append(attrs, prop.String())
			}
		}
		break
	}

	return strings.Join(attrs, "\n")
}

// Stylesheet returns the provided styles using the binding as the argument for the
// provided css template.
func (r *Rule) Stylesheet(bind interface{}, parentNode string) (*bcss.Stylesheet, error) {
	if r.feed != nil {
		sheet, err := r.feed.Stylesheet(bind, parentNode)
		if err != nil {
			return nil, err
		}

		r.feedStyle = sheet
	}

	var stylesheet bcss.Stylesheet

	{
		for _, rule := range r.depends {
			sheet, err := rule.Stylesheet(bind, parentNode)
			if err != nil {
				return nil, err
			}

			stylesheet.Rules = append(stylesheet.Rules, sheet.Rules...)
		}
	}

	var content bytes.Buffer
	if err := r.template.Execute(&content, bind); err != nil {
		return nil, err
	}

	sheet, err := parser.Parse(content.String())
	if err != nil {
		return nil, err
	}

	for _, rule := range sheet.Rules {
		r.morphRule(rule, parentNode)
	}

	stylesheet.Rules = append(stylesheet.Rules, sheet.Rules...)

	return &stylesheet, nil
}

// adjustName adjust the provided name according to the set rules of for specific
// css selectors.
func (r *Rule) adjustName(sel string, parentNode string) string {
	sel = strings.TrimSpace(sel)

	switch {
	case strings.Contains(sel, "&"):
		return strings.Replace(sel, "&", parentNode, -1)

	case strings.HasPrefix(sel, ":"):
		return parentNode + "" + sel

	default:
		return sel
	}
}

// morphRules adjusts the provided rules with the parent selector.
func (r *Rule) morphRule(base *bcss.Rule, parentNode string) {
	for index, sel := range base.Selectors {
		base.Selectors[index] = r.adjustName(sel, parentNode)
	}

	for _, rule := range base.Rules {
		if rule.Kind == bcss.AtRule {
			r.morphRule(rule, parentNode)
			continue
		}

		for index, sel := range rule.Selectors {
			rule.Selectors[index] = r.adjustName(sel, parentNode)
		}
	}
}
