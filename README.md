# ffh

[English](README.en.md) | 日本語

SSH config をパースして、fzf でホストを対話的に選択する CLI ツールです。

## 特徴

- `~/.ssh/config` の `Include` ディレクティブを再帰的に解決
- ホスト選択中に **左プレビューペイン** でホスト詳細を表示
- タブによるホスト絞り込み。**設定ファイル単位**（デフォルト）または **`Tag` ディレクティブ単位** の2種類のグループ化を `Ctrl-T` で切り替え可能
- `# Description:` コメントによる説明文の記載（複数行対応）
- 1つの `Host` 行に複数のホスト名を並べた場合、それぞれ個別のエントリとして表示
- 接続履歴の記録・履歴からの再接続（`--history`）
- 重複するホスト定義の検出（`--check`）
- タグを指定して複数ホストへ一括コマンド実行（`--exec`）
- ssh コマンドのクリップボードへのコピー、TCP 到達確認、`ssh -G` 出力の閲覧とインライン編集
- hosts ファイルモード（パスは環境変数・設定ファイル・CLI で指定可能）
- UI 言語は日本語・英語を切り替え可能

## インストール

```bash
# 依存ツールのインストール（未導入の場合）
sudo apt install fzf

# ビルドとインストール
make install   # /usr/local/bin/ffh に配置
```

**必要な環境**

| ツール | バージョン |
|--------|-----------|
| Go     | 1.24+     |
| fzf    | 0.44+     |
| ssh    | 任意      |

## 使い方

### 基本

```bash
ffh
```

fzf が起動し、`~/.ssh/config` に定義されたホスト一覧が表示されます。ホストを選択すると `ssh <host>` を実行します。

ssh のオプションは `--` の後に渡します。

```bash
ffh -- -L 8080:localhost:8080   # ポートフォワード
ffh -- -v                       # デバッグ出力
```

### コマンドラインオプション

| オプション | 用途 |
|---|---|
| `-h`, `--help` | ヘルプを表示 |
| `-v`, `--version` | バージョンを表示 |
| `-F <file>` | 使用する SSH config ファイルを指定（環境変数・設定ファイルより優先） |
| `--tab-source <tag\|source>` | タブのグループ化方法を指定（デフォルト: `source`） |
| `--hosts [path]` | hosts ファイルモードで起動 |
| `--history` | 接続履歴から選択 |
| `--history --delete <host>` | 履歴エントリを削除 |
| `--check` | 重複するホスト定義を検出 |
| `--exec <tag> <command...>` | 指定タグの全ホストでコマンドを実行 |

### fzf 操作

| キー | 動作 |
|------|------|
| `↑` / `↓` | ホストを選択 |
| `Enter` | SSH 接続 |
| `Ctrl-G` | 選択中ホストの `ssh -G` 全オプションを表示（さらに `Enter` でその場編集） |
| `Ctrl-Y` | 選択中ホストへの `ssh` コマンドをクリップボードにコピー |
| `Ctrl-P` | 選択中ホストへの TCP 到達確認をプレビューに表示 |
| `Ctrl-T` | タブのグループ化を `Tag` / 設定ファイル単位で切り替え |
| `Tab` | 次のタブへ移動 |
| `Shift-Tab` | 前のタブへ移動 |
| `Esc` / `Ctrl-C` | キャンセル |
| 文字入力 | ファジー検索 |

### タブによる絞り込み

タブはデフォルトで **ホストが定義されているソースファイル単位** にグループ化されます。`Ctrl-T`（または `--tab-source tag` / `FFH_TAB_SOURCE=tag`）で **`Tag` ディレクティブ単位** のグループ化に切り替えられます。

```
  [ All ]  [ dev ]  [ prod ]
```

- **All** — 全ホストを表示（デフォルト）
- **タグ名 / ソースファイル名** — 選択中のグループ化方法に属するホストだけを表示

`Tab` / `Shift-Tab` でタブを切り替えます。

### プレビューペイン

ホストにカーソルを合わせると、左ペインにホスト詳細が表示されます。接続履歴がある場合は最終接続時刻と接続回数も表示されます。

```
  Host:           myserver
  ────────────────────────────────
  HostName:       10.0.0.1
  User:           admin
  Port:           22 (default)
  ProxyJump:      bastion
  IdentityFile:   ~/.ssh/id_ed25519
  Tag:            prod
  Source:         ~/.ssh/config.d/servers
  Last Used:      3日前 (5回接続)

  ────────────────────────────────
  Description
  本番 Web サーバー
  詳細は wiki を参照
```

### SSH オプション表示・インライン編集（Ctrl-G）

`Ctrl-G` を押すと、選択中ホストの `ssh -G <host>` 出力を一覧表示するネスト fzf が開きます。各行にカーソルを合わせると右ペインにそのオプションの説明（日本語/英語）が表示されます。

オプション行で `Enter` を押すと、値を編集する小さな入力ダイアログが開きます。保存すると元の SSH config ファイルにディレクティブが書き込まれ、`ssh -G` で構文チェックが行われます。エラーがあれば変更は自動的にロールバックされます。

### クリップボードコピー（Ctrl-Y）

`Ctrl-Y` で選択中ホストへの `ssh` コマンド（`-l`/`-p`/`-J` を含む）をクリップボードにコピーします。`wl-copy` / `xclip` / `xsel` / `pbcopy` のいずれかが必要です。

### TCP 到達確認（Ctrl-P）

`Ctrl-P` を押すとプレビューペインが切り替わり、選択中ホストの SSH ポートへの TCP 接続確認結果（UP/DOWN と応答時間）を表示します。

### 接続履歴（--history）

```bash
ffh --history          # 履歴から選択して接続
ffh --history --delete myserver   # 履歴エントリを削除
```

接続すると自動的に `~/.local/share/ffh/history.json` に記録されます。履歴一覧では最終接続時刻と接続回数で表示され、`Ctrl-D` でエントリを削除できます。

### 重複ホスト検出（--check）

```bash
ffh --check
```

`Include` で読み込む複数の設定ファイル間で同名の `Host` が重複していないかを検出します。実際に有効になる定義（最初に出現したもの）と、無視される定義を区別して表示します。

### タグへの一括コマンド実行（--exec）

```bash
ffh --exec web uptime
```

指定した `Tag` を持つ全ホストに対して、SSH 経由で同じコマンドを並列実行し、ホスト名をプレフィックスとして結果を表示します。

### hosts ファイルモード

```bash
ffh --hosts                        # 設定で解決されたファイルを使用
ffh --hosts /path/to/custom/hosts  # 任意のパスを直接指定
```

hosts ファイルを読み込み、fzf でホストを選択して SSH 接続します。ループバックアドレス（`127.x.x.x`、`::1`）は除外されます。

使用するファイルは以下の優先順位で決定されます。

| 優先度 | 方法 | 例 |
| --- | --- | --- |
| 1 | CLI 引数 | `ffh --hosts /path/to/hosts` |
| 2 | 環境変数 `FFH_HOSTS_FILE` | `export FFH_HOSTS_FILE=/path/to/hosts` |
| 3 | 設定ファイル `~/.config/ffh/config` | `hosts_file = /path/to/hosts` |
| 4 | デフォルト | `/etc/hosts` |

### 使用する SSH config ファイルの指定

以下の優先順位で決定されます。

| 優先度 | 方法 | 例 |
| --- | --- | --- |
| 1 | CLI 引数 `-F` | `ffh -F ~/work/ssh_config` |
| 2 | 環境変数 `FFH_SSH_CONFIG` | `export FFH_SSH_CONFIG=/path/to/ssh_config` |
| 3 | 設定ファイル `~/.config/ffh/config` | `ssh_config = /path/to/ssh_config` |
| 4 | デフォルト | `~/.ssh/config` |

### 言語設定

デフォルトは、システムの `LANG` が `ja` で始まる場合は日本語、それ以外は英語です。以下の優先順位で上書きできます。

| 優先度 | 方法 | 例 |
| --- | --- | --- |
| 1 | 環境変数 `FFH_LANG` | `FFH_LANG=en ffh` |
| 2 | 設定ファイル `~/.config/ffh/config` | `language = en` |
| 3 | システム `LANG` | `ja` で始まれば日本語 |

**設定ファイルの例** (`~/.config/ffh/config`):

```ini
# ffh 設定ファイル
hosts_file = /path/to/hosts
ssh_config = /path/to/ssh_config
tab_source = tag
language = ja
```

---

## SSH config の書き方

### Tag — タブ絞り込み

```ssh-config
Host myserver
    HostName 10.0.0.1
    Tag prod
```

複数ホストに同じ `Tag` を付けると、そのタグのタブでまとめて表示されます（`Ctrl-T` で Tag グループ表示に切り替えたとき）。

### Description — 説明文

**単行:**

```ssh-config
# Description: 本番 Web サーバー
Host myserver
    HostName 10.0.0.1
```

**複数行（`# Description:` をマーカーとして記述）:**

```ssh-config
# Description:
# 本番 Web サーバー
# 詳細は wiki を参照
Host myserver
    HostName 10.0.0.1
```

- `# Description:` 行がマーカーです。それ以降の `#` コメント行が説明文の本文になります
- `# Description:` と `Host` の間に空行を入れると Description は取得されません

### 設定例

```ssh-config
# Description:
# EVE-NG の踏み台サーバー
# ProxyJump 経由でアクセス
Host bastion
    HostName 203.0.113.10
    User ec2-user
    IdentityFile ~/.ssh/bastion_key
    Tag infra

Host dev-server
    HostName 10.0.1.20
    User admin
    ProxyJump bastion
    Tag dev

Host prod-db
    HostName 10.0.2.30
    User dbadmin
    ProxyJump bastion
    Tag prod
```
