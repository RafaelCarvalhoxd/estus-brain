package assistant

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// drawThingsURL is Draw Things' own local server (Advanced tab → API
// Server → Protocol HTTP, port 7860) — a free, offline image generator for
// whoever doesn't have an OpenAI key. DRAWTHINGS_URL overrides it, the same
// way OLLAMA_URL does for Ollama.
func drawThingsURL() string {
	if u := os.Getenv("DRAWTHINGS_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://127.0.0.1:7860"
}

// addImages registers generate_image when there's somewhere to keep the
// result: it is saved as an ordinary Document (the same storage the
// Documentos screen uses), so the picture survives a restart and shows up
// there too, not just in the conversation that made it.
func (r *Registry) addImages() {
	d := r.deps
	if d.Documents == nil || d.AssistantRepo == nil {
		return
	}
	r.add(Tool{
		Name: "generate_image", Title: "Gerar imagem", Module: "geral",
		Description: "Gera uma imagem a partir de uma descrição e mostra na conversa. Use quando a pessoa pedir um desenho, ilustração ou imagem.",
		Input:       object(map[string]any{"prompt": str("O que desenhar, em detalhes")}, "prompt"),
		run: typed(func(ctx context.Context, in struct {
			Prompt string `json:"prompt"`
		}) (any, error) {
			prompt := strings.TrimSpace(in.Prompt)
			if prompt == "" {
				return nil, invalid("descreva o que gerar")
			}
			data, contentType, err := generateImage(ctx, d, prompt)
			if err != nil {
				return nil, err
			}
			name := fmt.Sprintf("Imagem gerada %s.png", time.Now().Format("2006-01-02 15-04-05"))
			doc, err := d.Documents.Save(ctx, nil, name, contentType, bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("salvar imagem gerada: %w", err)
			}
			return map[string]any{"document_id": doc.ID, "descricao": prompt}, nil
		}),
	})

	r.add(Tool{
		Name: "edit_image", Title: "Editar imagem", Module: "geral",
		Description: "Edita uma imagem já anexada na conversa ou salva em Documentos, a partir de um pedido em texto (ex.: remover o fundo, mudar a cor, adicionar algo). Usa a API key da OpenAI se houver, senão o Draw Things local.",
		Input: object(map[string]any{
			"document_id": str("Id da imagem a editar"),
			"prompt":      str("O que mudar na imagem"),
		}, "document_id", "prompt"),
		run: typed(func(ctx context.Context, in struct {
			DocumentID string `json:"document_id"`
			Prompt     string `json:"prompt"`
		}) (any, error) {
			prompt := strings.TrimSpace(in.Prompt)
			if prompt == "" {
				return nil, invalid("descreva o que mudar")
			}
			doc, file, err := d.Documents.Open(ctx, in.DocumentID)
			if err != nil {
				return nil, err
			}
			imgData, err := io.ReadAll(io.LimitReader(file, maxAttachmentBytes))
			file.Close()
			if err != nil {
				return nil, fmt.Errorf("ler imagem: %w", err)
			}
			out, contentType, err := editImage(ctx, d, imgData, doc.Name, prompt)
			if err != nil {
				return nil, err
			}
			name := fmt.Sprintf("Imagem editada %s.png", time.Now().Format("2006-01-02 15-04-05"))
			newDoc, err := d.Documents.Save(ctx, nil, name, contentType, bytes.NewReader(out))
			if err != nil {
				return nil, fmt.Errorf("salvar imagem editada: %w", err)
			}
			return map[string]any{"document_id": newDoc.ID, "descricao": prompt}, nil
		}),
	})
}

// generateImage tries OpenAI first when a key is configured (paid, quick,
// general-purpose), then Draw Things running locally (free, offline —
// confirmed against its real Automatic1111-compatible API, not assumed).
// Neither cares which chat engine called the tool: Apple's on-device Image
// Playground did (a real foreground app, one at a time) and was pulled back
// out for it — this one works the same from any engine, including Ollama
// and Apple, because it never touches the Mac's own UI at all.
func generateImage(ctx context.Context, d Deps, prompt string) (data []byte, contentType string, err error) {
	if key := apiKeyFrom(ctx, d.AssistantRepo, d.VaultKey, "openai", os.Getenv("OPENAI_API_KEY")); key != "" {
		return generateImageOpenAI(ctx, key, prompt)
	}
	if drawThingsReachable(ctx) {
		return generateImageDrawThings(ctx, prompt)
	}
	return nil, "", invalid("nenhum motor de imagem disponível — configure a API key da OpenAI em Configurações, ou abra o Draw Things com o servidor de API ligado (Avançado → API Server)")
}

// drawThingsReachable is a quick, short-timeout probe: Draw Things not being
// open is the common case, and generateImage must fall through to the clear
// "nothing configured" message instead of a raw connection-refused error.
func drawThingsReachable(ctx context.Context) bool {
	reqCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, drawThingsURL()+"/sdapi/v1/options", nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode < 500
}

// generateImageDrawThings hits the same /sdapi/v1/txt2img shape
// Automatic1111 popularized and Draw Things implements for compatibility —
// confirmed live against a running Draw Things instance, not from docs
// alone. It runs whatever model is currently selected in the app (FLUX,
// Stable Diffusion, …), so there is no model name to pass here.
func generateImageDrawThings(ctx context.Context, prompt string) ([]byte, string, error) {
	var res struct {
		Images []string `json:"images"`
	}
	err := postJSON(ctx, drawThingsURL()+"/sdapi/v1/txt2img", nil,
		map[string]any{"prompt": prompt, "width": 1024, "height": 1024},
		&res)
	if err != nil {
		return nil, "", fmt.Errorf("gerar imagem no Draw Things: %w", err)
	}
	if len(res.Images) == 0 {
		return nil, "", fmt.Errorf("gerar imagem no Draw Things: resposta sem imagem")
	}
	data, err := base64.StdEncoding.DecodeString(res.Images[0])
	if err != nil {
		return nil, "", fmt.Errorf("decodificar imagem do Draw Things: %w", err)
	}
	return data, "image/png", nil
}

func generateImageOpenAI(ctx context.Context, key, prompt string) ([]byte, string, error) {
	var res struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	err := postJSON(ctx, "https://api.openai.com/v1/images/generations",
		map[string]string{"Authorization": "Bearer " + key},
		map[string]any{"model": "gpt-image-1", "prompt": prompt, "size": "1024x1024"},
		&res)
	if err != nil {
		return nil, "", fmt.Errorf("gerar imagem na OpenAI: %w", err)
	}
	if len(res.Data) == 0 || res.Data[0].B64JSON == "" {
		return nil, "", fmt.Errorf("gerar imagem na OpenAI: resposta sem imagem")
	}
	data, err := base64.StdEncoding.DecodeString(res.Data[0].B64JSON)
	if err != nil {
		return nil, "", fmt.Errorf("decodificar imagem da OpenAI: %w", err)
	}
	return data, "image/png", nil
}

// editImage mirrors generateImage's fallback: OpenAI when a key is
// configured, else Draw Things running locally.
func editImage(ctx context.Context, d Deps, imageData []byte, imageName, prompt string) ([]byte, string, error) {
	if key := apiKeyFrom(ctx, d.AssistantRepo, d.VaultKey, "openai", os.Getenv("OPENAI_API_KEY")); key != "" {
		return editImageOpenAI(ctx, key, imageData, imageName, prompt)
	}
	if drawThingsReachable(ctx) {
		return editImageDrawThings(ctx, imageData, prompt)
	}
	return nil, "", invalid("nenhum motor de imagem disponível — configure a API key da OpenAI em Configurações, ou abra o Draw Things com o servidor de API ligado (Avançado → API Server)")
}

// editImageDrawThings hits /sdapi/v1/img2img, the same Automatic1111-shaped
// endpoint Draw Things implements for txt2img's edit — confirmed live.
// denoising_strength is deliberately mid-range: low barely changes the
// image, high ignores it and just generates fresh from the prompt.
func editImageDrawThings(ctx context.Context, imageData []byte, prompt string) ([]byte, string, error) {
	var res struct {
		Images []string `json:"images"`
	}
	err := postJSON(ctx, drawThingsURL()+"/sdapi/v1/img2img", nil,
		map[string]any{
			"prompt":             prompt,
			"init_images":        []string{base64.StdEncoding.EncodeToString(imageData)},
			"denoising_strength": 0.6,
		},
		&res)
	if err != nil {
		return nil, "", fmt.Errorf("editar imagem no Draw Things: %w", err)
	}
	if len(res.Images) == 0 {
		return nil, "", fmt.Errorf("editar imagem no Draw Things: resposta sem imagem")
	}
	data, err := base64.StdEncoding.DecodeString(res.Images[0])
	if err != nil {
		return nil, "", fmt.Errorf("decodificar imagem editada do Draw Things: %w", err)
	}
	return data, "image/png", nil
}

// editImageOpenAI hits OpenAI's separate images/edits endpoint — distinct
// from images/generations, and multipart (it takes the source image as a
// file part) rather than JSON, so it doesn't go through postJSON.
func editImageOpenAI(ctx context.Context, key string, imageData []byte, imageName, prompt string) ([]byte, string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("model", "gpt-image-1"); err != nil {
		return nil, "", err
	}
	if err := w.WriteField("prompt", prompt); err != nil {
		return nil, "", err
	}
	part, err := w.CreateFormFile("image", imageName)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(imageData); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/images/edits", &body)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("editar imagem na OpenAI: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, "", err
	}
	if res.StatusCode >= 300 {
		var e struct {
			Error json.RawMessage `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := string(e.Error)
		if msg == "" {
			msg = truncate(string(raw), 300)
		}
		return nil, "", &apiError{Host: "api.openai.com", Status: res.StatusCode, Message: msg}
	}
	var out struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, "", err
	}
	if len(out.Data) == 0 || out.Data[0].B64JSON == "" {
		return nil, "", fmt.Errorf("editar imagem na OpenAI: resposta sem imagem")
	}
	data, err := base64.StdEncoding.DecodeString(out.Data[0].B64JSON)
	if err != nil {
		return nil, "", fmt.Errorf("decodificar imagem editada: %w", err)
	}
	return data, "image/png", nil
}
