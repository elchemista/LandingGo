package contact

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mailgun "github.com/mailgun/mailgun-go/v5"

	"github.com/elchemista/LandingGo/internal/config"
)

func TestServiceSendSuccess(t *testing.T) {
	received := make(chan url.Values, 1)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "api" || password != "key" {
			t.Fatalf("unexpected basic auth: %s %s", username, password)
		}

		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/x-www-form-urlencoded") && !strings.HasPrefix(ct, "multipart/form-data") {
			t.Fatalf("unexpected content type: %s", ct)
		}

		if path := r.URL.Path; path != "/v3/mg.example.com/messages" && path != "/mg.example.com/messages" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}

		if strings.HasPrefix(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatalf("parse multipart form: %v", err)
			}
		} else if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		values := url.Values{}
		for key, vals := range r.PostForm {
			for _, v := range vals {
				values.Add(key, v)
			}
		}
		if len(values) == 0 && r.MultipartForm != nil {
			for key, vals := range r.MultipartForm.Value {
				for _, v := range vals {
					values.Add(key, v)
				}
			}
		}

		received <- values
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"123","message":"Queued"}`))
	}))
	t.Cleanup(ts.Close)

	cfg := config.Contact{
		Recipient: "owners@example.com",
		From:      "Landing Page <no-reply@example.com>",
		Subject:   "Website enquiry",
		Mailgun:   config.Mailgun{Domain: "mg.example.com", APIKey: "key"},
	}

	mg := mailgun.NewMailgun(cfg.Mailgun.APIKey)
	mg.SetHTTPClient(ts.Client())
	if err := mg.SetAPIBase(ts.URL); err != nil {
		t.Fatalf("set api base: %v", err)
	}

	svc := NewService(cfg, mg)

	if err := svc.Send(context.Background(), Message{Name: "Jane", Email: "jane@example.com", Body: "Hi"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	form := <-received

	if got := form.Get("to"); got != "owners@example.com" {
		t.Fatalf("unexpected recipient: %s", got)
	}
	if got := form.Get("from"); got != "Landing Page <no-reply@example.com>" {
		t.Fatalf("unexpected from: %s", got)
	}
	if !strings.Contains(form.Get("subject"), "Website enquiry") {
		t.Fatalf("unexpected subject: %s", form.Get("subject"))
	}
	if !strings.Contains(form.Get("text"), "Jane") {
		t.Fatalf("plain text body missing name: %s", form.Get("text"))
	}
}

func TestServiceSendDisabled(t *testing.T) {
	svc := NewService(config.Contact{}, nil)
	if err := svc.Send(context.Background(), Message{Name: "a", Email: "b@example.com", Body: "c"}); err == nil {
		t.Fatal("expected error for disabled service")
	}
}
