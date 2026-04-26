package main

import (
	"fmt"
	"os"
	"strings"
)

type messages struct {
	helpText                func(ver string) string
	tabAll                  string
	promptSSH               string
	promptHosts             string
	labelHostDetails        string
	labelOptionDesc         string
	configViewHeader        func(hostname string) string
	portDefault             string
	labelDescriptionSection string
	labelValue              string
	labelDesc               string
	noDescription           string
	msgConnectTo            string
	errParseSSHConfig       string
	errReadHostsFile        string
	errSSHNotFound          string
	errExecSSH              string
	errTempFile             string
	optionDescriptions      map[string]string
	// history
	promptHistory         string
	historyHeader         string
	labelLastUsed         string
	labelHistoryConnected string
	msgHistoryEmpty       string
	msgHistoryDeleted     string
	errHistoryNotFound    string
	colHost               string
	colLastUsed           string
	colCount              string
	// time ago
	agoJustNow  string
	agoMinutes  string
	agoHours    string
	agoDays     string
	// clipboard
	errClipboard string
	msgCopied    string
	// check
	statusUp            string
	statusDown          string
	errHostNotFound     string
	msgNoDuplicates     string
	msgDuplicatesFound  string
	labelEffective      string
	labelIgnored        string
	// exec
	errNoHostsForTag string
}

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
  ffh [ssh-options]                    interactive host selection from ~/.ssh/config
  ffh --hosts [path] [ssh-options]     selection from a hosts file
  ffh --history                        show connection history
  ffh --history --delete <host>        delete a history entry
  ffh --check [sshconfig]              detect duplicate host definitions
  ffh --exec <tag> <command...>        run a command on all hosts with the given tag

Options:
  -h, --help                  show this help
  -v, --version               show version
  -F <file>                   use alternative SSH config file (overrides env/config)
  --tab-source <tag|source>   group tabs by source config file (default) or by Tag

SSH config file (priority: -F flag > FFH_SSH_CONFIG env > ssh_config in config > ~/.ssh/config):
  FFH_SSH_CONFIG=/path/to/ssh_config ffh
  echo "ssh_config = /path/to/ssh_config" >> ~/.config/ffh/config

Tab source (priority: --tab-source flag > FFH_TAB_SOURCE env > tab_source in config > tag):
  FFH_TAB_SOURCE=source ffh
  echo "tab_source = source" >> ~/.config/ffh/config

fzf key bindings:
  Enter          connect to selected host
  Ctrl-G         show full ssh -G config for focused host
  Ctrl-Y         copy ssh command to clipboard
  Ctrl-P         check TCP connectivity to focused host
  Ctrl-T         toggle tab grouping between Tag and source file
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
  ffh -L 8080:localhost:8080       forward port after selection
  ffh --tab-source source          group tabs by config file instead of Tag
  ffh --hosts                      select from hosts file
  ffh --hosts /etc/hosts           select from specific hosts file
  ffh --history                    show connection history
  ffh --check                      detect duplicate hosts
  ffh --exec web uptime            run uptime on all hosts tagged "web"
`, ver)
		},
		tabAll:                  "All",
		promptSSH:               "ssh> ",
		promptHosts:             "hosts> ",
		labelHostDetails:        " Host Details ",
		labelOptionDesc:         " Option Description ",
		configViewHeader: func(hostname string) string {
			return fmt.Sprintf(" Ctrl-G: SSH config options for %s  (Esc to close) ", hostname)
		},
		portDefault:             "22 (default)",
		labelDescriptionSection: " Description ",
		labelValue:              "Value:",
		labelDesc:               "Description:",
		noDescription:           "(no description)",
		msgConnectTo:            "Connect to",
		errParseSSHConfig:       "Error parsing SSH config:",
		errReadHostsFile:        "Error reading hosts file:",
		errSSHNotFound:          "ssh not found in PATH",
		errExecSSH:              "exec ssh:",
		errTempFile:             "cannot create temp file:",
		optionDescriptions:      sshOptionDescriptionsEN,
		// history
		promptHistory:         "history> ",
		historyHeader:         " Enter: connect  Ctrl-D: delete entry ",
		labelLastUsed:         "Last Used",
		labelHistoryConnected: "connected",
		msgHistoryEmpty:       "No connection history.",
		msgHistoryDeleted:     "Deleted history entry for",
		errHistoryNotFound:    "No history entry found for",
		colHost:               "Host",
		colLastUsed:           "Last Used",
		colCount:              "Count",
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
	}
}

func jaMessages() messages {
	return messages{
		helpText: func(ver string) string {
			return fmt.Sprintf(`ffh %s — fzf を使った SSH ホスト選択 CLI

使い方:
  ffh [ssh-オプション]                   ~/.ssh/config からホストを対話的に選択
  ffh --hosts [パス] [ssh-オプション]    hosts ファイルからホストを対話的に選択
  ffh --history                          接続履歴を表示
  ffh --history --delete <ホスト>        履歴エントリを削除
  ffh --check [sshconfig]               重複ホスト定義を検出
  ffh --exec <タグ> <コマンド...>        指定タグの全ホストでコマンドを実行

オプション:
  -h, --help                        ヘルプを表示
  -v, --version                     バージョンを表示
  -F <ファイル>                     代替 SSH config ファイルを指定（環境変数・設定ファイルより優先）
  --tab-source <tag|source>         タブをソースファイル（デフォルト）またはタグで分類

SSH config ファイルの優先順位 (-F フラグ > FFH_SSH_CONFIG 環境変数 > 設定ファイルの ssh_config > ~/.ssh/config):
  FFH_SSH_CONFIG=/path/to/ssh_config ffh
  echo "ssh_config = /path/to/ssh_config" >> ~/.config/ffh/config

タブソースの優先順位 (--tab-source フラグ > FFH_TAB_SOURCE 環境変数 > 設定ファイルの tab_source > tag):
  FFH_TAB_SOURCE=source ffh
  echo "tab_source = source" >> ~/.config/ffh/config

fzf キーバインド:
  Enter          選択したホストに SSH 接続
  Ctrl-G         フォーカス中ホストの ssh -G 全設定を表示
  Ctrl-Y         ssh コマンドをクリップボードにコピー
  Ctrl-P         フォーカス中ホストの TCP 疎通確認
  Ctrl-T         タブのグループをタグとソースファイルで切り替え
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
  ffh -L 8080:localhost:8080         選択後にポートフォワード
  ffh --tab-source source            タブをソースファイルで分類
  ffh --hosts                        hosts ファイルから選択
  ffh --hosts /etc/hosts             指定した hosts ファイルから選択
  ffh --history                      接続履歴を表示
  ffh --check                        重複ホストを検出
  ffh --exec web uptime              "web" タグの全ホストで uptime を実行
`, ver)
		},
		tabAll:                  "すべて",
		promptSSH:               "ssh> ",
		promptHosts:             "hosts> ",
		labelHostDetails:        " ホスト詳細 ",
		labelOptionDesc:         " オプション説明 ",
		configViewHeader: func(hostname string) string {
			return fmt.Sprintf(" Ctrl-G: %s の SSH 設定オプション  (Esc で閉じる) ", hostname)
		},
		portDefault:             "22 (デフォルト)",
		labelDescriptionSection: " 説明 ",
		labelValue:              "値:",
		labelDesc:               "説明:",
		noDescription:           "(説明なし)",
		msgConnectTo:            "接続先:",
		errParseSSHConfig:       "SSH config の解析エラー:",
		errReadHostsFile:        "hosts ファイルの読み込みエラー:",
		errSSHNotFound:          "ssh が PATH に見つかりません",
		errExecSSH:              "ssh の実行エラー:",
		errTempFile:             "一時ファイルの作成に失敗:",
		optionDescriptions:      sshOptionDescriptionsJA,
		// history
		promptHistory:         "履歴> ",
		historyHeader:         " Enter: 接続  Ctrl-D: 履歴削除 ",
		labelLastUsed:         "最終接続",
		labelHistoryConnected: "回接続",
		msgHistoryEmpty:       "接続履歴がありません。",
		msgHistoryDeleted:     "履歴を削除しました:",
		errHistoryNotFound:    "履歴が見つかりません:",
		colHost:               "ホスト",
		colLastUsed:           "最終接続",
		colCount:              "回数",
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
	}
}
