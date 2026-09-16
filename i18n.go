package main

import (
	"fmt"
	"os"
	"strings"
)

// keyBinding is one row of the "?" help overlay: a key spec paired with its action.
type keyBinding struct {
	Key    string
	Action string
}

type messages struct {
	helpText                func(ver string) string
	tabAll                  string
	promptSSH               string
	promptHosts             string
	promptTabJump           string
	tabJumpHeader           string
	keyHintsSSH             string
	helpKeyBindings         []keyBinding
	helpModalLabel          string
	helpModalHeader         string
	labelHostDetails        string
	labelOptionDesc         string
	configViewHeader        func(hostname string) string
	portDefault             string
	labelDescriptionSection string
	labelDesc               string
	noDescription           string
	msgConnectTo            string
	errParseSSHConfig       string
	errReadHostsFile        string
	errSSHNotFound          string
	errExecSSH              string
	errTempFile             string
	errUnknownFlag          string
	optionDescriptions      map[string]string
	// credential (password manager backends)
	warnCredNotSignedIn func(backend string) string
	promptCredSignin    func(backend string) string
	errCredSignin       string
	// history
	promptHistory         string
	historyHeader         string
	labelLastUsed         string
	labelHistoryConnected string
	msgHistoryEmpty       string
	msgHistoryDeleted     string
	errHistoryNotFound    string
	// time ago
	agoJustNow string
	agoMinutes string
	agoHours   string
	agoDays    string
	// clipboard
	errClipboard string
	msgCopied    string
	// check
	statusUp           string
	statusDown         string
	errHostNotFound    string
	msgNoDuplicates    string
	msgDuplicatesFound string
	labelEffective     string
	labelIgnored       string
	// exec
	errNoHostsForTag string
	// edit
	editModalLabel  string
	editModalHeader func(hostname, keyword, src, current string) string
	editRollback    string
	errEditNoSource string
}

// msgs is a package-global set by initMessages() at startup and reassigned by
// individual tests (via initMessages()/t.Setenv("FFH_LANG", ...)) to exercise both
// locales. It is not safe for concurrent access -- no test in this codebase uses
// t.Parallel(), and adding it anywhere that reads msgs (directly or via a function
// under test) would race with any other test still reassigning it.
var msgs messages

func initMessages() {
	switch resolveLanguage() {
	case "ja":
		msgs = jaMessages()
	default:
		msgs = enMessages()
	}
}

// resolveLanguage determines the UI language.
// Priority: FFH_LANG env > language in config file > system LANG env > "en"
func resolveLanguage() string {
	if v := os.Getenv("FFH_LANG"); v != "" {
		return normalizeLang(v)
	}
	if v := loadConfig()["language"]; v != "" {
		return normalizeLang(v)
	}
	if strings.HasPrefix(os.Getenv("LANG"), "ja") {
		return "ja"
	}
	return "en"
}

func normalizeLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "ja") {
		return "ja"
	}
	return "en"
}

func enMessages() messages {
	return messages{
		helpText: func(ver string) string {
			return fmt.Sprintf(`ffh %s — SSH host selector powered by fzf

Usage:
  ffh [ffh-options] [-- ssh-options]   interactive host selection from ~/.ssh/config
  ffh --hosts [path] [-- ssh-options]  selection from a hosts file
  ffh --history [-- ssh-options]       show connection history
  ffh --history --delete <host>        delete a history entry
  ffh --check [-F <file>]              detect duplicate host definitions
  ffh --exec <tag> <command...>        run a command on all hosts with the given tag

Options (before '--'):
  -h, --help                  show this help
  -v, --version               show version
  -F <file>                   use alternative SSH config file (overrides env/config)
  --tab-source <tag|source>   group tabs by source config file (default) or by Tag

SSH options (after '--'):
  Any option accepted by ssh(1), e.g. -L, -R, -D, -o, -i ...

SSH config file (priority: -F flag > FFH_SSH_CONFIG env > ssh_config in config > ~/.ssh/config):
  FFH_SSH_CONFIG=/path/to/ssh_config ffh
  echo "ssh_config = /path/to/ssh_config" >> ~/.config/ffh/config

Tab source (priority: --tab-source flag > FFH_TAB_SOURCE env > tab_source in config > tag):
  FFH_TAB_SOURCE=source ffh
  echo "tab_source = source" >> ~/.config/ffh/config

fzf key bindings:
  Enter          connect to selected host
  Ctrl-G         show full ssh -G config for focused host (Enter to edit)
  Ctrl-Y         copy ssh command to clipboard
  Ctrl-P         check TCP connectivity to focused host
  Ctrl-T         toggle tab grouping between Tag and source file
  Ctrl-/         jump to a tab by name (fuzzy search)
  Tab            next tab
  Shift-Tab      previous tab
  Esc / Ctrl-C   cancel

SSH config directives (ffh-specific):
  Tag <name>              group hosts for tab filtering
  # Description: <text>   description shown in the preview pane

Language:
  Set FFH_LANG=ja or add "language = ja" to ~/.config/ffh/config for Japanese.

Examples:
  ffh                              open host selector
  ffh -F ~/work/ssh_config         use alternative SSH config
  ffh -- -L 8080:localhost:8080    forward port after selection
  ffh --tab-source source          group tabs by config file instead of Tag
  ffh --hosts                      select from hosts file
  ffh --hosts /etc/hosts           select from specific hosts file
  ffh --history                    show connection history
  ffh --history -- -v              show history; connect with ssh -v
  ffh --check                      detect duplicate hosts
  ffh --exec web uptime            run uptime on all hosts tagged "web"
`, ver)
		},
		tabAll:        "All",
		promptSSH:     "ssh> ",
		promptHosts:   "hosts> ",
		promptTabJump: "tab> ",
		tabJumpHeader: " Ctrl-/: jump to tab  Enter: select  (Esc to cancel) ",
		keyHintsSSH:   "Ctrl-/:jump tab  ?:help",
		helpKeyBindings: []keyBinding{
			{"Enter", "connect"},
			{"Ctrl-G", "show ssh -G config"},
			{"Ctrl-Y", "copy ssh command"},
			{"Ctrl-P", "check TCP connectivity"},
			{"Ctrl-T", "toggle tab grouping"},
			{"Ctrl-/", "jump to a tab by name"},
			{"Tab/Shift-Tab", "cycle tabs"},
			{"Esc/Ctrl-C", "cancel"},
		},
		helpModalLabel:   " Key Bindings ",
		helpModalHeader:  " (Esc or Enter to close) ",
		labelHostDetails: " Host Details ",
		labelOptionDesc:  " Option Description ",
		configViewHeader: func(hostname string) string {
			return fmt.Sprintf(" Ctrl-G: SSH config options for %s  Enter: edit  (Esc to close) ", hostname)
		},
		portDefault:             "22 (default)",
		labelDescriptionSection: " Description ",
		labelDesc:               "Description:",
		noDescription:           "(no description)",
		msgConnectTo:            "Connect to",
		errParseSSHConfig:       "Error parsing SSH config:",
		errReadHostsFile:        "Error reading hosts file:",
		errSSHNotFound:          "ssh not found in PATH",
		errExecSSH:              "exec ssh:",
		errTempFile:             "cannot create temp file:",
		errUnknownFlag:          "unknown flag %q — SSH options must come after '--', e.g.: ffh -- -L 8080:localhost:8080",
		optionDescriptions:      sshOptionDescriptionsEN,
		// credential (password manager backends)
		warnCredNotSignedIn: func(backend string) string {
			return fmt.Sprintf("%s is not signed in — sign in and try again.", backend)
		},
		promptCredSignin: func(backend string) string {
			return fmt.Sprintf("Authenticate with %s now and retry? [y/N]: ", backend)
		},
		errCredSignin: "sign-in failed:",
		// history
		promptHistory:         "history> ",
		historyHeader:         " Enter: connect  Ctrl-D: delete  Ctrl-G: config  Ctrl-Y: copy  Ctrl-P: check ",
		labelLastUsed:         "Last Used",
		labelHistoryConnected: "connected",
		msgHistoryEmpty:       "No connection history.",
		msgHistoryDeleted:     "Deleted history entry for",
		errHistoryNotFound:    "No history entry found for",
		// time ago
		agoJustNow: "just now",
		agoMinutes: "m ago",
		agoHours:   "h ago",
		agoDays:    "d ago",
		// clipboard
		errClipboard: "clipboard error:",
		msgCopied:    "Copied:",
		// check
		statusUp:           "UP",
		statusDown:         "DOWN",
		errHostNotFound:    "host not found:",
		msgNoDuplicates:    "No duplicate host definitions found.",
		msgDuplicatesFound: "Duplicate host definitions found:",
		labelEffective:     "effective",
		labelIgnored:       "ignored",
		// exec
		errNoHostsForTag: "no hosts found with tag:",
		// edit
		editModalLabel: " Edit directive ",
		editModalHeader: func(hostname, keyword, src, current string) string {
			return fmt.Sprintf("Host: %s  |  %s  |  Source: %s\nCurrent: %s\nEnter to save  Esc to cancel", hostname, keyword, src, current)
		},
		editRollback:    "Syntax error — rolled back:",
		errEditNoSource: "no source file tracked for host:",
	}
}

func jaMessages() messages {
	return messages{
		helpText: func(ver string) string {
			return fmt.Sprintf(`ffh %s — fzf を使った SSH ホスト選択 CLI

使い方:
  ffh [ffh-オプション] [-- ssh-オプション]   ~/.ssh/config からホストを対話的に選択
  ffh --hosts [パス] [-- ssh-オプション]     hosts ファイルからホストを対話的に選択
  ffh --history [-- ssh-オプション]          接続履歴を表示
  ffh --history --delete <ホスト>            履歴エントリを削除
  ffh --check [-F <ファイル>]               重複ホスト定義を検出
  ffh --exec <タグ> <コマンド...>            指定タグの全ホストでコマンドを実行

オプション ('--' より前):
  -h, --help                        ヘルプを表示
  -v, --version                     バージョンを表示
  -F <ファイル>                     代替 SSH config ファイルを指定（環境変数・設定ファイルより優先）
  --tab-source <tag|source>         タブをソースファイル（デフォルト）またはタグで分類

SSH オプション ('--' より後):
  ssh(1) が受け付けるオプションを指定可能。例: -L, -R, -D, -o, -i ...

SSH config ファイルの優先順位 (-F フラグ > FFH_SSH_CONFIG 環境変数 > 設定ファイルの ssh_config > ~/.ssh/config):
  FFH_SSH_CONFIG=/path/to/ssh_config ffh
  echo "ssh_config = /path/to/ssh_config" >> ~/.config/ffh/config

タブソースの優先順位 (--tab-source フラグ > FFH_TAB_SOURCE 環境変数 > 設定ファイルの tab_source > tag):
  FFH_TAB_SOURCE=source ffh
  echo "tab_source = source" >> ~/.config/ffh/config

fzf キーバインド:
  Enter          選択したホストに SSH 接続
  Ctrl-G         フォーカス中ホストの ssh -G 全設定を表示（Enter で編集）
  Ctrl-Y         ssh コマンドをクリップボードにコピー
  Ctrl-P         フォーカス中ホストの TCP 疎通確認
  Ctrl-T         タブのグループをタグとソースファイルで切り替え
  Ctrl-/         タブ名で絞り込んでジャンプ（あいまい検索）
  Tab            次のタブへ移動
  Shift-Tab      前のタブへ移動
  Esc / Ctrl-C   キャンセル

SSH config ディレクティブ (ffh 独自):
  Tag <名前>                タブ絞り込み用グループ
  # Description: <テキスト>  プレビューペインに表示される説明文

言語切り替え:
  FFH_LANG=en を設定するか ~/.config/ffh/config に "language = en" を追加すると英語になります。

使用例:
  ffh                                ホスト選択画面を開く
  ffh -F ~/work/ssh_config           代替 SSH config を使用
  ffh -- -L 8080:localhost:8080      選択後にポートフォワード
  ffh --tab-source source            タブをソースファイルで分類
  ffh --hosts                        hosts ファイルから選択
  ffh --hosts /etc/hosts             指定した hosts ファイルから選択
  ffh --history                      接続履歴を表示
  ffh --history -- -v                履歴から選択して ssh -v で接続
  ffh --check                        重複ホストを検出
  ffh --exec web uptime              "web" タグの全ホストで uptime を実行
`, ver)
		},
		tabAll:        "すべて",
		promptSSH:     "ssh> ",
		promptHosts:   "hosts> ",
		promptTabJump: "tab> ",
		tabJumpHeader: " Ctrl-/: タブへジャンプ  Enter: 選択  (Esc でキャンセル) ",
		keyHintsSSH:   "Ctrl-/:タブ検索  ?:ヘルプ",
		helpKeyBindings: []keyBinding{
			{"Enter", "接続"},
			{"Ctrl-G", "ssh -G 設定を表示"},
			{"Ctrl-Y", "ssh コマンドをコピー"},
			{"Ctrl-P", "TCP 疎通確認"},
			{"Ctrl-T", "タブのグループ切替"},
			{"Ctrl-/", "タブ名で検索してジャンプ"},
			{"Tab/Shift-Tab", "タブ移動"},
			{"Esc/Ctrl-C", "キャンセル"},
		},
		helpModalLabel:   " キーバインド ",
		helpModalHeader:  " (Esc または Enter で閉じる) ",
		labelHostDetails: " ホスト詳細 ",
		labelOptionDesc:  " オプション説明 ",
		configViewHeader: func(hostname string) string {
			return fmt.Sprintf(" Ctrl-G: %s の SSH 設定オプション  Enter: 編集  (Esc で閉じる) ", hostname)
		},
		portDefault:             "22 (デフォルト)",
		labelDescriptionSection: " 説明 ",
		labelDesc:               "説明:",
		noDescription:           "(説明なし)",
		msgConnectTo:            "接続先:",
		errParseSSHConfig:       "SSH config の解析エラー:",
		errReadHostsFile:        "hosts ファイルの読み込みエラー:",
		errSSHNotFound:          "ssh が PATH に見つかりません",
		errExecSSH:              "ssh の実行エラー:",
		errTempFile:             "一時ファイルの作成に失敗:",
		errUnknownFlag:          "不明なフラグ %q — SSH オプションは '--' の後に指定してください。例: ffh -- -L 8080:localhost:8080",
		optionDescriptions:      sshOptionDescriptionsJA,
		// credential (password manager backends)
		warnCredNotSignedIn: func(backend string) string {
			return fmt.Sprintf("%s にサインインしていません。認証してから再度お試しください。", backend)
		},
		promptCredSignin: func(backend string) string {
			return fmt.Sprintf("今すぐ%sで認証して再試行しますか？ [y/N]: ", backend)
		},
		errCredSignin: "認証に失敗しました:",
		// history
		promptHistory:         "履歴> ",
		historyHeader:         " Enter: 接続  Ctrl-D: 削除  Ctrl-G: 設定表示  Ctrl-Y: コピー  Ctrl-P: 疎通確認 ",
		labelLastUsed:         "最終接続",
		labelHistoryConnected: "回接続",
		msgHistoryEmpty:       "接続履歴がありません。",
		msgHistoryDeleted:     "履歴を削除しました:",
		errHistoryNotFound:    "履歴が見つかりません:",
		// time ago
		agoJustNow: "たった今",
		agoMinutes: "分前",
		agoHours:   "時間前",
		agoDays:    "日前",
		// clipboard
		errClipboard: "クリップボードエラー:",
		msgCopied:    "コピーしました:",
		// check
		statusUp:           "UP",
		statusDown:         "DOWN",
		errHostNotFound:    "ホストが見つかりません:",
		msgNoDuplicates:    "重複するホスト定義はありません。",
		msgDuplicatesFound: "重複するホスト定義が見つかりました:",
		labelEffective:     "有効",
		labelIgnored:       "無視",
		// exec
		errNoHostsForTag: "指定タグのホストが見つかりません:",
		// edit
		editModalLabel: " ディレクティブ編集 ",
		editModalHeader: func(hostname, keyword, src, current string) string {
			return fmt.Sprintf("ホスト: %s  |  %s  |  ソース: %s\n現在の値: %s\nEnter で保存  Esc でキャンセル", hostname, keyword, src, current)
		},
		editRollback:    "構文エラー — ロールバックしました:",
		errEditNoSource: "ホストのソースファイルが不明:",
	}
}
