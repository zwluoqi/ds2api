package toolstream

import "strings"

func findMatchingXMLToolWrapperClose(s, openTag, closeTag string, openIdx int) int {
	if s == "" || openTag == "" || closeTag == "" || openIdx < 0 {
		return -1
	}
	lower := strings.ToLower(s)
	openTarget := strings.ToLower(openTag)
	closeTarget := strings.ToLower(closeTag)
	depth := 1
	for i := openIdx + len(openTarget); i < len(s); {
		switch {
		case strings.HasPrefix(lower[i:], "<![cdata["):
			end := strings.Index(lower[i+len("<![cdata["):], "]]>")
			if end < 0 {
				return -1
			}
			i += len("<![cdata[") + end + len("]]>")
		case strings.HasPrefix(lower[i:], "<!--"):
			end := strings.Index(lower[i+len("<!--"):], "-->")
			if end < 0 {
				return -1
			}
			i += len("<!--") + end + len("-->")
		case strings.HasPrefix(lower[i:], closeTarget):
			depth--
			if depth == 0 {
				return i
			}
			i += len(closeTarget)
		case strings.HasPrefix(lower[i:], openTarget) && hasXMLToolTagBoundary(s, i+len(openTarget)):
			depth++
			i += len(openTarget)
		default:
			i++
		}
	}
	return -1
}

func findXMLOpenOutsideCDATA(s, openTag string, start int) int {
	if s == "" || openTag == "" {
		return -1
	}
	if start < 0 {
		start = 0
	}
	lower := strings.ToLower(s)
	target := strings.ToLower(openTag)
	for i := start; i < len(s); {
		switch {
		case strings.HasPrefix(lower[i:], "<![cdata["):
			end := strings.Index(lower[i+len("<![cdata["):], "]]>")
			if end < 0 {
				return -1
			}
			i += len("<![cdata[") + end + len("]]>")
		case strings.HasPrefix(lower[i:], "<!--"):
			end := strings.Index(lower[i+len("<!--"):], "-->")
			if end < 0 {
				return -1
			}
			i += len("<!--") + end + len("-->")
		case strings.HasPrefix(lower[i:], target) && hasXMLToolTagBoundary(s, i+len(target)):
			return i
		default:
			i++
		}
	}
	return -1
}

func findXMLCloseOutsideCDATA(s, closeTag string, start int) int {
	if s == "" || closeTag == "" {
		return -1
	}
	if start < 0 {
		start = 0
	}
	lower := strings.ToLower(s)
	target := strings.ToLower(closeTag)
	for i := start; i < len(s); {
		switch {
		case strings.HasPrefix(lower[i:], "<![cdata["):
			end := strings.Index(lower[i+len("<![cdata["):], "]]>")
			if end < 0 {
				return -1
			}
			i += len("<![cdata[") + end + len("]]>")
		case strings.HasPrefix(lower[i:], "<!--"):
			end := strings.Index(lower[i+len("<!--"):], "-->")
			if end < 0 {
				return -1
			}
			i += len("<!--") + end + len("-->")
		case strings.HasPrefix(lower[i:], target):
			return i
		default:
			i++
		}
	}
	return -1
}

func hasXMLToolTagBoundary(text string, idx int) bool {
	if idx >= len(text) {
		return true
	}
	switch text[idx] {
	case ' ', '\t', '\n', '\r', '>', '/':
		return true
	default:
		return false
	}
}

func findXMLTagEnd(s string, start int) int {
	quote := byte(0)
	for i := start; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '>' {
			return i
		}
	}
	return -1
}
