package mailer

import (
	"bytes"
	"fmt"
	htmltmpl "html/template"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	texttmpl "text/template"
	"time"

	"github.com/iris-gate/iris-gate/libs/identity/application"
)

// DefaultBrand is the product name used by the base email layout when none is specified.
const DefaultBrand = "Iris Gate"

const htmlFallbackText = "Please view this message in an HTML-capable email client."

// LayoutData holds the inputs for the English base email layout.
type LayoutData struct {
	// Title is the heading shown at the top of the email.
	Title string
	// BodyText is the plain-text content for the text/plain part.
	BodyText string
	// BodyHTML is optional HTML content inserted into the HTML layout. When empty,
	// BodyText is HTML-escaped and rendered as paragraphs.
	BodyHTML htmltmpl.HTML
	// Brand is the product name shown in the header and footer; defaults to DefaultBrand.
	Brand string
}

type htmlLayoutView struct {
	Brand      string
	Title      string
	BodyHTML   htmltmpl.HTML
	Paragraphs []string
}

type textLayoutView struct {
	Brand    string
	Title    string
	BodyText string
}

var (
	baseTextTemplate = texttmpl.Must(texttmpl.New("base_text").Parse(
		`{{.Brand}} - {{.Title}}

{{.BodyText}}

--
This is an automated message from {{.Brand}}. Please do not reply.
`))

	baseHTMLTemplate = htmltmpl.Must(htmltmpl.New("base_html").Parse(
		`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>{{.Title}}</title>
</head>
<body style="margin:0;padding:24px;background-color:#f8fafc;color:#0f172a;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;line-height:1.5;">
  <div style="max-width:560px;margin:0 auto;background-color:#ffffff;border:1px solid #e2e8f0;border-radius:8px;padding:24px;">
    <p style="margin:0 0 8px 0;font-size:12px;font-weight:600;letter-spacing:0.04em;text-transform:uppercase;color:#475569;">{{.Brand}}</p>
    <h1 style="margin:0 0 16px 0;font-size:20px;line-height:1.3;color:#0f172a;">{{.Title}}</h1>
    <div style="font-size:15px;color:#1e293b;">
{{- if .BodyHTML}}
      {{.BodyHTML}}
{{- else}}
{{- range .Paragraphs}}
      <p style="margin:0 0 12px 0;">{{.}}</p>
{{- end}}
{{- end}}
    </div>
    <hr style="border:none;border-top:1px solid #e2e8f0;margin:24px 0 16px 0;">
    <p style="margin:0;font-size:12px;color:#64748b;">This is an automated message from {{.Brand}}. Please do not reply.</p>
  </div>
</body>
</html>
`))
)

// RenderBaseLayout renders the English plain-text and simple HTML base templates.
func RenderBaseLayout(data LayoutData) (string, string, error) {
	brand := strings.TrimSpace(data.Brand)
	if brand == "" {
		brand = DefaultBrand
	}

	bodyText := data.BodyText
	if strings.TrimSpace(bodyText) == "" {
		bodyText = htmlFallbackText
	}

	var textBuf bytes.Buffer
	if err := baseTextTemplate.Execute(&textBuf, textLayoutView{
		Brand:    brand,
		Title:    data.Title,
		BodyText: bodyText,
	}); err != nil {
		return "", "", fmt.Errorf("render text layout: %w", err)
	}

	var paragraphs []string
	if data.BodyHTML == "" {
		paragraphs = splitParagraphs(bodyText)
	}

	var htmlBuf bytes.Buffer
	if err := baseHTMLTemplate.Execute(&htmlBuf, htmlLayoutView{
		Brand:      brand,
		Title:      data.Title,
		BodyHTML:   data.BodyHTML,
		Paragraphs: paragraphs,
	}); err != nil {
		return "", "", fmt.Errorf("render html layout: %w", err)
	}

	return textBuf.String(), htmlBuf.String(), nil
}

func splitParagraphs(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	rawParts := strings.Split(normalized, "\n\n")
	out := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(normalized)}
	}
	return out
}

func isFullHTMLDocument(html string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(html))
	return strings.HasPrefix(trimmed, "<!doctype") || strings.HasPrefix(trimmed, "<html")
}

// buildMIMEMessage constructs an RFC 5322 multipart/alternative message with
// quoted-printable text/plain and text/html parts.
func buildMIMEMessage(from, to mail.Address, msg application.Message, now time.Time) ([]byte, error) {
	textBody, htmlBody, err := prepareBodies(msg)
	if err != nil {
		return nil, err
	}

	var bodyBuf bytes.Buffer
	mw := multipart.NewWriter(&bodyBuf)

	if err := writeQuotedPrintablePart(mw, `text/plain; charset="utf-8"`, textBody); err != nil {
		return nil, err
	}
	if err := writeQuotedPrintablePart(mw, `text/html; charset="utf-8"`, htmlBody); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "From: %s\r\n", from.String())
	fmt.Fprintf(&out, "To: %s\r\n", to.String())
	fmt.Fprintf(&out, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&out, "Date: %s\r\n", now.UTC().Format(time.RFC1123Z))
	out.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&out, "Content-Type: multipart/alternative; boundary=%q\r\n", mw.Boundary())
	out.WriteString("\r\n")
	out.Write(bodyBuf.Bytes())

	return out.Bytes(), nil
}

func prepareBodies(msg application.Message) (string, string, error) {
	if msg.HTML != "" && isFullHTMLDocument(msg.HTML) {
		textBody := msg.Text
		if strings.TrimSpace(textBody) == "" {
			textBody = htmlFallbackText
		}
		return textBody, msg.HTML, nil
	}
	// LayoutData.BodyHTML expects trusted HTML produced by application templates.
	// Plain-text-only messages leave BodyHTML empty and are auto-escaped by html/template.
	return RenderBaseLayout(LayoutData{
		Title:    msg.Subject,
		BodyText: msg.Text,
		BodyHTML: htmltmpl.HTML(msg.HTML),
	})
}

func writeQuotedPrintablePart(mw *multipart.Writer, contentType, content string) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", contentType)
	header.Set("Content-Transfer-Encoding", "quoted-printable")

	part, err := mw.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create mime part (%s): %w", contentType, err)
	}
	qp := quotedprintable.NewWriter(part)
	if _, err := qp.Write([]byte(content)); err != nil {
		return fmt.Errorf("write mime part (%s): %w", contentType, err)
	}
	if err := qp.Close(); err != nil {
		return fmt.Errorf("close mime part (%s): %w", contentType, err)
	}
	return nil
}
