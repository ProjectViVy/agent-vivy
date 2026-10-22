package provider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestD14ResolvingCacheKeepsCredentialRotationAndReadiness(t *testing.T) {
	var auth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"reply","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	vendor := testOpenAIVendor(server.URL)
	firstSpec := LiveSpec{Provider: vendor.Name, Model: "gpt-4o", BaseURL: server.URL, APIKey: "key-A", Ready: true}
	model := NewResolvingChatModel(testModelHost(t, vendor), NewCatalog(vendor), staticSpecSource{live: firstSpec}).(*resolvingChatModel)
	ctx := context.Background()
	first, err := model.inner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	again, err := model.inner(ctx)
	if err != nil || first != again {
		t.Fatal("unchanged exact model selection did not reuse the cache")
	}
	if _, err := model.Generate(ctx, []*schema.AgenticMessage{testUser("hi")}); err != nil {
		t.Fatal(err)
	}
	rotated := firstSpec
	rotated.APIKey = "key-B"
	model.src = staticSpecSource{live: rotated}
	second, err := model.inner(ctx)
	if err != nil || first == second {
		t.Fatal("same-length credential rotation reused an old model")
	}
	if _, err := model.Generate(ctx, []*schema.AgenticMessage{testUser("hi")}); err != nil {
		t.Fatal(err)
	}
	if len(auth) != 2 || auth[0] != "Bearer key-A" || auth[1] != "Bearer key-B" {
		t.Fatal("outbound authorization did not follow current credential selection")
	}
	rotated.Ready = false
	model.src = staticSpecSource{live: rotated}
	_, err = model.inner(ctx)
	var missing *KeyMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("cached model bypassed missing credential state: %v", err)
	}
	if len(auth) != 2 {
		t.Fatal("unready selection reached the provider")
	}
}
