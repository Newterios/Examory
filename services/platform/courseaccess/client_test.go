package courseaccess

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestCourseAndGroupAuthorization(t *testing.T) {
	response := map[string]string{"teacher_id": `{"allowed":true}`, "group_id": `{"allowed":false}`}
	var selectors []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != "GET" || r.URL.Path != "/internal/v1/access" || r.Header.Get("Authorization") != "Bearer test" || q.Get("course_id") != "course" || len(q) != 2 {
			t.Error("incorrect access request")
		}
		for _, key := range []string{"teacher_id", "group_id"} {
			if q.Get(key) != "" {
				selectors = append(selectors, key)
				_, _ = w.Write([]byte(response[key]))
				return
			}
		}
		t.Error("missing selector")
	}))
	defer server.Close()
	client := New(server.URL, "test")
	ok, err := client.Allowed(context.Background(), "course", "teacher", "group")
	if err != nil || ok || !reflect.DeepEqual(selectors, []string{"teacher_id", "group_id"}) {
		t.Fatal(ok, err, selectors)
	}
	for _, bad := range []string{"{}", `{"allowed":true} trailing`, `{"allowed":null}`} {
		response["group_id"] = bad
		ok, err = client.Allowed(context.Background(), "course", "", "group")
		if ok || !errors.Is(err, ErrUnavailable) {
			t.Fatal(bad, ok, err)
		}
	}
	response["group_id"] = `{"allowed":true}`
	ok, err = client.Allowed(context.Background(), "course", "", "group")
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	server.Close()
	ok, err = client.Allowed(context.Background(), "course", "", "group")
	if ok || !errors.Is(err, ErrUnavailable) {
		t.Fatal(ok, err)
	}
}
