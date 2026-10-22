package cmd

import (
	stdhtml "html"
	"strings"

	nethtml "golang.org/x/net/html"

	"github.com/openclaw/gogcli/internal/gmailcontent"
)

// Conversions between plain text and HTML, for the author's text and for
// text taken from the original message.

// composeText is the user's text above a quote or a forwarded message (a quoted
// reply's body, which by then includes any signature, or a forward's note) in
// whichever forms were given. A form that was given comes back as given; a
// missing one is filled in from the other and trimmed. Whitespace-only counts
// as missing.
type composeText struct {
	plain string
	html  string
}

// hasHTML reports whether HTML was given.
func (t composeText) hasHTML() bool {
	return strings.TrimSpace(t.html) != ""
}

// plainVersion returns the plain text, derived from the HTML when only HTML
// was given.
func (t composeText) plainVersion() string {
	if strings.TrimSpace(t.plain) == "" && t.hasHTML() {
		return htmlToPlainText(t.html)
	}
	return t.plain
}

// htmlVersion returns the HTML, or the plain text escaped with its line breaks
// kept when only plain text was given.
func (t composeText) htmlVersion() string {
	if !t.hasHTML() {
		return escapeTextToHTML(strings.TrimSpace(t.plain))
	}
	return t.html
}

// htmlOnlyWithoutText reports whether the text was given only as HTML with no
// plain-text rendering, such as a lone image, so a plain part could not carry it.
func (t composeText) htmlOnlyWithoutText() bool {
	return t.hasHTML() && strings.TrimSpace(t.plainVersion()) == ""
}

func escapeTextToHTML(value string) string {
	value = stdhtml.EscapeString(value)
	return strings.ReplaceAll(value, "\n", "<br>\n")
}

func htmlToPlainText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	doc, err := nethtml.Parse(strings.NewReader(value))
	if err != nil {
		return gmailcontent.StripHTMLTags(value)
	}

	var out strings.Builder
	var walk func(*nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n == nil {
			return
		}
		switch n.Type {
		case nethtml.TextNode:
			out.WriteString(n.Data)
		case nethtml.ElementNode:
			if hiddenHTMLElement(n) {
				return
			}
			switch strings.ToLower(n.Data) {
			case "head", literalStyle, "script", "template", "noscript", literalTitle:
				return
			case "br":
				writeHTMLNewline(&out)
				return
			case "div", "p", "li":
				writeHTMLNewline(&out)
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					walk(child)
				}
				writeHTMLNewline(&out)
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	lines := strings.Split(stdhtml.UnescapeString(out.String()), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	return strings.Join(kept, "\n")
}

func hiddenHTMLElement(n *nethtml.Node) bool {
	for _, attr := range n.Attr {
		switch strings.ToLower(attr.Key) {
		case "hidden":
			return true
		case "aria-hidden":
			if strings.EqualFold(strings.TrimSpace(attr.Val), "true") {
				return true
			}
		case "type":
			if strings.EqualFold(n.Data, "input") && strings.EqualFold(strings.TrimSpace(attr.Val), "hidden") {
				return true
			}
		case literalStyle:
			if hiddenInlineStyle(attr.Val) {
				return true
			}
		}
	}
	return false
}

func hiddenInlineStyle(value string) bool {
	for declaration := range strings.SplitSeq(value, ";") {
		property, setting, ok := strings.Cut(declaration, ":")
		if !ok {
			continue
		}
		property = strings.ToLower(strings.TrimSpace(property))
		setting = strings.ToLower(strings.TrimSpace(setting))
		switch property {
		case "display", "visibility":
			if strings.HasPrefix(setting, "none") || strings.HasPrefix(setting, "hidden") {
				return true
			}
		case "mso-hide":
			if strings.HasPrefix(setting, "all") {
				return true
			}
		}
	}
	return false
}

func writeHTMLNewline(out *strings.Builder) {
	if out.Len() == 0 {
		return
	}
	if !strings.HasSuffix(out.String(), "\n") {
		out.WriteByte('\n')
	}
}
