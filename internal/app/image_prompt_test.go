package app

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPromptImageWireRequest(t *testing.T) {
	if !Models["gemini-image"].PromptImage || Models["gemini-image"].HexID != hexPro31 {
		t.Fatal("image generation must use the Pro prompt route")
	}
	raw, err := promptImagePayload("Generate an image of a cat")
	if err != nil {
		t.Fatal(err)
	}
	var outer []interface{}
	if err := json.Unmarshal(raw, &outer); err != nil || len(outer) != 2 {
		t.Fatalf("outer request: %v, %v", outer, err)
	}
	var inner []interface{}
	if err := json.Unmarshal([]byte(outer[1].(string)), &inner); err != nil {
		t.Fatal(err)
	}
	if len(inner) != 4 || inner[3] != "gemini-3-pro" || !reflect.DeepEqual(inner[0], []interface{}{"Generate an image of a cat"}) {
		t.Fatalf("unexpected image prompt request: %v", inner)
	}
	var header []interface{}
	if err := json.Unmarshal([]byte(promptImageModelHeader(hexPro31)), &header); err != nil || len(header) != 12 || header[4] != hexPro31 {
		t.Fatalf("unexpected model header: %v, %v", header, err)
	}
}

func TestCollectCandidateImageURLs(t *testing.T) {
	image := func(link string) interface{} {
		return []interface{}{[]interface{}{nil, nil, nil, []interface{}{nil, 1, "a.jpg", link}}}
	}
	block := make([]interface{}, 8)
	block[7] = []interface{}{[]interface{}{
		image("https://lh3.googleusercontent.com/gg-dl/AAA"),
		image("https://example.com/not-an-artifact"),
	}}
	candidate := make([]interface{}, 13)
	candidate[12] = block
	body := make([]interface{}, 5)
	body[4] = []interface{}{candidate}
	encoded, _ := json.Marshal(body)
	frame, _ := json.Marshal([]interface{}{[]interface{}{"wrb.fr", "x", string(encoded)}})
	got := collectCandidateImageURLs(string(frame) + "\n" + string(frame))
	want := []string{"https://lh3.googleusercontent.com/gg-dl/AAA"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate image URLs: got %v, want %v", got, want)
	}
}
