# ffh

[English](README.en.md) | 日本語

SSH config をパースして、fzf でホストを対話的に選択する CLI ツールです。

## 特徴

- `~/.ssh/config` の `Include` ディレクティブを再帰的に解決
- ホスト選択中に **左プレビューペイン** でホスト詳細を表示
- タブによるホスト絞り込み。**設定ファイル単位**（デフォルト）または **`Tag` ディレクティブ単位** の2種類のグループ化を `Ctrl-T` で切り替え可能。`Ctrl-/` でタブ名をあいまい検索して直接ジャンプ
- タブバー直下の短いヒント表示 + `?` キーで全キーバインド一覧をオーバーレイ表示
- `# Description:` コメントによる説明文の記載（複数行対応）
- 1つの `Host` 行に複数のホスト名を並べた場合、それぞれ個別のエントリとして表示
- 接続履歴の記録・履歴からの再接続（`--history`）
- 重複するホスト定義の検出（`--check`）
- タグを指定して複数ホストへ一括コマンド実行（`--exec`）
- ssh コマンドのクリップボードへのコピー、TCP 到達確認、`ssh -G` 出力の閲覧とインライン編集
- hosts ファイルモード（パスは環境変数・設定ファイル・CLI で指定可能）
- UI 言語は日本語・英語を切り替え可能
- 1Password（`op` CLI）と連携したパスワード自動入力（`op_vault` 設定時のみ有効）

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
| `Ctrl-/` | タブ名をあいまい検索して直接ジャンプ |
| `?` | 全キーバインド一覧をオーバーレイ表示 |
| `Tab` | 次のタブへ移動 |
| `Shift-Tab` | 前のタブへ移動 |
| `Esc` / `Ctrl-C` | キャンセル |
| 文字入力 | ファジー検索 |

タブバーの直下には `Ctrl-/:タブ検索  ?:ヘルプ` という短いヒントが常時表示されます。`?` を押すと、全キーバインドを一覧表示するオーバーレイが開きます（`Esc` または `Enter` で閉じる）。タブバーと隣接して表示が混み合わないよう、常時表示は最小限にとどめています。

### タブによる絞り込み

タブはデフォルトで **ホストが定義されているソースファイル単位** にグループ化されます。`Ctrl-T`（または `--tab-source tag` / `FFH_TAB_SOURCE=tag`）で **`Tag` ディレクティブ単位** のグループ化に切り替えられます。

```
  [ All ]  [ dev ]  [ prod ]
  Ctrl-/:タブ検索  ?:ヘルプ
```

- **All** — 全ホストを表示（デフォルト）
- **タグ名 / ソースファイル名** — 選択中のグループ化方法に属するホストだけを表示

`Tab` / `Shift-Tab` で隣のタブへ順に移動できます。タブ数が多い場合は `Ctrl-/` を押すとタブ名一覧のあいまい検索が開き（k9s の `:` コマンドバーのような操作感）、目的のタブへ一発でジャンプできます。

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

指定した `Tag` を持つ全ホストに対して、SSH 経由で同じコマンドを並列実行し、ホスト名をプレフィックスとして結果を表示します。`op_vault` が設定されている場合はホストごとに1Password連携も適用されますが、複数ホストへ同時接続する性質上、未認証時の認証確認プロンプトは表示されません（該当ホストは通常の対話的パスワード入力にフォールバックします）。

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
tag_delimiter = /
language = ja
op_vault = Private
```

### 1Password連携によるパスワード自動入力

`op_vault` を設定すると、パスワード認証が必要なホストへの接続時に `SSH_ASKPASS` 経由で1Password (`op` CLI) からパスワードを取得し自動入力します(`op` にサインイン済みであることが必要)。現時点で対応しているパスワードマネージャーは1Passwordのみですが、内部的には特定のパスワードマネージャーに依存しない形で実装されており、将来的に他のパスワードマネージャー(Bitwardenなど)を追加しやすい構造になっています(詳細は `AGENTS.md` の "Credential Integration" 参照)。

1Passwordのアイテム名は、ホストごとに登録するのではなく **`ssh -G <host>` で解決される実効ユーザー名** をそのまま使います。同じユーザーでログインするホストが複数あっても、1Passwordには1つのアイテムを作るだけで済みます。

- アイテム名 = 実効 `User`(例: `pocuser` ユーザーでログインするホストは、1Passwordの `pocuser` という名前のアイテムの `password` フィールドを使う)
- 同じユーザー名でもパスワードが異なる例外ケースは、該当ホストに `SetEnv FFH_CREDENTIAL=<item名>` を書いて明示的に上書きできる(次節参照)
- そのアイテムに `username` フィールドが設定されている場合、`ssh_config` 側の `User` より優先してそのユーザー名で接続する(`-l` オプションで上書き)。共有ログイン用に `SetEnv FFH_CREDENTIAL` でアイテムを明示指定しているホストでのみ意味を持つ挙動で、コマンドラインで明示的に `-l` / `-o User=` を指定した場合はそちらが優先される
- 該当ユーザーの1Passwordアイテムが見つからない場合は何もせず、通常の対話的なパスワード入力とssh_configの`User`にフォールバックする(鍵認証のみのホストに影響はない)
- `op` に未認証(`op signin` が必要)の場合は、フォールバックする前にその旨を促すメッセージを表示する(アイテムが単に存在しない場合とは区別され、その場合は何も表示しない)。続けてその場で `op signin` を実行して再試行するか、認証せずそのまま通常のSSH接続(対話的パスワード入力)を継続するかを選択できる。認証を選んだ場合は `op signin` を実行し、成功すれば1Password経由のパスワード自動入力で接続を続行する
- `FFH_OP_VAULT` 環境変数で `op_vault` を上書きできる

---

## SSH config の書き方

### Tag — タブ絞り込み

```ssh-config
Host myserver
    HostName 10.0.0.1
    Tag prod
```

複数ホストに同じ `Tag` を付けると、そのタグのタブでまとめて表示されます（`Ctrl-T` で Tag グループ表示に切り替えたとき）。

#### `tag_delimiter` — 1つの Tag を複数タブに分割

```ssh-config
Host myserver
    HostName 10.0.0.1
    Tag /hoge/fuga/
```

`Tag` の値はデフォルトで `/` を区切り文字として分割され、分割後の各要素がタブとして扱われます。上記の例では `myserver` は `hoge` タブと `fuga` タブの両方に表示されます。先頭・末尾のデリミタによる空要素は無視されるので `/hoge/fuga/` は `["hoge", "fuga"]` になります(`["", "hoge", "fuga", ""]` にはなりません)。区切り文字を含まない通常の `Tag`(例: `prod`)は今まで通り単一のタブになります。

```ini
# ~/.config/ffh/config
tag_delimiter = ,
```

- デフォルトは `/`。別の文字にしたい場合は `tag_delimiter`(設定ファイル)または `FFH_TAG_DELIMITER`(環境変数)で変更できる
- 分割を無効化して `Tag` の値を常に単一のタブとして扱いたい場合は `tag_delimiter = off` / `FFH_TAG_DELIMITER=off` を指定する
- `ffh --exec <tag> <command>` でのタグ一致判定にも同じ分割ロジックが使われるので、`ffh --exec hoge <cmd>` は `Tag /hoge/fuga/` のホストにもマッチします

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

### SetEnv FFH_CREDENTIAL — 1Passwordアイテム名の上書き

```ssh-config
Host poc-str1_agg1_dc4
    HostName 192.168.255.240
    User root
    SetEnv FFH_CREDENTIAL=root-str1agg
```

`op_vault` が設定されている場合、1Passwordのアイテム名はデフォルトで実効 `User` 名になりますが、同じユーザー名でもホストによってパスワードが異なる場合は `SetEnv FFH_CREDENTIAL=<item名>` でホスト単位に上書きできます。`SetEnv` はネイティブな SSH ディレクティブ(OpenSSH 7.8+)なので、`ssh -G` でも構文エラーにならず、複数ホストをまとめる `Match` ブロックにも書けます。

#### ⚠️ `Match all` のような全ホスト共通ブロックに書く場合の注意

`User`/`IdentityFile` などをまとめて設定する目的で `Match all` のような無条件マッチのデフォルトブロックを使っている場合、そこに `SetEnv FFH_CREDENTIAL=<item名>` を書くと **公開鍵認証のみのホストにも同じ設定が継承されます**。ssh_config は同じキーワードについて最初に見つかった値を使うため、より具体的な `Host` ブロックで既に `SetEnv FFH_CREDENTIAL` を設定していない限り、そのホストも同じ1Passwordアイテムを参照してしまいます。

これは特にパスフレーズ付きの秘密鍵を使うホストで問題になります。`SSH_ASKPASS_REQUIRE=force` は鍵のパスフレーズ入力も横取りするため、無関係な1Passwordアイテムの値が渡り鍵認証が失敗する可能性があります。また公開鍵認証が何らかの理由で失敗した場合、無関係なパスワードが自動送信され、ロックアウトポリシーのある機器では意図せずロックされるリスクもあります。

対策として、`SetEnv FFH_CREDENTIAL=<item名>` は本来対象のホスト/タグだけを絞った `Match` ブロックに書くことを推奨します。それでも共通デフォルトブロックから外せない事情がある場合は、次節の `off` で個別に無効化してください。

### SetEnv FFH_CREDENTIAL=off / 空値 — ホスト単位での無効化

```ssh-config
Host keyonly-host
    HostName 192.168.10.5
    User devuser
    SetEnv FFH_CREDENTIAL=off
```

`FFH_CREDENTIAL` に予約語 `off`(大文字小文字を区別しない)、または空値(`SetEnv FFH_CREDENTIAL=`)を指定すると、そのホストは1Password連携を完全にスキップし、通常の鍵認証/対話的パスワード入力にフォールバックします。どちらも動作は同じで、`off` は意図が読み取りやすく、空値は既存の値を削除するだけで無効化できる手軽さがあります。この無効化は該当ホストの `Host` ブロックが `Match all` などの共通デフォルトブロックより**ファイル内で先に**解決される必要があります(ssh_configの「最初に見つかった値が有効」というルールに従うため)。

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

---

## 開発者向け情報

内部アーキテクチャの詳細（ファイル構成、SSH config パーサーの状態機械、タブ機能・1Password連携の実装フローなど）は [DEVELOPMENT.md](DEVELOPMENT.md) を参照してください。
