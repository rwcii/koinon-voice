package summarize

import (
	"regexp"
	"strings"
)

// Kinds of text a listener should not hear read out, in the order NotForSpeech reports them.
const (
	Hash     = "hash"
	Path     = "path"
	URL      = "url"
	Markdown = "markdown"
)

var (
	// A hash is seven or more hexadecimal characters with at least one digit and one letter.
	hexWord = regexp.MustCompile(`\b[0-9a-fA-F]{7,64}\b`)
	// A path starts with /, ~/ or ./, has two slashes, or ends in a file extension.
	pathWord = regexp.MustCompile(`(^|[\s(])(~|\.\.?)?/[\w.-]+|\b[\w-]+(/[\w.-]+){2,}|\b[\w./-]+\.(go|md|json|toml|ya?ml|sh|swift|py|txt|onnx)\b`)
	urlWord  = regexp.MustCompile(`\bhttps?://\S+`)
	// Markdown is a list or heading marker at a line start, an asterisk (a list marker or
	// emphasis) or code.
	markdownMark = regexp.MustCompile("(?m)^\\s*([-+]|#+)\\s|\\*|`")
)

// NotForSpeech reports the kinds of text in a summary that should not be read aloud: commit
// hashes, file paths, URLs and Markdown. A summary is spoken as written, so each is heard
// character by character or as a symbol's name.
func NotForSpeech(text string) []string {
	var kinds []string
	for _, w := range hexWord.FindAllString(text, -1) {
		if strings.ContainsAny(w, "0123456789") && strings.ContainsAny(strings.ToLower(w), "abcdef") {
			kinds = append(kinds, Hash)
			break
		}
	}
	// A URL's own slashes are not a path.
	if pathWord.MatchString(urlWord.ReplaceAllString(text, " ")) {
		kinds = append(kinds, Path)
	}
	if urlWord.MatchString(text) {
		kinds = append(kinds, URL)
	}
	if markdownMark.MatchString(text) {
		kinds = append(kinds, Markdown)
	}
	return kinds
}
