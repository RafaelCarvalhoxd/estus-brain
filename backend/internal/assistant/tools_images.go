package assistant

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

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
}

// generateImage tries the OpenAI key configured in Settings first — the
// general-purpose, photorealistic option — falling back to the Mac's own
// Image Playground only when no key is set. It doesn't ask which chat engine
// is talking: Apple's own tool-calling loop runs the tool through the same
// HTTP endpoint every other caller uses (see /api/assistant/tools), with no
// way to tell them apart, so the tool picks the best backend it has instead
// of the one "asking".
func generateImage(ctx context.Context, d Deps, prompt string) (data []byte, contentType string, err error) {
	if key := apiKeyFrom(ctx, d.AssistantRepo, d.VaultKey, "openai", os.Getenv("OPENAI_API_KEY")); key != "" {
		return generateImageOpenAI(ctx, key, prompt)
	}
	if d.AppleBridge != nil {
		return generateImageApple(ctx, d.AppleBridge, prompt)
	}
	return nil, "", invalid("nenhum motor com geração de imagem configurado — configure a API key da OpenAI em Configurações, ou use um Mac com Apple Intelligence")
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

// generateImageApple asks the Swift helper's /generate-image route, which
// starts the bridge on demand the same way a chat message would (see
// AppleBridge.ensure). Image Playground only draws in a few illustrated
// styles, never a photo — see apple-bridge's own doc comment on the route.
func generateImageApple(ctx context.Context, bridge *AppleBridge, prompt string) ([]byte, string, error) {
	h, err := bridge.ensure(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("ponte do Apple Intelligence: %w", err)
	}
	if !h.Available {
		return nil, "", fmt.Errorf("Apple Intelligence indisponível: %s", h.Reason)
	}
	var res struct {
		ImageBase64 string `json:"image_base64"`
		Error       string `json:"error"`
	}
	if err := postJSON(ctx, bridge.url+"/generate-image", nil, map[string]any{"prompt": prompt}, &res); err != nil {
		return nil, "", fmt.Errorf("gerar imagem no Apple Intelligence: %w", err)
	}
	if res.Error != "" {
		return nil, "", fmt.Errorf("gerar imagem no Apple Intelligence: %s", res.Error)
	}
	data, err := base64.StdEncoding.DecodeString(res.ImageBase64)
	if err != nil {
		return nil, "", fmt.Errorf("decodificar imagem do Apple Intelligence: %w", err)
	}
	return data, "image/png", nil
}
