package errors

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultTemplatesMirrorWebPages(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine caller path")
	}

	root := filepath.Join(filepath.Dir(file), "..", "..", "web", "pages")

	cases := []struct {
		name     string
		source   string
		filename string
	}{
		{name: "404", source: default404Source, filename: "404.html"},
		{name: "500", source: default500Source, filename: "500.html"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(root, tc.filename))
			if err != nil {
				t.Fatalf("read %s: %v", tc.filename, err)
			}

			if string(want) != tc.source {
				t.Fatalf("embedded template for %s does not match web/pages/%s", tc.name, tc.filename)
			}
		})
	}
}
