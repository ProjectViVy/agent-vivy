package runtime

import (
	"errors"
	"testing"

	"agent-vivy/internal/domain"
)

func TestNormalizeFileContextsClonesBoundedTextSnapshots(t *testing.T) {
	body := []byte("package main\n")
	got, err := normalizeFileContexts([]domain.FileContext{{
		Path: "cmd/main.go", Name: "main.go", Size: int64(len(body)), Content: body,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || string(got[0].Content) != string(body) || got[0].Size != int64(len(body)) {
		t.Fatalf("normalized snapshot = %+v", got)
	}
	body[0] = 'X'
	if string(got[0].Content) != "package main\n" {
		t.Fatalf("normalized content aliases caller body: %q", got[0].Content)
	}
}

func TestNormalizeFileContextsKeepsEmptyBodyNonNil(t *testing.T) {
	got, err := normalizeFileContexts([]domain.FileContext{{Path: "empty.txt", Name: "empty.txt", Content: []byte{}, Size: 0}})
	if err != nil || len(got) != 1 || got[0].Content == nil {
		t.Fatalf("empty context = %+v/%v", got, err)
	}
}

func TestNormalizeFileContextsRejectsBypassInputs(t *testing.T) {
	cases := []struct {
		name string
		item domain.FileContext
		want error
	}{
		{name: "parent", item: domain.FileContext{Path: "../main.go", Content: []byte("x"), Size: 1}, want: errFileContextPath},
		{name: "absolute", item: domain.FileContext{Path: "/main.go", Content: []byte("x"), Size: 1}, want: errFileContextPath},
		{name: "sensitive", item: domain.FileContext{Path: ".env", Content: []byte("x"), Size: 1}, want: errFileContextSensitive},
		{name: "password", item: domain.FileContext{Path: "password", Content: []byte("x"), Size: 1}, want: errFileContextSensitive},
		{name: "token", item: domain.FileContext{Path: "token", Content: []byte("x"), Size: 1}, want: errFileContextSensitive},
		{name: "keys", item: domain.FileContext{Path: "keys.txt", Content: []byte("x"), Size: 1}, want: errFileContextSensitive},
		{name: "naked keys", item: domain.FileContext{Path: "keys", Content: []byte("x"), Size: 1}, want: errFileContextSensitive},
		{name: "binary", item: domain.FileContext{Path: "main.go", Content: []byte{0, 1}, Size: 2}, want: errFileContextBinary},
		{name: "wrong size", item: domain.FileContext{Path: "main.go", Content: []byte("x"), Size: 2}, want: errFileContextTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeFileContexts([]domain.FileContext{tc.item})
			if !errors.Is(err, tc.want) {
				t.Fatalf("normalize = %v, want %v", err, tc.want)
			}
		})
	}
}
