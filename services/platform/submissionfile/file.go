// Package submissionfile defines the immutable UTF-8 upload contract shared by files and assignments.
package submissionfile

import (
	"bytes"
	"crypto/subtle"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 2 << 20
const MaxFiles = 4

var extensions = map[string][]string{"text/plain": {".txt", ".go", ".py", ".java", ".c", ".cpp", ".h", ".js", ".ts", ".json", ".csv"}, "text/markdown": {".md"}, "text/x-python": {".py"}, "text/x-java-source": {".java"}}

func Supported(mime string) bool { _, ok := extensions[mime]; return ok }
func ValidName(name, mime string) bool {
	if !utf8.ValidString(name) || len(name) == 0 || len(name) > 200 || strings.ContainsAny(name, "/\\\r\n\x00") || strings.TrimSpace(name) != name {
		return false
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return false
		}
	}
	for _, ext := range extensions[mime] {
		if strings.ToLower(path.Ext(name)) == ext {
			return true
		}
	}
	return false
}
func ValidText(b []byte) bool {
	return len(b) > 0 && utf8.Valid(b) && !bytes.ContainsRune(b, 0) && !bytes.ContainsFunc(b, func(r rune) bool { return r < 32 && r != '\r' && r != '\n' && r != '\t' })
}

type Ref struct {
	ID     string `json:"file_id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type File struct {
	Ref
	MIME string `json:"mime"`
	Text string `json:"text"`
}

func Internal(r *http.Request, token string) bool {
	return token != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) == 1
}
