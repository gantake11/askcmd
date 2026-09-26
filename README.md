# askcmd - CLIコマンド学習支援ツール

日本語の自然言語で「やりたいこと」を入力すると、最適なCLIコマンドとそのオプション解説をリアルタイムで提示してくれる学習支援CLIツールです。

Ubuntu環境で稼働するローカルLLM（[Ollama](https://ollama.com/)）をバックエンドとして活用します（WSLからWindows側のOllamaへの接続も可能）。

---

## 🌟 特徴

- **2段階自動判定アーキテクチャ**:
  1. 入力された要望から最適なツール（`rg`, `fd`, `fzf`, `bat`, `jq`, `gh`, `eza` などのモダンツール、または標準Linuxコマンド）を判定。
  2. 環境に対象ツールがインストールされていれば、ローカルの `--help` 情報（先頭120行）を取得してLLMに注入。バージョンやオプション差分に対応した正確なコマンドを生成します。
- **リアルタイム・ストリーミング出力**: Ollamaの生成結果を待たずに標準出力へ順次表示。
- **軽量・外部依存ゼロ**: Go言語の標準ライブラリのみで実装。デフォルトでローカル（`http://localhost:11434`）に接続するため設定不要で即座に使えます。

---

## 📋 動作要件

| 環境 | 要件 |
| :--- | :--- |
| **Ubuntu / Linux (WSL含む)** | Go 1.22 以上、`~/.local/bin` へのPATH設定 |
| **Ollama** | ローカル稼働中（デフォルト: `http://localhost:11434`） |
| **推奨モデル** | `qwen2.5-coder:7b`(デフォルト)もしくは`qwen2.5-coder:3b` (変更可能) |

---

## 🚀 セットアップ手順

### Step 1: Go言語のインストール

Goが未導入の場合は、以下のいずれかの方法でインストールします（Go 1.22 以上が必要です）。

#### 方法A: aptパッケージマネージャーを使用（簡単・推奨）
Ubuntu 24.04等のモダンな環境では標準リポジトリからGo 1.22以上がインストール可能です。
```bash
sudo apt update
sudo apt install -y golang-go
```

#### 方法B: 公式バイナリを使用（最新版を導入したい場合）
公式アーカイブを取得して `/usr/local` に展開します。
```bash
# バイナリのダウンロードと展開
wget https://go.dev/dl/go1.22.2.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.2.linux-amd64.tar.gz

# PATHの設定（~/.bashrc に追加）
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

**バージョンの確認**:
```bash
go version
# 出力例: go version go1.22.2 linux/amd64
```

---

### Step 2: Ubuntu側へのOllama導入（推奨）

UbuntuローカルにOllamaを導入している場合は、追加のネットワーク設定は不要です。

1. **Ollamaのインストール**:
   ```bash
   curl -fsSL https://ollama.com/install.sh | sh
   ```
2. **サービスの起動確認**:
   ```bash
   systemctl status ollama
   ```
   ※ active (running) となっていれば正常に起動しています。手動起動の場合は `ollama serve` を実行します。
3. **推奨モデルの取得**:
   ```bash
   ollama pull qwen2.5-coder:7b
   ```
4. **モデル一覧の確認**:
   ```bash
   ollama list
   ```

> [!NOTE]
> **Windows側のOllamaを利用したい場合（任意）**:
> Windows側でOllamaを動かし、WSLから接続したい場合は以下のように設定します:
> 1. Windows側で環境変数 `OLLAMA_HOST=0.0.0.0:11434` を設定してOllamaを再起動。
> 2. WSL側の `~/.bashrc` に以下を追加:
>    ```bash
>    export OLLAMA_HOST="http://$(ip route show default | awk '{print $3}'):11434"
>    ```
> ※ UbuntuローカルのOllamaを利用する場合は、上記のような `OLLAMA_HOST` の設定は不要（または未設定）です。

---

### Step 3: `askcmd` のインストール

#### 方法A: GitHub Releases からバイナリをダウンロード（推奨）
[GitHub Releases](https://github.com/gantake11/askcmd/releases) からお使いのOS・アーキテクチャ（Linux / macOS / Windows、amd64 / arm64）に応じたアーカイブをダウンロードして展開し、実行可能パス（例: `~/.local/bin`）に配置します。

```bash
# 例: Linux (x86_64 / amd64) の場合
curl -sL https://github.com/gantake11/askcmd/releases/latest/download/askcmd_Linux_x86_64.tar.gz | tar -xz askcmd
mv askcmd ~/.local/bin/
```

#### 方法B: ソースコードからビルド
```bash
go build -o ~/.local/bin/askcmd main.go
```

**PATHの確認**:
`~/.local/bin` にPATHが通っていることを確認します。
```bash
which askcmd
# 出力例: /home/<user>/.local/bin/askcmd
askcmd --version
```
※ PATHが通っていない場合は、`export PATH="$HOME/.local/bin:$PATH"` を `~/.bashrc` に追加して `source ~/.bashrc` を実行してください。

---

## 📖 使い方

### 基本構文
```bash
askcmd "<やりたいこと・要望>"
```

### 実行例

#### 1. ファイル内容の検索
```bash
askcmd "特定の拡張子を除外して中身を検索したい"
```
**出力例:**
```markdown
### コマンド例
```bash
grep -r --exclude="*.ext" "検索する文字列" /path/to/search
```

### オプション・引数の解説
- `-r`: ディレクトリツリーを再帰的に検索します。
- `--exclude="*.ext"`: 指定した拡張子のファイルを除外します。
- `"検索する文字列"`: 検索したい文字列を指定します。
- `/path/to/search`: 検索対象のディレクトリパスを指定します。

### ポイント
- 大規模なプロジェクトでは `--exclude` で不要なファイル（ビルド成果物やログ等）を省くと高速です。
```

#### 2. ファイル一覧の表示
```bash
askcmd "ファイル一覧を詳細情報付きで色付けして見たい"
```

#### 3. Git操作
```bash
askcmd "直近のコミット履歴をグラフ付きで1行ずつ見たい"
```

#### 4. JSONデータの加工
```bash
askcmd "JSONの特定キーの値を抽出してユニーク一覧にしたい"
```

---

## ⚙️ 環境変数によるカスタマイズ

| 環境変数 | 説明 | デフォルト値 |
| :--- | :--- | :--- |
| `OLLAMA_HOST` | OllamaのエンドポイントURL | `http://localhost:11434` |
| `ASKCMD_MODEL` | 使用するLLMモデル名 | `qwen2.5-coder:7b` |

#### モデルの変更例:
```bash
export ASKCMD_MODEL="llama3.1:8b"
askcmd "ポート8080を使っているプロセスを調べたい"
```

---

## 🛠️ トラブルシューティング

### Q. 「Ollamaへの接続・生成に失敗しました」と表示される

1. **Ubuntu側のOllamaサービスが起動しているか確認**:
   ```bash
   systemctl status ollama
   ```
   停止している場合は起動します:
   ```bash
   sudo systemctl start ollama
   ```
2. **モデルがダウンロードされているか確認**:
   ```bash
   ollama list
   ```
   一覧に `qwen2.5-coder:7b` がない場合は pull してください:
   ```bash
   ollama pull qwen2.5-coder:7b
   ```
3. **接続先確認**:
   環境変数 `OLLAMA_HOST` が意図せず別のIPに設定されていないか確認してください:
   ```bash
   echo $OLLAMA_HOST
   ```
   UbuntuローカルのOllamaを使う場合、未設定であれば自動的に `http://localhost:11434` へ接続されます。過去に設定した不要な記述が `~/.bashrc` 等にあれば削除してください。

---

## 📁 ファイル構成

本リポジトリに含まれるファイルとその役割です。Go標準ライブラリのみで実装されており、外部依存はありません。

| ファイル | 概要 | 説明 |
| :--- | :--- | :--- |
| `main.go` | アプリケーション本体 | CLI引数の解析、Ollama APIとの通信、最適ツール判定（Step 1）、ローカルの `--help` 情報取得（Step 2）、ストリーミング生成出力（Step 3）など、ツールの全メインロジックが実装されています。 |
| `main_test.go` | 単体テスト | `httptest` を利用したモックサーバーによるツール判定・ストリーミング処理のテストや、環境変数 `OLLAMA_HOST` のURL正規化ロジックに対する単体テストです。 |
| `go.mod` | モジュール定義 | モジュール名 `askcmd` とGoバージョン（`go 1.22.2`）を定義しています。外部パッケージへの依存はありません。 |
| `.goreleaser.yaml` | リリース設定 | GoReleaser v2 のクロスコンパイル（Linux/macOS/Windows）、アーカイブ圧縮、チェックサム生成の設定です。 |
| `.github/workflows/release.yml` | CI/CDワークフロー | `v*` タグのプッシュをトリガーに全OS向けバイナリを自動ビルド・リリース公開するGitHub Actions定義です。 |
| `README.md` | プロジェクトドキュメント | 本ツールの特徴、セットアップ手順、使い方、環境変数設定、トラブルシューティング、ファイル構成などをまとめたドキュメントです。 |

---

## 🧪 テストの実行

```bash
go test -v ./...
```
モックサーバーを用いた単体テストが実行されます。

