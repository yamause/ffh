# 開発者ドキュメント

## アーキテクチャ概要

ffh は単一バイナリで、通常起動と fzf からのコールバック起動の両方を担います。

```
ffh（通常起動）
  │
  ├─ SSH config パース
  ├─ タブ状態を一時ファイルに保存
  └─ fzf を起動
         │
         ├─ preview コールバック: ffh --preview-host <name>
         ├─ Tab キー: ffh --tab-list <statefile> 1
         └─ Shift-Tab キー: ffh --tab-list <statefile> -1
                              ↓
                         ffh --tab-header <statefile>
```

fzf の `--preview` / `--bind` に自身のパスを埋め込み、自己呼び出しで各機能を実装しています。

## ファイル構成

```
ffh/
├── main.go          エントリポイント・fzf 起動・タブ状態管理
├── parser.go        SSH config パーサー
├── hosts.go         hosts ファイル読み込み
├── config.go        設定解決（設定ファイル・環境変数）
├── parser_test.go   パーサーのユニットテスト
├── hosts_test.go    hosts パーサーのユニットテスト
├── config_test.go   設定解決のユニットテスト
├── go.mod           モジュール定義（依存なし）
├── Makefile         ビルド・インストール
├── AGENTS.md        AI エージェント向けハーネス説明
└── README.md        ユーザー向けドキュメント
```

## データ型

### `Host`（parser.go）

SSH config の `Host` ブロック 1 件を表します。

| フィールド | 対応ディレクティブ | 備考 |
|---|---|---|
| `Name` | `Host` | ワイルドカードを含むものは除外 |
| `HostName` | `HostName` | |
| `User` | `User` | |
| `Port` | `Port` | 空文字 = 22 |
| `ProxyJump` | `ProxyJump` | |
| `IdentityFile` | `IdentityFile` | 最初の1件のみ。`~/` 形式で保存 |
| `Tag` | `Tag` | タブ絞り込みに使用 |
| `Description` | `# Description:` コメント | 詳細は後述 |
| `SourceFile` | — | 読み込み元ファイルの絶対パス |

### `tabState`（main.go）

タブ状態を一時ファイルに保存するための構造体です。

```
// ファイルフォーマット（テキスト、改行区切り）
0          ← 現在のインデックス
All        ← tags[0]（常に "All"）
dev        ← tags[1]
prod       ← tags[2]
```

## コマンドライン引数

| 引数 | 用途 | 呼び出し元 |
|---|---|---|
| `（なし）` | SSH モードで起動 | ユーザー |
| `--hosts [path]` | hosts ファイルモード（パス解決は `resolveHostsPath` 参照） | ユーザー |
| `--preview-host <name>` | プレビューペイン出力 | fzf preview |
| `--tab-list <statefile> <delta>` | タブ切り替え＋ホスト一覧出力 | fzf reload |
| `--tab-header <statefile>` | タブヘッダー出力 | fzf transform-header |
| `--ssh-config-view <hostname>` | `ssh -G` 全オプションをネスト fzf で表示 | fzf execute (Ctrl-G) |
| `--preview-option <option-line>` | SSH オプションの日本語説明を出力 | ネスト fzf preview |

## SSH config パーサー

### ファイル収集（`collectFiles`）

1. メイン config（`~/.ssh/config`）を読む
2. `Include` ディレクティブを検出し、グロブパターンを解決
   - `~/` → `$HOME` に展開（`filepath.Glob` は `~` を展開しないため）
   - 相対パス → config ファイルのディレクトリ基準で解決
3. **Include ファイルを先に、メイン config を後**に処理（OpenSSH の動作に準拠）
4. 同名ホストは最初の出現を採用（first-match-wins）

### ブロック抽出（`parseFile`）

状態機械で行単位に処理します。

```
状態変数:
  current         *Host    // 処理中の Host ブロック（nil = ブロック外）
  pendingComments []string // 直前のコメント行バッファ
  inMatch         bool     // Match ブロック内フラグ
```

| 行の種別 | 処理 |
|---|---|
| 空行 | `current` を確定。`pendingComments` をクリア |
| `#` コメント（ブロック外） | `pendingComments` に追加 |
| `#` コメント（ブロック内） | 無視 |
| `Host <pattern>`（ワイルドカードなし） | 新 Host 作成。`pendingComments` から Description を抽出 |
| `Host <pattern>`（`*` or `?` 含む） | スキップ |
| `Match` | `current` を確定。`inMatch = true` |
| ディレクティブ行 | `current` に各フィールドを設定 |

### Description の抽出（`extractDescription`）

`pendingComments` から以下のパターンを解析します。

```
# Description:        ← マーカー行
# 説明1行目           ← 本文（後続の # コメント行すべて）
# 説明2行目
```

インライン形式も対応:

```
# Description: 単行の説明
```

マーカー行以前のコメントは無視されます。マーカーと `Host` の間に空行があると `pendingComments` がクリアされるため Description は取得されません。

## タブ機能の実装

### 状態管理フロー

```
sshMode()
  └─ buildTabState(hosts) → タグ一覧を収集・ソート、"All" を先頭に
  └─ os.CreateTemp() → 一時ファイル作成
  └─ s.save(statefile)
  └─ fzf 起動（--bind に statefile パスを埋め込む）
  └─ defer os.Remove(statefile)
```

### fzf バインディング

```
Tab       → reload(ffh --tab-list <sf> 1)  + transform-header(ffh --tab-header <sf>)
Shift-Tab → reload(ffh --tab-list <sf> -1) + transform-header(ffh --tab-header <sf>)
```

`--tab-list` はインデックスを更新してからホスト一覧を stdout に出力するため、fzf の `reload()` アクションで直接受け取れます。

### `selfPath()` の重要性

`os.Executable()` はシンボリックリンクのパスを返す場合があるため、`filepath.EvalSymlinks()` で実体パスに解決しています。`/usr/local/bin/ffh` がシンボリックリンクの場合でも fzf からの呼び出しが壊れません。

## ビルド・開発手順

```bash
# ビルド
make build        # ./ffh を生成

# テスト
make test         # go test ./...

# インストール
make install      # /usr/local/bin/ffh にコピー

# クリーン
make clean        # ./ffh バイナリを削除
```

### テスト方針

fzf・ssh を呼び出すコードはテスト対象外とし、純粋なロジック部分のみをテストします。

**parser_test.go**

- ワイルドカードホストのスキップ
- `# Description:` マーカー形式・インライン形式・複数行
- Description と Host の間に空行がある場合（取得しない）
- キーワードの大文字小文字非依存
- `Match` ブロック内ディレクティブの無視
- バックツーバックの Host ブロック
- 末尾改行なしファイル
- Include glob の解決
- 重複ホスト名の first-match-wins

**hosts_test.go**

- ループバック（`127.0.0.1` / `::1`）のスキップ
- コメント行・空行のスキップ
- 複数ホスト名がある行（最初のみ取得）
- ファイルが存在しない場合のエラー

### プレビュー出力の確認

```bash
./ffh --preview-host <ホスト名>
```

### タブ状態の確認

```bash
statefile=$(mktemp)
printf '0\nAll\ndev\nprod\n' > "$statefile"

./ffh --tab-header "$statefile"         # ヘッダー表示
./ffh --tab-list "$statefile" 1         # Tab 1回分の一覧
./ffh --tab-header "$statefile"         # 更新後ヘッダー確認

rm "$statefile"
```

## 依存関係

外部 Go パッケージへの依存はありません（標準ライブラリのみ）。

実行時の外部依存:

| コマンド | 用途 |
|---|---|
| `fzf` | 対話的選択 UI |
| `ssh` | 最終的な SSH 接続（`syscall.Exec` で置換） |

## 注意点

- `Tag` ディレクティブは OpenSSH の公式仕様です（`ssh_config(5)` 参照）。本来は `Match Tag` ディレクティブで設定ブロックを選択するために使われますが、ffh ではホストの分類・絞り込みにも活用しています
- fzf 0.44 未満では `transform-header` アクションが利用できないため、タブ切り替え時にヘッダーが更新されない場合があります
