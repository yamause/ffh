# ffh

[English](README.en.md) | 日本語

SSH config をパースして、fzf でホストを対話的に選択する CLI ツールです。

## 特徴

- `~/.ssh/config` の `Include` ディレクティブを再帰的に解決
- ホスト選択中に **左プレビューペイン** でホスト詳細を表示
- `Tag` ディレクティブによる **タブ切り替え** でホストを絞り込み
- `# Description:` コメントによる説明文の記載（複数行対応）
- hosts ファイルモード（パスは環境変数・設定ファイル・CLI で指定可能）

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

ssh のオプションはそのまま渡せます。

```bash
ffh -L 8080:localhost:8080   # ポートフォワード
ffh -v                        # デバッグ出力
```

### fzf 操作

| キー | 動作 |
|------|------|
| `↑` / `↓` | ホストを選択 |
| `Enter` | SSH 接続 |
| `Ctrl-G` | 選択中ホストの `ssh -G` 全オプションを表示 |
| `Tab` | 次のタグタブへ移動 |
| `Shift-Tab` | 前のタグタブへ移動 |
| `Esc` / `Ctrl-C` | キャンセル |
| 文字入力 | ファジー検索 |

### タブによる絞り込み

`~/.ssh/config` に `Tag` ディレクティブが含まれる場合、ヘッダーにタブが表示されます。

```
  [ All ]  [ dev ]  [ prod ]
```

- **All** — 全ホストを表示（デフォルト）
- **タグ名** — そのタグに属するホストだけを表示

`Tab` / `Shift-Tab` でタブを切り替えます。

### プレビューペイン

ホストにカーソルを合わせると、左ペインにホスト詳細が表示されます。

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

  ────────────────────────────────
  Description
  本番 Web サーバー
  詳細は wiki を参照
```

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

**設定ファイルの例** (`~/.config/ffh/config`):

```ini
# ffh 設定ファイル
hosts_file = /path/to/hosts
```

---

## SSH config の書き方

### Tag — タブ絞り込み

```ssh-config
Host myserver
    HostName 10.0.0.1
    Tag prod
```

複数ホストに同じ `Tag` を付けると、そのタグのタブでまとめて表示されます。

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
