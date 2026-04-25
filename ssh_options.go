package main

// sshOptionDescriptionsJA maps lowercase ssh_config(5) keywords to Japanese descriptions.
var sshOptionDescriptionsJA = map[string]string{
	// --- 接続先 ---
	"host":          "SSH config の Host ブロック名。マッチングパターンとして機能する。",
	"hostname":      "実際に接続するホスト名または IP アドレス。省略時は Host 名をそのまま使用。",
	"port":          "接続先の SSH ポート番号。デフォルトは 22。",
	"user":          "SSH 接続時のリモートユーザー名。省略時はローカルのユーザー名を使用。",
	"addressfamily": "使用する IP アドレスファミリ。any / inet (IPv4) / inet6 (IPv6) のいずれかを指定。",

	// --- 認証 ---
	"identityfile":                "公開鍵認証に使用する秘密鍵ファイルのパス。複数指定可。",
	"identitiesonly":              "yes の場合、ssh-agent や PKCS11 を無視し IdentityFile のみを使用する。",
	"pubkeyauthentication":        "公開鍵認証を行うかどうか。デフォルトは yes。",
	"passwordauthentication":      "パスワード認証を許可するかどうか。デフォルトは yes。",
	"kbdinteractiveauthentication": "キーボードインタラクティブ認証を許可するかどうか。",
	"hostbasedauthentication":     "ホストベース認証を使用するかどうか。デフォルトは no。",
	"gssapiauthentication":        "GSSAPI (Kerberos 等) による認証を使用するかどうか。",
	"gssapidelegatecredentials":   "GSSAPI 資格情報をリモートホストに委譲するかどうか。",
	"gssapikeyexchange":           "GSSAPI を使った鍵交換を行うかどうか。",
	"numberofpasswordprompts":     "パスワード認証の最大試行回数。デフォルトは 3。",
	"addkeystoagent":              "認証に成功した秘密鍵を ssh-agent に自動追加するかどうか。",
	"preferredauthentications":    "認証方式の優先順位をカンマ区切りで指定する。",

	// --- プロキシ・ジャンプ ---
	"proxyjump":      "接続先へのジャンプホスト（踏み台サーバー）を指定する。カンマ区切りで複数段指定可。",
	"proxycommand":   "SSH 接続確立前に実行するプロキシコマンド。%h:%p でホスト・ポートを展開できる。",
	"proxyusefdpass": "ProxyCommand でファイルディスクリプタを渡すかどうか。",

	// --- セキュリティ・検証 ---
	"stricthostkeychecking":       "ホスト鍵の確認動作。ask / yes / no / accept-new のいずれかを指定。",
	"userknownhostsfile":          "ユーザーの known_hosts ファイルのパス。",
	"globalknownhostsfile":        "システム全体の known_hosts ファイルのパス。",
	"checkhostip":                 "known_hosts でホスト名に加え IP アドレスも検証するかどうか。",
	"verifyhostkeydns":            "DNS の SSHFP レコードでホスト鍵を検証するかどうか。",
	"hashknownhosts":              "yes の場合、known_hosts のホスト名をハッシュ化して記録する。",
	"updatehostkeys":              "接続時にサーバーから提示された最新ホスト鍵を known_hosts に追記するかどうか。",
	"fingerprinthash":             "ホスト鍵フィンガープリントの表示に使用するハッシュアルゴリズム。デフォルトは SHA256。",
	"hostkeyalias":                "known_hosts に記録する際のホスト鍵エイリアス名。",
	"enablesshkeysign":            "HostbasedAuthentication で ssh-keysign の使用を許可するかどうか。",
	"hostbasedacceptedalgorithms": "ホストベース認証で受け入れる鍵アルゴリズムのリスト。",

	// --- 転送・トンネル ---
	"forwardagent":         "ssh-agent 転送を有効にするかどうか。有効にするとリモート側でも鍵を使える。",
	"forwardx11":           "X11 転送を有効にするかどうか。",
	"forwardx11trusted":    "yes の場合、X11 転送を信頼モードで行う（セキュリティが低下する）。",
	"forwardx11timeout":    "X11 転送の認証情報の有効期限（秒）。",
	"gatewayports":         "リモートの転送ポートをリモート側の全インターフェースでリッスンするかどうか。",
	"tunnel":               "VPN トンネルデバイス転送を使用するかどうか。デフォルトは no。",
	"tunneldevice":         "トンネルデバイスの指定（ローカル:リモート）。",
	"permitlocalcommand":   "LocalCommand ディレクティブの実行を許可するかどうか。",
	"permitremoteopen":     "RemoteForward による接続先として許可するホスト:ポートを制限する。",
	"clearallforwardings":  "yes の場合、他の設定で定義されたすべての転送設定をキャンセルする。",
	"exitonforwardfailure": "ポートフォワーディングに失敗した場合に接続を切断するかどうか。",

	// --- 接続維持・タイムアウト ---
	"serveraliveinterval": "サーバーからの応答がない場合にキープアライブパケットを送る間隔（秒）。0 で無効。",
	"serveralivecountmax": "キープアライブ未応答の最大回数。超えると接続を切断する。デフォルトは 3。",
	"connecttimeout":      "SSH 接続の TCP タイムアウト（秒）。デフォルトはシステムの TCP タイムアウトに従う。",
	"connectionattempts":  "接続失敗時の再試行回数。デフォルトは 1。",
	"tcpkeepalive":        "TCP レベルのキープアライブを送信するかどうか。デフォルトは yes。",
	"controlmaster":       "接続多重化（ControlMaster）を有効にするかどうか。auto / yes / no 等を指定。",
	"controlpersist":      "ControlMaster 接続をバックグラウンドで維持する時間。yes / 秒数を指定。",
	"controlpath":         "ControlMaster 接続を管理するソケットファイルのパス。%h %p %r 等で展開される。",

	// --- 暗号・アルゴリズム ---
	"ciphers":                 "使用する対称暗号アルゴリズムのリスト（優先順）。",
	"macs":                    "使用する MAC（メッセージ認証）アルゴリズムのリスト（優先順）。",
	"kexalgorithms":           "使用する鍵交換アルゴリズムのリスト（優先順）。",
	"hostkeyalgorithms":       "サーバーのホスト鍵として受け入れるアルゴリズムのリスト。",
	"pubkeyacceptedalgorithms": "公開鍵認証で使用する鍵アルゴリズムのリスト。",
	"casignaturealgorithms":   "CA 署名に使用するアルゴリズムのリスト。",
	"requiredrsasize":         "RSA 鍵の最小ビット長。これより短い鍵は拒否される。",
	"gssapikexalgorithms":     "GSSAPI 鍵交換で使用するアルゴリズムのリスト。",

	// --- 端末・セッション ---
	"requesttty":              "疑似端末 (PTY) の割り当てポリシー。auto / yes / no / force のいずれか。",
	"sessiontype":             "セッションの種類。default / none / subsystem のいずれかを指定。",
	"stdinnull":               "yes の場合、stdin を /dev/null にリダイレクトする。",
	"escapechar":              "エスケープシーケンスのプレフィックス文字。デフォルトは ~。",
	"enableescapecommandline": "エスケープ文字によるコマンドライン入力を有効にするかどうか。",
	"loglevel":                "ログの詳細度。QUIET / FATAL / ERROR / INFO / VERBOSE / DEBUG 等を指定。",
	"logverbose":              "特定のモジュール・関数のみ詳細ログを出力する。",
	"syslogfacility":          "syslog に出力する際のファシリティ名。デフォルトは USER。",
	"batchmode":               "yes の場合、パスワード等の対話入力を無効化する。スクリプト用。",
	"forkafterauthentication": "認証後にバックグラウンドプロセスに切り替えるかどうか。",
	"localcommand":            "接続確立後にローカルで実行するコマンド（PermitLocalCommand が yes の場合のみ）。",

	// --- 環境・その他 ---
	"sendenv":             "リモートサーバーに転送する環境変数名のパターン。",
	"setenv":              "リモートセッションに設定する環境変数を KEY=VALUE 形式で指定する。",
	"xauthlocation":       "xauth コマンドの絶対パス。X11 転送時に使用。",
	"ipqos":               "IPv4 の QoS (DiffServ) クラスの指定。対話セッションと非対話セッションで別々に指定可。",
	"rekeylimit":          "再鍵交換を行うデータ量と時間の上限。",
	"compression":         "データ圧縮を使用するかどうか。低速回線では有効に。",
	"canonicalizehostname":      "接続前にホスト名を DNS で正規化するかどうか。",
	"canonicaldomains":          "ホスト名正規化時に付加する検索ドメインのリスト。",
	"canonicalizefallbacklocal": "正規化に失敗した場合に非修飾ホスト名で接続を試みるかどうか。",
	"canonicalizemaxdots":       "正規化対象とするホスト名のドット数の上限。デフォルトは 1。",
	"streamlocalbindmask":       "Unix ドメインソケット転送時のファイル権限マスク。",
	"streamlocalbindunlink":     "Unix ドメインソケット転送前に既存ソケットを削除するかどうか。",
	"securitykeyprovider":       "FIDO/U2F セキュリティキーのプロバイダライブラリパス。",
	"nohostauthenticationforlocalhost": "ループバックアドレスへの接続でホスト認証をスキップするかどうか。",
	"channeltimeout":        "非アクティブなチャンネルを自動的に閉じるまでのタイムアウト。",
	"obscurekeystroketiming": "キーストロークのタイミング情報を難読化するかどうか。",
	"gssapitrustdns":        "GSSAPI 認証で DNS を信頼するかどうか。",
	"gssapirenewalforcesrekey": "GSSAPI 資格情報の更新時に鍵再交換を強制するかどうか。",
	"tag": "ffh 独自ディレクティブ。ホストをグループ化してタブフィルタリングに使用する。",
}

// sshOptionDescriptionsEN maps lowercase ssh_config(5) keywords to English descriptions.
var sshOptionDescriptionsEN = map[string]string{
	// --- Connection target ---
	"host":          "Host block name in SSH config. Acts as a matching pattern.",
	"hostname":      "Actual hostname or IP address to connect to. Defaults to the Host name if omitted.",
	"port":          "SSH port number on the remote host. Default is 22.",
	"user":          "Remote username for the SSH connection. Defaults to the local username.",
	"addressfamily": "IP address family to use: any, inet (IPv4), or inet6 (IPv6).",

	// --- Authentication ---
	"identityfile":                "Path to the private key file used for public-key authentication. Multiple entries allowed.",
	"identitiesonly":              "If yes, ignore ssh-agent and PKCS11 and use only the configured IdentityFile.",
	"pubkeyauthentication":        "Whether to attempt public-key authentication. Default is yes.",
	"passwordauthentication":      "Whether to allow password authentication. Default is yes.",
	"kbdinteractiveauthentication": "Whether to allow keyboard-interactive authentication.",
	"hostbasedauthentication":     "Whether to use host-based authentication. Default is no.",
	"gssapiauthentication":        "Whether to use GSSAPI (e.g. Kerberos) authentication.",
	"gssapidelegatecredentials":   "Whether to delegate GSSAPI credentials to the remote host.",
	"gssapikeyexchange":           "Whether to perform key exchange via GSSAPI.",
	"numberofpasswordprompts":     "Maximum number of password authentication attempts. Default is 3.",
	"addkeystoagent":              "Whether to automatically add authenticated private keys to ssh-agent.",
	"preferredauthentications":    "Comma-separated list of authentication methods in preference order.",

	// --- Proxy / Jump ---
	"proxyjump":      "Jump host (bastion) for the connection. Multiple hops can be specified as a comma-separated list.",
	"proxycommand":   "Command to use as a proxy before establishing the SSH connection. %h and %p expand to host and port.",
	"proxyusefdpass": "Whether to pass file descriptors via ProxyCommand.",

	// --- Security / Verification ---
	"stricthostkeychecking":       "Host key verification behavior: ask, yes, no, or accept-new.",
	"userknownhostsfile":          "Path to the user's known_hosts file.",
	"globalknownhostsfile":        "Path to the system-wide known_hosts file.",
	"checkhostip":                 "Whether to verify the host's IP address in known_hosts in addition to the hostname.",
	"verifyhostkeydns":            "Whether to verify the host key using DNS SSHFP records.",
	"hashknownhosts":              "If yes, hostnames in known_hosts are stored as hashes.",
	"updatehostkeys":              "Whether to append new host keys presented by the server to known_hosts.",
	"fingerprinthash":             "Hash algorithm used to display host key fingerprints. Default is SHA256.",
	"hostkeyalias":                "Alias name used when recording the host key in known_hosts.",
	"enablesshkeysign":            "Whether to allow ssh-keysign for HostbasedAuthentication.",
	"hostbasedacceptedalgorithms": "List of key algorithms accepted for host-based authentication.",

	// --- Forwarding / Tunneling ---
	"forwardagent":         "Whether to enable ssh-agent forwarding, allowing the remote side to use local keys.",
	"forwardx11":           "Whether to enable X11 forwarding.",
	"forwardx11trusted":    "If yes, X11 forwarding runs in trusted mode (reduces security).",
	"forwardx11timeout":    "Expiry time (seconds) for X11 forwarding credentials.",
	"gatewayports":         "Whether remote forwarded ports listen on all interfaces of the remote host.",
	"tunnel":               "Whether to use VPN tunnel device forwarding. Default is no.",
	"tunneldevice":         "Tunnel device specification (local:remote).",
	"permitlocalcommand":   "Whether to allow execution of the LocalCommand directive.",
	"permitremoteopen":     "Restricts the host:port targets allowed for RemoteForward.",
	"clearallforwardings":  "If yes, cancels all forwarding settings defined elsewhere.",
	"exitonforwardfailure": "Whether to disconnect if port forwarding fails.",

	// --- Keep-alive / Timeouts ---
	"serveraliveinterval": "Interval (seconds) between keep-alive packets when the server is unresponsive. 0 disables.",
	"serveralivecountmax": "Maximum number of unanswered keep-alive packets before disconnecting. Default is 3.",
	"connecttimeout":      "TCP connection timeout (seconds). Defaults to the system TCP timeout.",
	"connectionattempts":  "Number of connection retries on failure. Default is 1.",
	"tcpkeepalive":        "Whether to send TCP-level keep-alive packets. Default is yes.",
	"controlmaster":       "Whether to enable connection multiplexing (ControlMaster): auto, yes, no, etc.",
	"controlpersist":      "How long to keep a ControlMaster connection alive in the background. yes or seconds.",
	"controlpath":         "Path to the socket file for ControlMaster connections. Expands %h, %p, %r, etc.",

	// --- Ciphers / Algorithms ---
	"ciphers":                 "Ordered list of symmetric cipher algorithms to use.",
	"macs":                    "Ordered list of MAC (message authentication) algorithms to use.",
	"kexalgorithms":           "Ordered list of key exchange algorithms to use.",
	"hostkeyalgorithms":       "List of host key algorithms accepted from the server.",
	"pubkeyacceptedalgorithms": "List of key algorithms used for public-key authentication.",
	"casignaturealgorithms":   "List of algorithms used for CA signatures.",
	"requiredrsasize":         "Minimum RSA key size in bits. Keys shorter than this are rejected.",
	"gssapikexalgorithms":     "List of algorithms used for GSSAPI key exchange.",

	// --- Terminal / Session ---
	"requesttty":              "PTY allocation policy: auto, yes, no, or force.",
	"sessiontype":             "Session type: default, none, or subsystem.",
	"stdinnull":               "If yes, redirect stdin from /dev/null.",
	"escapechar":              "Escape sequence prefix character. Default is ~.",
	"enableescapecommandline": "Whether to enable command-line input via the escape character.",
	"loglevel":                "Logging verbosity: QUIET, FATAL, ERROR, INFO, VERBOSE, DEBUG, etc.",
	"logverbose":              "Enable detailed logging for specific modules or functions.",
	"syslogfacility":          "Syslog facility name for logging. Default is USER.",
	"batchmode":               "If yes, disable interactive prompts (passwords, etc.). Useful for scripts.",
	"forkafterauthentication": "Whether to fork into the background after authentication.",
	"localcommand":            "Local command to execute after the connection is established (requires PermitLocalCommand yes).",

	// --- Environment / Misc ---
	"sendenv":             "Pattern of environment variable names to forward to the remote server.",
	"setenv":              "Environment variables to set in the remote session, in KEY=VALUE format.",
	"xauthlocation":       "Absolute path to the xauth command, used for X11 forwarding.",
	"ipqos":               "IPv4 QoS (DiffServ) class. Can be specified separately for interactive and non-interactive sessions.",
	"rekeylimit":          "Maximum data volume and time before a rekey is performed.",
	"compression":         "Whether to enable data compression. Useful on slow links.",
	"canonicalizehostname":      "Whether to canonicalize the hostname via DNS before connecting.",
	"canonicaldomains":          "List of search domains appended during hostname canonicalization.",
	"canonicalizefallbacklocal": "Whether to fall back to the unqualified hostname if canonicalization fails.",
	"canonicalizemaxdots":       "Maximum number of dots in a hostname for canonicalization to apply. Default is 1.",
	"streamlocalbindmask":       "File permission mask for Unix domain socket forwarding.",
	"streamlocalbindunlink":     "Whether to remove an existing socket before creating a new one for forwarding.",
	"securitykeyprovider":       "Path to the provider library for FIDO/U2F security keys.",
	"nohostauthenticationforlocalhost": "Whether to skip host authentication for loopback addresses.",
	"channeltimeout":        "Timeout before automatically closing an inactive channel.",
	"obscurekeystroketiming": "Whether to obfuscate keystroke timing information.",
	"gssapitrustdns":        "Whether to trust DNS for GSSAPI authentication.",
	"gssapirenewalforcesrekey": "Whether to force a rekey when GSSAPI credentials are renewed.",
	"tag": "ffh-specific directive. Groups hosts for tab filtering in the ffh UI.",
}
