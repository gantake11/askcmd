package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestStep1AndStep3WithMockOllama(t *testing.T) {
	// モックサーバーの起動
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req GenerateRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !req.Stream {
			// Step 1: ツール判定のレスポンス (rg を返す)
			resp := GenerateResponse{
				Response: "rg",
				Done:     true,
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			// Step 3: ストリーミングレスポンス
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")

			chunks := []string{
				"### コマンド例\n```bash\nrg --type-not md 'keyword'\n```\n\n",
				"### オプション・引数の解説\n- `--type-not`: 指定した拡張子を除外\n",
				"### ポイント\nripgrepを使うと高速に検索できます。\n",
			}

			for _, chunk := range chunks {
				resp := GenerateResponse{
					Response: chunk,
					Done:     false,
				}
				json.NewEncoder(w).Encode(resp)
				flusher.Flush()
			}
			json.NewEncoder(w).Encode(GenerateResponse{Done: true})
			flusher.Flush()
		}
	}))
	defer server.Close()

	// Step 1 テスト
	client := &http.Client{}
	tool, err := step1DetermineTool(client, server.URL, "test-model", "特定の拡張子を除外して検索したい")
	if err != nil {
		t.Fatalf("step1DetermineTool failed: %v", err)
	}
	if tool != "rg" {
		t.Errorf("expected tool 'rg', got '%s'", tool)
	}

	// Step 2 テスト
	help := step2GetHelpContext("standard")
	if help != "" {
		t.Errorf("expected empty help for standard, got '%s'", help)
	}

	// Step 3 テスト
	err = step3StreamCommand(server.URL, "test-model", "特定の拡張子を除外して検索したい", "rg", "")
	if err != nil {
		t.Fatalf("step3StreamCommand failed: %v", err)
	}
}

func TestGetOllamaBaseURL(t *testing.T) {
	orig := os.Getenv("OLLAMA_HOST")
	defer os.Setenv("OLLAMA_HOST", orig)

	os.Setenv("OLLAMA_HOST", "")
	if u := getOllamaBaseURL(); u != "http://localhost:11434" {
		t.Errorf("expected default http://localhost:11434, got %s", u)
	}

	os.Setenv("OLLAMA_HOST", "192.168.1.100:11434")
	if u := getOllamaBaseURL(); u != "http://192.168.1.100:11434" {
		t.Errorf("expected http://192.168.1.100:11434, got %s", u)
	}

	os.Setenv("OLLAMA_HOST", "http://192.168.1.100:11434/")
	if u := getOllamaBaseURL(); u != "http://192.168.1.100:11434" {
		t.Errorf("expected http://192.168.1.100:11434, got %s", u)
	}

	os.Setenv("OLLAMA_HOST", ":11434")
	if u := getOllamaBaseURL(); u != "http://localhost:11434" {
		t.Errorf("expected http://localhost:11434, got %s", u)
	}
}
