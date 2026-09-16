# 開発者ドキュメント

## アーキテクチャ概要

ffh は単一バイナリで、通常起動と fzf からのコールバック起動の両方を担います。

```
ffh（通常起動）
  │
  ├─ SSH config パース
  ├─ タブ状態を一時ファイルに保存
  ├─ (op_vault 設定時) 1Password/資格情報バックエンドを解決 → SSH_ASKPASS 用 env を用意
  └─ fzf を起動
         │
         ├─ preview:      ffh --preview-host <name> <sshconfig>
         ├─ Tab/Shift-Tab: ffh --tab-list <statefile> ±1 <sshconfig>       (reload、ヘッダーも含めて出力)
         ├─ Ctrl-T:        ffh --tab-source-toggle <statefile> <sshconfig>  (reload)
         ├─ Ctrl-/:        ffh --tab-jump <statefile>                      (execute → ネスト fzf)
         │                    └─ 選択後 +reload(--tab-list ... 0 ...) で再描画
         ├─ ?:             ffh --show-help                                 (execute → ネスト fzf、情報表示のみ)
         ├─ Ctrl-G:        ffh --ssh-config-view <name> <sshconfig>        (execute → ネスト fzf)
         │                    └─ Enter: ffh --edit-host-option <name> <sshconfig> <kw> <val>
         ├─ Ctrl-Y:        ffh --copy-ssh-cmd <name> <sshconfig>           (execute)
         └─ Ctrl-P:        ffh --check-host <name> <sshconfig>             (preview)

接続実行時（execSSH）:
  ├─ resolveCredential() で該当ホストの資格情報を解決（未設定/未認証以外は握り潰し非致命的）
  ├─ (未認証なら) その場で op signin するか通常接続を続けるかを確認
  └─ syscall.Exec で ssh に完全に差し替わる（SSH_ASKPASS_MODE=1 で自分自身が askpass ヘルパーとして再実行される）
```

fzf の `--preview` / `--bind` に自身のパスを埋め込み、自己呼び出しで各機能を実装しています。1Password連携が有効なホストへの接続時は、`ssh` の `SSH_ASKPASS` にも自身のパスを渡し、`FFH_ASKPASS_MODE=1` で再実行されたときだけパスワードを標準出力に返す専用モードで動作します（詳細は「資格情報（1Password）連携」節を参照）。

## ファイル構成

```
ffh/
├── main.go               エントリポイント・フラグ分岐・sshMode/hostsMode/historyMode/execTag・ssh exec
├── tabs.go               タブ状態管理・ヘッダー描画・タブ一覧/ジャンプ/切替の fzf reload ハンドラ・? ヘルプ
├── clipboard.go          Ctrl-Y のクリップボードコピー
├── editor.go             Ctrl-G のネスト fzf 表示・オプション説明・インラインディレクティブ編集
├── parser.go             SSH config パーサー
├── hosts.go              hosts ファイル読み込み
├── config.go             設定解決（設定ファイル・環境変数）
├── history.go            接続履歴の記録・読み込み（~/.local/share/ffh/history.json）
├── i18n.go               日英メッセージ・ヘルプテキスト・言語解決
├── ssh_options.go        ssh -G オプションの日英説明文（Ctrl-G プレビュー用）
├── credential.go         資格情報バックエンド抽象化（credentialBackend インターフェース、ssh_config 側のアイテム名解決、SSH_ASKPASS 自己再実行）
├── credential_op.go      opBackend: 1Password (`op` CLI) 向けの credentialBackend 実装
├── main_test.go          hasLoginOverride・credentialSSHArgs のユニットテスト
├── tabs_test.go          タブ機能（tagSegments/buildTabState/filterHosts/tabIndexByLabel/renderHeader/formatHelpLines）のユニットテスト
├── editor_test.go        インラインディレクティブ編集のユニットテスト
├── parser_test.go        パーサーのユニットテスト
├── hosts_test.go         hosts パーサーのユニットテスト
├── config_test.go        設定解決のユニットテスト
├── history_test.go       接続履歴のユニットテスト
├── credential_test.go    資格情報解決（バックエンド非依存部分）のユニットテスト
├── credential_op_test.go opBackend 固有（op_vault 解決・fetchSecret/isAuthError/runAskpass・signin）のユニットテスト
├── go.mod                モジュール定義（依存なし）
├── Makefile              ビルド・インストール
├── AGENTS.md             AI エージェント向けハーネス説明（英語）
├── README.md             ユーザー向けドキュメント（日本語）
├── README.en.md          ユーザー向けドキュメント（英語）
└── DEVELOPMENT.md        このファイル（開発者向け詳細説明）
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

### `tabState`（tabs.go）

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
| `--tab-jump <statefile>` | タブ名をあいまい検索するネスト fzf を開き、選択したタブを current に設定 | fzf execute (Ctrl-/) |
| `--show-help` | 全キーバインド一覧をネスト fzf で表示（情報表示のみ） | fzf execute (`?`) |
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
Ctrl-/    → execute(ffh --tab-jump <sf>)+reload(ffh --tab-list <sf> 0 <sshconfig>)
?         → execute(ffh --show-help)   ※情報表示のみ、状態は変更しない
```

`Ctrl-/` はネストした fzf でタブ名をあいまい検索し、選択されたタブを `tabIndexByLabel` で元のインデックスに変換して `statefile` に保存する。その直後に `+reload` で `--tab-list ... 0 ...`（delta=0）を実行し、更新されたインデックスで再描画する。`:` はフォールバックできない — fzf 自身の `--bind` 構文が `KEY:ACTION` の区切りに `:` を使うため、単独の `:` をキーに指定すると `key name required` エラーになる（fzf 0.44.1 で確認済み）。

`--tab-list` / `--tab-source-toggle` はインデックス（またはグループ化方法）を更新してから、1〜2 行目にヘッダー（タブバー行＋常時表示のキーヒント行）・3 行目以降にホスト一覧を stdout に出力します。fzf 側は `--header-lines=2` でヘッダー2行を切り離して表示するため、`transform-header` は使用していません。ヘッダーを再出力するパス（`tabList`/`tabSourceToggle`）は必ず2行とも出力する必要があり、片方だけ出すと外側の fzf のヘッダー行解釈がずれます。

### `selfPath()` の重要性

`os.Executable()` はシンボリックリンクのパスを返す場合があるため、`filepath.EvalSymlinks()` で実体パスに解決しています。`/usr/local/bin/ffh` がシンボリックリンクの場合でも fzf からの呼び出しが壊れません。

## 資格情報（1Password）連携の実装

### `credentialBackend` インターフェース（credential.go）

1Password 専用ではなく、パスワードマネージャー一般を抽象化したインターフェースになっている。

```go
type credentialBackend interface {
	name() string                                         // 例: "op"（FFH_CRED_BACKEND に使う内部識別子）
	displayName() string                                   // 例: "1Password"（メッセージ表示用）
	vault() string                                         // 未設定なら ""
	fetchSecret(vault, item, field string) (string, error)
	isAuthError(err error) bool                            // 「未認証」かどうかの判定
	signin() error                                         // 対話的な認証フロー
	askpassEnv(vault, item string) []string                // SSH_ASKPASS 再実行用の env
}
```

現在の実装は `opBackend`（credential_op.go）のみ。`credentialBackends`（credential.go のスライス）に新しい実装を追加するだけで別のパスワードマネージャー（例: Bitwarden の `bw` CLI）に対応できる設計で、`resolveCredential`/`execSSH`/`runAskpass` 側の変更は不要。

### 接続フロー

```
execSSH(host, args)
  ├─ resolveCredential(sshConfigPath, host)
  │    ├─ op_vault 等が未設定 → (nil, nil, false) で即リターン（鍵認証のみのホストへの影響ゼロ）
  │    ├─ ssh -G でアイテム名を解決（実効 User、または SetEnv FFH_CREDENTIAL 上書き）
  │    └─ backend.fetchSecret(vault, item, "password") で疎通確認
  │         ├─ 成功 → credential{env, username} を返す
  │         ├─ 「未認証」以外の失敗（アイテムなし等） → (nil, nil, false)（無言フォールバック）
  │         └─ 「未認証」による失敗 → (nil, backend, true)
  ├─ notSignedIn が true なら confirmCredSignin(backend) でその場認証するか確認
  │    └─ "y" → backend.signin()（例: op signin を実端末に接続して実行）→ resolveCredential を再試行
  ├─ credentialSSHArgs(host, args, cred) で ssh の argv と追加 env を組み立て
  │    （cred.username があれば -l <username> を追加。呼び出し元が既に -l/-o User= を
  │      指定している場合は上書きしない）
  └─ syscall.Exec で ssh に完全に差し替わる
```

`resolveCredential` が非 nil の `credential` を返すと、`execSSH` は `SSH_ASKPASS_REQUIRE=force` と `SSH_ASKPASS=<selfPath>` を環境変数にセットして `ssh` を起動する。`ssh` がパスワードプロンプトの代わりに `SSH_ASKPASS` へ問い合わせると、`ffh` 自身が `FFH_ASKPASS_MODE=1` で再実行され（`main()` の最初のチェック）、`FFH_CRED_BACKEND`/`FFH_CRED_VAULT`/`FFH_CRED_ITEM` env から該当バックエンドを特定してシークレットを再取得し標準出力に返すだけの専用モードで動く（実パスワードは `resolveCredential` の疎通確認時点では一切保持されず、都度取得し直す）。

`execTag`（`--exec`）も同じ `credentialSSHArgs` を使って各ホストに資格情報を適用するが、対象ホストへ並行接続するため `confirmCredSignin`/`backend.signin()` は呼ばない — 未認証のホストは資格情報なしで通常のフォールバック接続になる。

### `Match all` のようなブロックでの注意

`SetEnv FFH_CREDENTIAL=<item名>` を `Match all` 等の全ホスト共通ブロックに書くと、鍵認証のみのホストにも同じアイテムが継承されてしまう。`SSH_ASKPASS_REQUIRE=force` は鍵のパスフレーズ入力も横取りするため、意図しない資格情報が鍵認証を妨害する可能性がある。対象を絞った `Match` ブロックに書くか、`SetEnv FFH_CREDENTIAL=off`（または空値）でホスト単位に無効化することを推奨する（詳細は README.md 参照）。

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

fzf を呼び出すコードはテスト対象外とし、純粋なロジック部分のみをテストします。`ssh`/`op` に依存するテスト（`credential_op_test.go` など）は、それぞれ `exec.LookPath` で見つからない場合に `t.Skip` します。

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
- `hostBlockMatches` が複数ホスト名行（`Host web1 web2` の2番目、`Host web* db-1` のようにワイルドカードでない名前が先頭以外にある場合）でも正しくマッチすること、および `Host *` のような無条件ブロックには決してマッチしないこと

**tabs_test.go**

- `tagSegments`/`buildTabState`/`filterHosts` の `tag_delimiter` あり/なし両方の挙動
- `tabIndexByLabel` の tag/source 両モードでのマッチング
- `renderHeader` が常にタブバー行とヒント行の2行を出力すること
- `formatHelpLines` が最長の `Key` に揃えて桁を合わせること

**main_test.go**

- `hasLoginOverride` による `-l`/`-o User=` の検出
- `credentialSSHArgs` の `-l <username>` 上書きと、呼び出し元の `-l`/`-o User=` がそれに優先すること

**credential_test.go / credential_op_test.go**

- バックエンド非依存部分: `ssh -G` 出力からのアイテム名解決（`SetEnv` 上書き、`off`/空値による無効化、`Match all` への優先関係）、バックエンド未設定時に `resolveCredential` が `(nil, nil, false)` を返すこと
- `opBackend` 固有部分（`credential_op_test.go`、`op`/`ssh` が PATH にない場合はスキップ）: `op_vault` の解決優先順位、`fetchSecret`/`isAuthError`/`runAskpass` の異常系、フェイクの `op` スクリプトを使った「未認証」判定（`notSignedIn`）、`applyOpSessionEnv` のパース

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
- 1つの `Host` 行に複数のホスト名（`Host web1 web2` 等）がある場合、インライン編集は2番目以降の名前でも正しく対象ブロックを見つけます（`hostBlockMatches` が全トークンを走査するため）。ただし `*`/`?` を含むワイルドカードトークンは常に除外され、`Host *` のような無条件ブロックを誤って書き換えることはありません
- `execTag`（`--exec`）はホストごとに goroutine を1つ立てて並行実行するため、出力は host-by-host の順序ではなくインターリーブされます（`prefix` でどのホストの行か判別可能）
