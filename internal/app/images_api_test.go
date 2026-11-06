package app

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImageAPIInputAndOutput(t *testing.T) {
	refs, err := imageReferences(map[string]interface{}{
		"images": []interface{}{
			map[string]interface{}{"image_url": map[string]interface{}{"url": "https://example.com/a.png"}},
			"aGVsbG8=",
		},
	})
	if err != nil || len(refs) != 2 || refs[0] != "https://example.com/a.png" || refs[1] != "data:image/png;base64,aGVsbG8=" {
		t.Fatalf("image references: %v, %v", refs, err)
	}
	if got := imagePrompt("draw a cat", map[string]interface{}{"size": "768x1376"}); !strings.Contains(got, "portrait orientation") {
		t.Fatalf("portrait size lost: %q", got)
	}
	if got := imagePrompt("draw a cat", map[string]interface{}{"aspect_ratio": "9:16"}); !strings.Contains(got, "9:16") {
		t.Fatalf("aspect ratio lost: %q", got)
	}
	want := base64.StdEncoding.EncodeToString([]byte("png bytes"))
	got, err := extractGeneratedImage("Done. ![image](data:image/png;base64," + want + ")")
	if err != nil || got != want {
		t.Fatalf("image extraction: %q, %v", got, err)
	}
	if _, err := extractGeneratedImage("No image generated"); err == nil {
		t.Fatal("missing image was accepted")
	}
}

func TestImageAPIRejectsUnsupportedRequests(t *testing.T) {
	for _, tc := range []struct {
		path, body, want string
	}{
		{"/v1/images/generations", `{"model":"gemini-image"}`, "prompt"},
		{"/v1/images/edits", `{"model":"gemini-image","prompt":"edit"}`, "image"},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		if tc.path == "/v1/images/edits" {
			handleImageEdit(w, req)
		} else {
			handleImageGeneration(w, req)
		}
		if w.Code != 400 || !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("%s: status=%d body=%s", tc.path, w.Code, w.Body.String())
		}
	}
}
