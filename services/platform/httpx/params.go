package httpx

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Newterios/Examory/services/platform/ids"
)

// MaxPageSize is the upper bound for any list endpoint.
const MaxPageSize = 100

// Page reads ?limit= (1..MaxPageSize, default def) and ?cursor= (a UUID,
// keyset pagination over UUIDv7 ids). It writes a 400 problem and returns
// ok=false on invalid input.
func Page(c *gin.Context, def int) (limit int, cursor string, ok bool) {
	limit = def
	if s := c.Query("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > MaxPageSize {
			Fail(c, http.StatusBadRequest, "invalid_parameter", "limit must be between 1 and 100")
			return 0, "", false
		}
		limit = n
	}
	if cursor = c.Query("cursor"); cursor != "" {
		canon, err := ids.Canonical(cursor)
		if err != nil {
			Fail(c, http.StatusBadRequest, "invalid_parameter", "cursor is invalid")
			return 0, "", false
		}
		cursor = canon
	}
	return limit, cursor, true
}

// PathID reads a UUID path parameter, writing 404 when it is malformed (a
// malformed id cannot name an existing resource).
func PathID(c *gin.Context, name string) (string, bool) {
	id, err := ids.Canonical(c.Param(name))
	if err != nil {
		Fail(c, http.StatusNotFound, "not_found", "resource not found")
		return "", false
	}
	return id, true
}
