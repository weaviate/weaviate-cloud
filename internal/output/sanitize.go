package output

import (
	"regexp"
	"strings"
)

var (
	ansiCSI = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	ansiOSC = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
)

const (
	c0Max     = 0x1F
	delByte   = 0x7F
	c1RuneMin = 0x80
	c1RuneMax = 0x9F

	softHyphen = 0x00AD

	zeroWidthMin = 0x200B
	zeroWidthMax = 0x200F

	bidiEmbeddingMin = 0x202A
	bidiEmbeddingMax = 0x202E

	bidiIsolateMin = 0x2066
	bidiIsolateMax = 0x2069

	byteOrderMark = 0xFEFF
)

func SanitizeText(s string) string {
	s = ansiCSI.ReplaceAllString(s, "")
	s = ansiOSC.ReplaceAllString(s, "")

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if isStrippedControlRune(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isStrippedControlRune(r rune) bool {
	switch {
	case r <= c0Max:
		return true
	case r == delByte:
		return true
	case r >= c1RuneMin && r <= c1RuneMax:
		return true
	case r == softHyphen:
		return true
	case r >= zeroWidthMin && r <= zeroWidthMax:
		return true
	case r >= bidiEmbeddingMin && r <= bidiEmbeddingMax:
		return true
	case r >= bidiIsolateMin && r <= bidiIsolateMax:
		return true
	case r == byteOrderMark:
		return true
	default:
		return false
	}
}
