package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func printOllamaError(baseURL, model string, err error) {
	fmt.Fprintf(os.Stderr, "エラー: Ollama (%s) への接続・生成に失敗しました。\n", baseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "詳細: %v\n\n", err)
	}
	fmt.Fprintln(os.Stderr, "【確認事項】")
	fmt.Fprintln(os.Stderr, "1. Ubuntu側でOllamaが起動しているか確認してください:")
	fmt.Fprintln(os.Stderr, "   systemctl status ollama  (手動起動の場合は 'ollama serve')")
	fmt.Fprintf(os.Stderr, "2. モデル '%s' が準備されているか確認してください:\n", model)
	fmt.Fprintf(os.Stderr, "   ollama list\n")
	fmt.Fprintf(os.Stderr, "   ollama pull %s\n", model)
	fmt.Fprintln(os.Stderr, "3. 接続先を変更したい場合は環境変数 OLLAMA_HOST を設定してください (デフォルト: http://localhost:11434):")
	fmt.Fprintln(os.Stderr, "   export OLLAMA_HOST=\"http://localhost:11434\"")
}

var modernTools = map[string]string{
	"rg":  "ripgrep (高速文字列検索)",
	"fd":  "fd-find (高速ファイル検索)",
	"fzf": "対話的絞り込み・ファジーファインダー",
	"bat": "シンタックスハイライト付きcat代替",
	"jq":  "JSONデータのパース・加工",
	"gh":  "GitHub CLI (PR/Issue操作)",
	"eza": "高機能ls代替",
	"glow": "マークダウンレンダリング",
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func getOllamaBaseURL() string {
	host := strings.TrimSpace(os.Getenv("OLLAMA_HOST"))
	if host == "" {
		return "http://localhost:11434"
	}
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	return strings.TrimRight(host, "/")
}

func getModel() string {
	model := os.Getenv("ASKCMD_MODEL")
	if model == "" {
		return "qwen2.5-coder:7b"
	}
	return model
}

func printUsage() {
	fmt.Println("Usage: askcmd <要望>")
	fmt.Println()
	fmt.Println("自然言語で実行したい処理を渡すと、最適なCLIコマンドとオプション解説を提示します。")
	fmt.Println()
	fmt.Println("例:")
	fmt.Println("  askcmd \"特定の拡張子を除外して中身を検索したい\"")
	fmt.Println("  askcmd \"直近のコミット履歴をグラフ付きで見たい\"")
}

func step1DetermineTool(client *http.Client, baseURL, model, query string) (string, error) {
	prompt := fmt.Sprintf(`あなたはCLIツールの専門家です。
以下のユーザーの要望に最も適したツール名を、下記のモダンツール候補リストから1単語のみで答えてください。
候補リストに適したツールがない場合や、標準的なLinuxコマンド（git, curl, tar, find, grep, ssh等）で十分な場合は「standard」と答えてください。
解説や余計な文字（マークダウンやクォート等）は一切含めず、ツール名のみを1行で出力してください。

【モダンツール候補リスト】
- rg: ripgrep (高速文字列検索)
- fd: fd-find (高速ファイル検索)
- fzf: 対話的絞り込み・ファジーファインダー
- bat: シンタックスハイライト付きcat代替
- jq: JSONデータのパース・加工
- gh: GitHub CLI (PR/Issue操作)
- eza: 高機能ls代替
- glow: マークダウンレンダリング

【ユーザーの要望】
%s

【ツール名】`, query)

	reqBody, err := json.Marshal(GenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "standard", err
	}

	req, err := http.NewRequest("POST", baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return "standard", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := strings.TrimSpace(string(body))
		if errMsg != "" {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, errMsg)
		}
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var genResp GenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "standard", err
	}

	rawTool := strings.TrimSpace(genResp.Response)
	rawTool = strings.ToLower(rawTool)
	rawTool = strings.Trim(rawTool, "`'\" \n\r\t")
	fields := strings.Fields(rawTool)
	if len(fields) > 0 {
		rawTool = fields[0]
	}

	if _, ok := modernTools[rawTool]; ok {
		return rawTool, nil
	}

	return "standard", nil
}

func step2GetHelpContext(toolName string) string {
	if toolName == "standard" {
		return ""
	}
	if _, ok := modernTools[toolName]; !ok {
		return ""
	}

	binPath, err := exec.LookPath(toolName)
	if err != nil && toolName == "fd" {
		binPath, err = exec.LookPath("fdfind")
	}
	if err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "--help")
	out, err := cmd.Output()
	if err != nil {
		if len(out) == 0 {
			return ""
		}
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	var lines []string
	lineCount := 0
	for scanner.Scan() && lineCount < 120 {
		lines = append(lines, scanner.Text())
		lineCount++
	}

	return strings.Join(lines, "\n")
}

func step3StreamCommand(baseURL, model, query, toolName, helpContext string) error {
	var promptBuilder strings.Builder
	promptBuilder.WriteString("あなたは親切で有能なCLIコマンド学習アシスタントです。\n")
	promptBuilder.WriteString("ユーザーの要望に対して、最適なCLIコマンドとそのオプション解説を提示してください。\n\n")

	if helpContext != "" {
		promptBuilder.WriteString(fmt.Sprintf("【参考: %s のヘルプ情報（抜粋）】\n```\n%s\n```\n\n", toolName, helpContext))
		promptBuilder.WriteString(fmt.Sprintf("上記のヘルプ情報を参考に、%s コマンドを活用した最適なコマンドラインを提示してください。\n\n", toolName))
	} else if toolName != "standard" {
		promptBuilder.WriteString(fmt.Sprintf("ユーザーの要望には「%s」コマンドが適しています。\n\n", toolName))
	}

	promptBuilder.WriteString("【ユーザーの要望】\n")
	promptBuilder.WriteString(query + "\n\n")

	promptBuilder.WriteString("【出力フォーマット】\n")
	promptBuilder.WriteString("以下の形式に従って、Markdown形式で分かりやすく日本語で出力してください。\n\n")
	promptBuilder.WriteString("### コマンド例\n")
	promptBuilder.WriteString("```bash\n")
	promptBuilder.WriteString("<実行可能なコマンド>\n")
	promptBuilder.WriteString("```\n\n")
	promptBuilder.WriteString("### オプション・引数の解説\n")
	promptBuilder.WriteString("- `<オプションや引数>`: <分かりやすい解説>\n\n")
	promptBuilder.WriteString("### ポイント\n")
	promptBuilder.WriteString("<コマンドの動作説明や実用的な注意点、代替案など>\n")

	reqBody, err := json.Marshal(GenerateRequest{
		Model:  model,
		Prompt: promptBuilder.String(),
		Stream: true,
	})
	if err != nil {
		return err
	}

	streamClient := &http.Client{}

	req, err := http.NewRequest("POST", baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := strings.TrimSpace(string(body))
		if errMsg != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, errMsg)
		}
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var genResp GenerateResponse
			if jsonErr := json.Unmarshal(line, &genResp); jsonErr == nil {
				os.Stdout.WriteString(genResp.Response)
				os.Stdout.Sync()
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	os.Stdout.WriteString("\n")
	os.Stdout.Sync()

	return nil
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	query := strings.Join(os.Args[1:], " ")
	baseURL := getOllamaBaseURL()
	model := getModel()

	// Step 1: ツールの動的判定
	step1Client := &http.Client{
		Timeout: 60 * time.Second,
	}

	toolName, err := step1DetermineTool(step1Client, baseURL, model, query)
	if err != nil {
		printOllamaError(baseURL, model, err)
		os.Exit(1)
	}

	// Step 2: ヘルプ情報のコンテキスト注入
	helpContext := step2GetHelpContext(toolName)

	// Step 3: コマンドと解説のストリーミング生成
	if err := step3StreamCommand(baseURL, model, query, toolName, helpContext); err != nil {
		printOllamaError(baseURL, model, err)
		os.Exit(1)
	}
}
