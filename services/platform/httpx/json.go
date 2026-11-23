package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// DefaultMaxJSONBytes is the default request body cap for JSON endpoints.
const DefaultMaxJSONBytes = 1 << 20

// DecodeError is returned by DecodeJSON; Status is 400, 413 or 415.
type DecodeError struct {
	Status int
	Code   string
	Msg    string
}

func (e *DecodeError) Error() string { return e.Msg }

// DecodeJSON strictly decodes one JSON object from r into dst:
//   - Content-Type must be application/json (charset utf-8 allowed);
//   - the body is capped at maxBytes;
//   - unknown fields are rejected;
//   - the body must be exactly one JSON object (no trailing data, no null,
//     no arrays or scalars at top level).
func DecodeJSON(r *http.Request, dst any, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxJSONBytes
	}
	mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return &DecodeError{http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json"}
	}
	if cs, ok := params["charset"]; ok && !strings.EqualFold(cs, "utf-8") {
		return &DecodeError{http.StatusUnsupportedMediaType, "unsupported_media_type", "charset must be utf-8"}
	}
	if r.ContentLength > maxBytes {
		return &DecodeError{http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large"}
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return &DecodeError{http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large"}
		}
		return &DecodeError{http.StatusBadRequest, "malformed_json", "could not read request body"}
	}
	if int64(len(body)) > maxBytes {
		return &DecodeError{http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large"}
	}
	if !utf8.Valid(body) {
		return &DecodeError{http.StatusBadRequest, "malformed_json", "body must be valid UTF-8"}
	}
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return &DecodeError{http.StatusBadRequest, "malformed_json", "body must be a single JSON object"}
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var ute *json.UnmarshalTypeError
		switch {
		case errors.As(err, &ute):
			return &DecodeError{http.StatusBadRequest, "malformed_json", "field " + ute.Field + " has the wrong type"}
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			msg := strings.TrimPrefix(err.Error(), "json: ")
			if len(msg) > 100 {
				msg = msg[:100]
			}
			return &DecodeError{http.StatusBadRequest, "unknown_field", msg}
		default:
			return &DecodeError{http.StatusBadRequest, "malformed_json", "body is not valid JSON"}
		}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return &DecodeError{http.StatusBadRequest, "malformed_json", "body must contain a single JSON object"}
	}
	return nil
}

// BindJSON decodes into dst and writes a problem response on failure.
// It returns false when the handler must stop.
func BindJSON(c *gin.Context, dst any, maxBytes int64) bool {
	if err := DecodeJSON(c.Request, dst, maxBytes); err != nil {
		var de *DecodeError
		if errors.As(err, &de) {
			Fail(c, de.Status, de.Code, de.Msg)
		} else {
			Fail(c, http.StatusBadRequest, "malformed_json", "invalid request body")
		}
		return false
	}
	return true
}
