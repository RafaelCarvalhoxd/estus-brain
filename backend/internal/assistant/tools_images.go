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

// addImages registers generate_image when there's somewhere to keep the
// result: it is saved as an ordinary Document (the same storage the
// Documentos screen uses), so the picture survives a restart and shows up
// there too, not just in the conversation that made it.
func (r *Registry) addImages() {
	d := r.deps
	if d.Documents == nil {
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
		Description: "Edita uma imagem já anexada na conversa ou salva em Documentos, a partir de um pedido em texto (ex.: remover o fundo, mudar a cor, adicionar algo). Requer OPENAI_API_KEY no servidor.",
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

// generateImage requires an OpenAI key. Draw Things (a free local
// alternative) was tried and pulled back out: on this machine it locked up
// the whole Mac during generation, an unresolved resource problem, not
// something to keep as a silent fallback.
func generateImage(ctx context.Context, d Deps, prompt string) (data []byte, contentType string, err error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, "", invalid("nenhum motor de imagem disponível — defina OPENAI_API_KEY no servidor")
	}
	return generateImageOpenAI(ctx, key, prompt)
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

// editImage requires an OpenAI key — see generateImage.
func editImage(ctx context.Context, d Deps, imageData []byte, imageName, prompt string) ([]byte, string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, "", invalid("nenhum motor de imagem disponível — defina OPENAI_API_KEY no servidor")
	}
	return editImageOpenAI(ctx, key, imageData, imageName, prompt)
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
