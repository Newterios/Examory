package submissionfile

import "testing"

func TestUploadBoundary(t *testing.T) {
	for _, pair := range [][2]string{{"../essay.txt", "text/plain"}, {"x.exe", "text/plain"}, {"x.md", "text/plain"}, {"essay.txt", "application/octet-stream"}, {"x\n.txt", "text/plain"}} {
		if ValidName(pair[0], pair[1]) {
			t.Fatal(pair)
		}
	}
	if !ValidName("answer.PY", "text/x-python") || !ValidText([]byte("Қазақша\r\nрубрика\t")) {
		t.Fatal("valid UTF-8 rejected")
	}
	for _, b := range [][]byte{nil, {0}, {1}, {0xff}} {
		if ValidText(b) {
			t.Fatal(b)
		}
	}
}
