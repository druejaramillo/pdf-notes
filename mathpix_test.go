package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMathpixConvert(t *testing.T) {
	var deleted bool
	var progress []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.Method + " " + request.URL.Path {
		case "POST /v3/pdf":
			if request.Header.Get("app_id") != "test-id" || request.Header.Get("app_key") != "test-key" {
				t.Error("Mathpix credentials were not sent")
			}
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("parse upload form: %v", err)
			}
			if got := request.FormValue("options_json"); got != `{"idiomatic_eqn_arrays":true}` {
				t.Errorf("options_json = %q", got)
			}
			file, _, err := request.FormFile("file")
			if err != nil {
				t.Fatalf("read uploaded file: %v", err)
			}
			defer file.Close()
			if contents, _ := io.ReadAll(file); string(contents) != "PDF" {
				t.Errorf("uploaded PDF = %q", contents)
			}
			response.Write([]byte(`{"pdf_id":"pdf-123"}`))
		case "GET /v3/pdf/pdf-123":
			response.Write([]byte(`{"status":"completed"}`))
		case "GET /v3/pdf/pdf-123.mmd":
			response.Write([]byte("\\section{Notes}\nText."))
		case "DELETE /v3/pdf/pdf-123":
			deleted = true
			response.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "chapter.pdf")
	if err := os.WriteFile(path, []byte("PDF"), 0o644); err != nil {
		t.Fatalf("create PDF fixture: %v", err)
	}
	client := mathpixClient{
		appID:        "test-id",
		appKey:       "test-key",
		baseURL:      server.URL + "/v3",
		httpClient:   server.Client(),
		pollInterval: time.Millisecond,
		report: func(message string) {
			progress = append(progress, message)
		},
	}

	markdown, err := client.convert(context.Background(), path)
	if err != nil {
		t.Fatalf("convert() error = %v", err)
	}
	if markdown != "\\section{Notes}\nText." {
		t.Fatalf("convert() = %q", markdown)
	}
	if !deleted {
		t.Fatal("convert() did not delete the uploaded Mathpix PDF")
	}
	if len(progress) < 4 {
		t.Fatalf("convert() reported %d progress messages, want at least 4", len(progress))
	}
}
