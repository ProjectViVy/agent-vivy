package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"agent-vivy/internal/domain"
)

type imageReadOps struct {
	recordingFileOps
	result FileReadResult
}

func (o *imageReadOps) ReadFile(context.Context, domain.RunID, FileReadRequest) (FileReadResult, error) {
	return o.result, nil
}

func TestReadFileReturnsImagePartsEnvelope(t *testing.T) {
	raw := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 1, 2, 3, 4}
	ops := &imageReadOps{result: FileReadResult{Path: "shots/shot.png", Bytes: len(raw), ImageMIME: "image/png", ImageData: raw}}
	ctx := WithRunID(context.Background(), domain.RunID("run_image"))

	out, err := NewReadFile(ops).InvokableRun(ctx, json.RawMessage(`{"path":"shots/shot.png"}`))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Parts []struct {
			Type       string `json:"type"`
			Text       string `json:"text,omitempty"`
			Base64Data string `json:"base64data,omitempty"`
			MIMEType   string `json:"mime_type,omitempty"`
		} `json:"parts"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("envelope: %v (%s)", err, out)
	}
	if len(envelope.Parts) != 2 {
		t.Fatalf("parts = %d (%s)", len(envelope.Parts), out)
	}
	if envelope.Parts[0].Type != "text" || envelope.Parts[0].Text == "" {
		t.Fatalf("text part = %+v", envelope.Parts[0])
	}
	img := envelope.Parts[1]
	if img.Type != "image" || img.MIMEType != "image/png" {
		t.Fatalf("image part = %+v", img)
	}
	if got, err := base64.StdEncoding.DecodeString(img.Base64Data); err != nil || string(got) != string(raw) {
		t.Fatalf("base64 roundtrip = %v, %d bytes", err, len(got))
	}
}

func TestReadFileTextResultHasNoEnvelope(t *testing.T) {
	ops := &imageReadOps{result: FileReadResult{Path: "a.go", Content: "package main\n", TotalLines: 1, StartLine: 1, EndLine: 1, Bytes: 13}}
	ctx := WithRunID(context.Background(), domain.RunID("run_image"))
	out, err := NewReadFile(ops).InvokableRun(ctx, json.RawMessage(`{"path":"a.go"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, present := result["parts"]; present {
		t.Fatalf("text read must not produce parts: %s", out)
	}
	if result["content"] != "1\tpackage main\n" {
		t.Fatalf("content = %s", out)
	}
}
