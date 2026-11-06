package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 9router's OpenAI-compatible image adapter sends JSON to these two endpoints.
// Gemini's image tool returns a Markdown data URL, so translate it into the
// OpenAI images response without storing the generated image on this server.
var generatedImageURL = regexp.MustCompile(`data:image/(png|jpeg|jpg|webp);base64,([A-Za-z0-9+/=]+)`)

func handleImageGeneration(w http.ResponseWriter, r *http.Request) { handleImageRequest(w, r, false) }
func handleImageEdit(w http.ResponseWriter, r *http.Request)       { handleImageRequest(w, r, true) }

func imageAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{"error": map[string]string{
		"message": message, "type": "invalid_request_error",
	}})
}

func imageReferences(req map[string]interface{}) ([]string, error) {
	var raw []interface{}
	if v, ok := req["images"].([]interface{}); ok {
		raw = append(raw, v...)
	}
	if v := req["image"]; v != nil {
		raw = append(raw, v)
	}
	refs := make([]string, 0, len(raw))
	for _, v := range raw {
		src := ""
		switch x := v.(type) {
		case string:
			src = x
		case map[string]interface{}:
			src = firstNonEmpty(getStr(x, "url"), getStr(x, "image_url"), getStr(x, "b64_json"))
			if nested, ok := x["image_url"].(map[string]interface{}); ok {
				src = firstNonEmpty(getStr(nested, "url"), src)
			}
		}
		src = strings.TrimSpace(src)
		if src == "" {
			return nil, fmt.Errorf("image reference must be a URL or base64 data URI")
		}
		// OpenAI-style b64_json is an unprefixed PNG payload.
		if !strings.HasPrefix(src, "data:") && !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
			src = "data:image/png;base64," + src
		}
		refs = append(refs, src)
	}
	return refs, nil
}

func imagePrompt(prompt string, req map[string]interface{}) string {
	var hints []string
	if ratio := getStr(req, "aspect_ratio"); ratio != "" {
		hints = append(hints, "aspect ratio "+ratio)
	} else if size := getStr(req, "size"); size != "" {
		parts := strings.Split(size, "x")
		if len(parts) == 2 {
			width, e1 := strconv.Atoi(parts[0])
			height, e2 := strconv.Atoi(parts[1])
			if e1 == nil && e2 == nil && width > 0 && height > 0 {
				switch {
				case height > width:
					hints = append(hints, "portrait orientation")
				case width > height:
					hints = append(hints, "landscape orientation")
				default:
					hints = append(hints, "square composition")
				}
			}
		}
	}
	if len(hints) == 0 {
		return prompt
	}
	return prompt + "\n\nImage layout: " + strings.Join(hints, ", ") + "."
}

func extractGeneratedImage(text string) (string, error) {
	matches := generatedImageURL.FindStringSubmatch(text)
	if len(matches) != 3 {
		return "", fmt.Errorf("upstream returned no image")
	}
	if _, err := base64.StdEncoding.DecodeString(matches[2]); err != nil {
		return "", fmt.Errorf("upstream returned invalid image data: %w", err)
	}
	return matches[2], nil
}

func handleImageRequest(w http.ResponseWriter, r *http.Request, edit bool) {
	if r.Method == http.MethodOptions {
		handleOptions(w, r)
		return
	}
	if r.Method != http.MethodPost {
		imageAPIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(r.Body, 24*1024*1024)).Decode(&req); err != nil {
		imageAPIError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	prompt := strings.TrimSpace(getStr(req, "prompt"))
	if prompt == "" {
		imageAPIError(w, 400, "missing required field: prompt")
		return
	}
	model := getStr(req, "model")
	if model == "" {
		model = "gemini-image"
	}
	if model != "gemini-image" {
		imageAPIError(w, 400, "unsupported image model: "+model)
		return
	}
	if n, ok := req["n"].(float64); ok && n != 1 {
		imageAPIError(w, 400, "n must be 1: upstream returns one image")
		return
	}
	if req["mask"] != nil {
		imageAPIError(w, 400, "mask editing is not supported")
		return
	}
	if format := getStr(req, "response_format"); format != "" && format != "b64_json" && format != "binary" {
		imageAPIError(w, 400, "response_format must be b64_json or binary")
		return
	}
	refs, err := imageReferences(req)
	if err != nil {
		imageAPIError(w, 400, err.Error())
		return
	}
	if edit && len(refs) == 0 {
		imageAPIError(w, 400, "missing required field: image (or images[])")
		return
	}
	if !edit && len(refs) > 0 {
		imageAPIError(w, 400, "use /v1/images/edits when supplying an image")
		return
	}
	_, mc, err := resolveModel(model)
	if err != nil {
		imageAPIError(w, 400, err.Error())
		return
	}
	var images []pendingUpload
	for i, src := range refs {
		img, err := materializeImage(src, "", i+1)
		if err != nil {
			imageAPIError(w, 400, err.Error())
			return
		}
		images = append(images, img)
	}
	fullPrompt := imagePrompt(prompt, req)
	text, _, result, err := callGemini(fullPrompt, fullPrompt, mc, nil, images, nil, nil)
	endpoint := "images.generations"
	if edit {
		endpoint = "images.edits"
	}
	if err != nil {
		status := 502
		if _, ok := err.(*RateLimitError); ok {
			status = 429
		}
		recordRequest(endpoint, model, fullPrompt, "", result, status, err.Error(), false)
		imageAPIError(w, status, "upstream error: "+err.Error())
		return
	}
	encoded, err := extractGeneratedImage(text)
	if err != nil {
		recordRequest(endpoint, model, fullPrompt, "", result, 502, err.Error(), false)
		imageAPIError(w, 502, err.Error())
		return
	}
	recordRequest(endpoint, model, fullPrompt, text, result, 200, "", false)
	writeJSON(w, 200, map[string]interface{}{
		"created": time.Now().Unix(),
		"data":    []map[string]string{{"b64_json": encoded, "revised_prompt": prompt}},
		"usage":   usageOf(fullPrompt, text, result),
	})
}
