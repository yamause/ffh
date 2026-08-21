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
         ├─ preview:      ffh --preview-host <name> <sshconfig>
         ├─ Tab/Shift-Tab: ffh --tab-list <statefile> ±1 <sshconfig>       (reload、ヘッダーも含めて出力)
         ├─ Ctrl-T:        ffh --tab-source-toggle <statefile> <sshconfig>  (reload)
         ├─ Ctrl-G:        ffh --ssh-config-view <name> <sshconfig>        (execute → ネスト fzf)
         │                    └─ Enter: ffh --edit-host-option <name> <sshconfig> <kw> <val>
         ├─ Ctrl-Y:        ffh --copy-ssh-cmd <name> <sshconfig>           (execute)
         └─ Ctrl-P:        ffh --check-host <name> <sshconfig>             (preview)
```

fzf の `--preview` / `--bind` に自身のパスを埋め込み、自己呼び出しで各機能を実装しています。

## ファイル構成

```
ffh/
├── main.go          エントリポイント・fzf 起動・タブ状態管理・クリップボード・接続確認・インライン編集
├── parser.go        SSH config パーサー
├── hosts.go         hosts ファイル読み込み
├── config.go        設定解決（設定ファイル・環境変数）
├── history.go       接続履歴の記録・読み込み（~/.local/share/ffh/history.json）
├── i18n.go          日英メッセージ・ヘルプテキスト・言語解決
├── ssh_options.go   ssh -G オプションの日英説明文（Ctrl-G プレビュー用）
├── editor_test.go   インラインディレクティブ編集のユニットテスト
├── parser_test.go   パーサーのユニットテスト
├── hosts_test.go    hosts パーサーのユニットテスト
├── config_test.go   設定解決のユニットテスト
├── history_test.go  接続履歴のユニットテスト
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
source     ← グループ化方法（"tag" または "source"）
All        ← tags[0]（常に "All"）
dev        ← tags[1]
prod       ← tags[2]
```

`Ctrl-T`（`--tab-source-toggle`）でグループ化方法を切り替えると、タグ一覧が再構築されインデックスは 0 に戻ります。

## コマンドライン引数

| 引数 | 用途 | 呼び出し元 |
|---|---|---|
| `（なし）` | SSH モードで起動 | ユーザー |
| `-F <file>` | 使用する SSH config を指定 | ユーザー |
| `--tab-source <tag\|source>` | タブのグループ化方法を指定（デフォルト `source`） | ユーザー |
| `--hosts [path]` | hosts ファイルモード（パス解決は `resolveHostsPath` 参照） | ユーザー |
| `--history` | 接続履歴から選択 | ユーザー |
| `--history --delete <host>` | 履歴エントリを削除 | ユーザー |
| `--check` | 重複ホスト定義を検出 | ユーザー |
| `--exec <tag> <command...>` | 指定タグの全ホストでコマンドを並列実行 | ユーザー |
| `--preview-host <name> [<sshconfig>]` | プレビューペイン出力 | fzf preview |
| `--tab-list <statefile> <delta> [<sshconfig>]` | タブ切り替え＋ヘッダー・ホスト一覧出力 | fzf reload (Tab/Shift-Tab) |
| `--tab-source-toggle <statefile> [<sshconfig>]` | タブのグループ化（tag/source）切り替え | fzf reload (Ctrl-T) |
| `--ssh-config-view <hostname> [<sshconfig>]` | `ssh -G` 全オプションをネスト fzf で表示 | fzf execute (Ctrl-G) |
| `--edit-host-option <host> <sshconfig> <kw> [val]` | ディレクティブのインライン編集ダイアログ | ネスト fzf execute (Ctrl-G 内 Enter) |
| `--preview-option <option-line>` | SSH オプションの説明を出力 | ネスト fzf preview |
| `--check-host <name> [<sshconfig>]` | TCP 到達確認（UP/DOWN）を出力 | fzf preview (Ctrl-P) |
| `--copy-ssh-cmd <name> [<sshconfig>]` | ssh コマンドをクリップボードにコピー | fzf execute (Ctrl-Y) |
| `--history --list` | 履歴一覧を出力 | fzf reload (Ctrl-D) |

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
Tab       → reload(ffh --tab-list <sf> 1 <sshconfig>)
Shift-Tab → reload(ffh --tab-list <sf> -1 <sshconfig>)
Ctrl-T    → reload(ffh --tab-source-toggle <sf> <sshconfig>)
```

`--tab-list` / `--tab-source-toggle` はインデックス（またはグループ化方法）を更新してから、1 行目にヘッダー・2 行目以降にホスト一覧を stdout に出力します。fzf 側は `--header-lines=1` でヘッダー行を切り離して表示するため、`transform-header` は使用していません。

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

**config_test.go**

- SSH config パス・hosts ファイルパス・タブグループ化方法・言語の優先順位解決
- `~/.config/ffh/config` のパース（コメント・空行のスキップ）

**history_test.go**

- 履歴の記録・更新（同一ホストへの再接続で `ConnCount` が増加）
- 最終接続時刻順のソート、エントリ削除

**editor_test.go**

- ディレクティブの更新・新規挿入（インデント保持、末尾改行の保持）
- `ssh -G` による構文チェックとロールバック（`ssh` が PATH にない場合はスキップ）

### プレビュー出力の確認

```bash
./ffh --preview-host <ホスト名>
```

### タブ状態の確認

`--tab-header` という単独コマンドは存在しません。ヘッダーは `--tab-list` / `--tab-source-toggle` の出力 1 行目に含まれます。

```bash
statefile=$(mktemp)
printf '0\ntag\nAll\ndev\nprod\n' > "$statefile"

./ffh --tab-list "$statefile" 1 ~/.ssh/config          # Tab 1回分：ヘッダー＋ホスト一覧
./ffh --tab-source-toggle "$statefile" ~/.ssh/config   # グループ化方法を切り替え

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
- タブのグループ化方法は `--tab-source` / `FFH_TAB_SOURCE` / 設定ファイルの `tab_source` で指定でき、デフォルトはソースファイル単位（`source`）です。`Ctrl-T` で `tag` / `source` を対話的に切り替えられます
- ディレクティブのインライン編集（Ctrl-G → Enter）は変更後に `ssh -G` で構文検証し、失敗時は元の内容にロールバックします
