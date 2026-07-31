package tools

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// extractDocxMarkdown converts the main document part of a .docx archive
// into markdown-ish text: headings, list items, tables, and plain
// paragraphs. Formatting beyond that (bold, images, nested tables) is
// dropped — the goal is readable content for the model, not fidelity.
func extractDocxMarkdown(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a valid docx archive: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open document part: %w", err)
		}
		defer rc.Close()
		return docxMarkdownFromXML(rc)
	}
	return "", fmt.Errorf("docx archive has no word/document.xml part")
}

func docxMarkdownFromXML(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)

	var (
		out     strings.Builder
		para    strings.Builder
		cell    strings.Builder
		row     []string
		heading int
		list    bool
		inText  bool
		inTable bool
		tblRows int
	)

	// Runs land in the current table cell when inside a table, otherwise
	// in the current paragraph.
	target := func() *strings.Builder {
		if inTable {
			return &cell
		}
		return &para
	}

	flushPara := func() {
		text := strings.TrimSpace(para.String())
		para.Reset()
		prefix := ""
		switch {
		case heading > 0:
			prefix = strings.Repeat("#", min(heading, 6)) + " "
		case list:
			prefix = "- "
		}
		heading, list = 0, false
		if text == "" {
			return
		}
		out.WriteString(prefix)
		out.WriteString(text)
		out.WriteString("\n\n")
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to parse document part: %w", err)
		}

		switch el := tok.(type) {
		case xml.StartElement:
			switch el.Name.Local {
			case "tbl":
				inTable = true
				tblRows = 0
			case "tr":
				row = row[:0]
			case "tc":
				cell.Reset()
			case "pStyle":
				if lvl := docxHeadingLevel(docxAttr(el, "val")); lvl > 0 {
					heading = lvl
				}
			case "numPr":
				list = true
			case "t":
				inText = true
			case "tab":
				target().WriteString("\t")
			case "br", "cr":
				target().WriteString("\n")
			}
		case xml.EndElement:
			switch el.Name.Local {
			case "t":
				inText = false
			case "p":
				if inTable {
					// Paragraph breaks inside a cell collapse to a space.
					cell.WriteString(" ")
				} else {
					flushPara()
				}
			case "tc":
				row = append(row, strings.TrimSpace(cell.String()))
				cell.Reset()
			case "tr":
				out.WriteString("| " + strings.Join(row, " | ") + " |\n")
				tblRows++
				if tblRows == 1 {
					out.WriteString("|" + strings.Repeat(" --- |", len(row)) + "\n")
				}
			case "tbl":
				inTable = false
				out.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				target().Write(el)
			}
		}
	}

	flushPara()
	return strings.TrimSpace(out.String()), nil
}

func docxHeadingLevel(style string) int {
	s := strings.ToLower(strings.TrimSpace(style))
	if s == "title" {
		return 1
	}
	if rest, ok := strings.CutPrefix(s, "heading"); ok {
		if n, err := strconv.Atoi(rest); err == nil && n >= 1 {
			return n
		}
	}
	return 0
}

func docxAttr(el xml.StartElement, local string) string {
	for _, attr := range el.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}
