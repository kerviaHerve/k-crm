#!/usr/bin/env bash
# Bootstrap K-CRM: trouve ou installe Go, construit le binaire, pose un
# service systemd utilisateur, puis le wizard navigateur prend le relais.
# L'install produit reste le wizard. Ce script n'est pas un .deb.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

GO_MIN_MAJOR=1
GO_MIN_MINOR=26
GO_TARBALL_VER="1.26.5"
BIN="$ROOT/k-crm"
DRY_RUN=0
LISTEN_FLAG=""
DATA_FLAG=""
PLAIN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1; shift ;;
    --plain) PLAIN=1; shift ;;
    --listen)
      LISTEN_FLAG="${2-}"
      shift 2
      ;;
    --data)
      DATA_FLAG="${2-}"
      shift 2
      ;;
    -h|--help)
      printf '%s\n' "Usage: ./install.sh [--listen IP:PORT] [--data DIR] [--dry-run] [--plain]"
      exit 0
      ;;
    *)
      printf 'Erreur: option inconnue %s\n' "$1" >&2
      exit 1
      ;;
  esac
done

USE_TUI=0
if [[ "$PLAIN" -eq 0 && -t 0 && -t 1 && "${TERM:-dumb}" != "dumb" ]]; then
  USE_TUI=1
fi

if [[ "$USE_TUI" -eq 1 && -z "${NO_COLOR:-}" ]]; then
  C_COPPER=$'\033[38;2;216;106;50m'
  C_MUTE=$'\033[38;2;138;128;118m'
  C_FG=$'\033[38;2;244;239;232m'
  C_OK=$'\033[38;2;122;158;90m'
  C_ERR=$'\033[38;2;196;70;50m'
  C_BOLD=$'\033[1m'
  C_RESET=$'\033[0m'
  C_HIDE=$'\033[?25l'
  C_SHOW=$'\033[?25h'
else
  C_COPPER="" C_MUTE="" C_FG="" C_OK="" C_ERR="" C_BOLD="" C_RESET=""
  C_HIDE="" C_SHOW=""
fi

tui_cleanup() {
  printf '%s' "$C_SHOW"
}
trap tui_cleanup EXIT INT TERM

say() { printf '%s\n' "$*"; }
err() { printf '%sErreur:%s %s\n' "$C_ERR" "$C_RESET" "$*" >&2; }

need_tty() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    return 0
  fi
  if [[ -n "$LISTEN_FLAG" && -n "$DATA_FLAG" ]]; then
    return 0
  fi
  if [[ ! -t 0 || ! -t 1 ]]; then
    err "lance ce script dans un terminal (pas en pipe)."
    exit 1
  fi
}

tui_clear() {
  if [[ "$USE_TUI" -eq 1 ]]; then
    printf '\033[H\033[2J%s' "$C_HIDE"
  fi
}

tui_rule() {
  local n="${1:-56}"
  local i s=""
  for ((i = 0; i < n; i++)); do
    s+="─"
  done
  printf '  %s%s%s\n' "$C_MUTE" "$s" "$C_RESET"
}

tui_header() {
  [[ "$USE_TUI" -eq 1 ]] || return 0
  local step="$1" title="$2"
  tui_clear
  printf '\n  %s%sK-CRM%s  %s0.1.0-beta%s\n' "$C_COPPER" "$C_BOLD" "$C_RESET" "$C_MUTE" "$C_RESET"
  printf '  %sLe wizard navigateur fait l'\''install. Ici: Go, binaire, écoute, service.%s\n\n' "$C_MUTE" "$C_RESET"
  tui_rail "$step"
  tui_rule 56
  printf '\n  %s%s%s\n\n' "$C_BOLD$C_FG" "$title" "$C_RESET"
}

tui_rail() {
  local cur="$1"
  local names=("Go" "Binaire" "Écoute" "Données" "Service")
  local i label
  printf '  '
  for i in 1 2 3 4 5; do
    label="${names[$((i - 1))]}"
    if (( i == cur )); then
      printf '%s%s%d %s%s' "$C_COPPER$C_BOLD" "" "$i" "$label" "$C_RESET"
    elif (( i < cur )); then
      printf '%s%d %s%s' "$C_OK" "$i" "$label" "$C_RESET"
    else
      printf '%s%d %s%s' "$C_MUTE" "$i" "$label" "$C_RESET"
    fi
    if (( i < 5 )); then
      printf '%s  ·  %s' "$C_MUTE" "$C_RESET"
    fi
  done
  printf '\n\n'
}

tui_note() {
  if [[ "$USE_TUI" -eq 0 ]]; then
    say "$*"
    return 0
  fi
  printf '  %s%s%s\n' "$C_MUTE" "$*" "$C_RESET"
}

tui_ok() {
  if [[ "$USE_TUI" -eq 0 ]]; then
    say "$*"
    return 0
  fi
  printf '  %s●%s  %s\n' "$C_OK" "$C_RESET" "$*"
}

tui_key() {
  local k
  IFS= read -rsn1 k || return 1
  if [[ "$k" == $'\x1b' ]]; then
    local r=""
    IFS= read -rsn2 r || true
    case "$r" in
      '[A') printf 'UP' ;;
      '[B') printf 'DOWN' ;;
      '[C') printf 'RIGHT' ;;
      '[D') printf 'LEFT' ;;
      *) printf 'ESC' ;;
    esac
    return 0
  fi
  if [[ -z "$k" || "$k" == $'\n' || "$k" == $'\r' ]]; then
    printf 'ENTER'
    return 0
  fi
  printf '%s' "$k"
}

ask() {
  local prompt="$1" default="${2-}" reply
  if [[ "$USE_TUI" -eq 1 ]]; then
    printf '  %s%s%s' "$C_FG" "$prompt" "$C_RESET" >&2
    if [[ -n "$default" ]]; then
      printf '  %s[%s]%s ' "$C_MUTE" "$default" "$C_RESET" >&2
    else
      printf ' ' >&2
    fi
    printf '%s' "$C_SHOW" >&2
    read -r reply
    printf '%s' "$C_HIDE" >&2
    printf '%s' "${reply:-$default}"
    return
  fi
  if [[ -n "$default" ]]; then
    read -r -p "$prompt [$default] " reply
    printf '%s' "${reply:-$default}"
  else
    read -r -p "$prompt " reply
    printf '%s' "$reply"
  fi
}

yesno() {
  local prompt="$1" default="${2:-n}" reply sel
  if [[ "$USE_TUI" -eq 1 ]]; then
    if [[ "$default" == "o" || "$default" == "O" ]]; then sel=0; else sel=1; fi
    while true; do
      printf '\r  %s%s%s  ' "$C_FG" "$prompt" "$C_RESET"
      if (( sel == 0 )); then
        printf '%s[ Oui ]%s  %sNon%s   ' "$C_COPPER$C_BOLD" "$C_RESET" "$C_MUTE" "$C_RESET"
      else
        printf '%sOui%s  %s[ Non ]%s   ' "$C_MUTE" "$C_RESET" "$C_COPPER$C_BOLD" "$C_RESET"
      fi
      printf '%s' "$C_SHOW"
      reply="$(tui_key || true)"
      printf '%s' "$C_HIDE"
      case "$reply" in
        LEFT|RIGHT) sel=$((1 - sel)) ;;
        ENTER) printf '\n'; (( sel == 0 )) && return 0 || return 1 ;;
        o|O|y|Y) printf '\n'; return 0 ;;
        n|N) printf '\n'; return 1 ;;
      esac
    done
  fi
  read -r -p "$prompt " reply
  reply="${reply:-$default}"
  case "$reply" in
    o|O|oui|Oui|y|Y|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}

pause() {
  if [[ "$DRY_RUN" -eq 1 ]]; then
    return 0
  fi
  if [[ "$USE_TUI" -eq 1 ]]; then
    printf '\n  %sEntrée pour continuer%s' "$C_MUTE" "$C_RESET"
    printf '%s' "$C_SHOW"
    read -r _
    printf '%s' "$C_HIDE"
    return 0
  fi
  if [[ -t 0 ]]; then
    read -r -p "Entree pour continuer. " _
  fi
}

go_ver_ok() {
  local bin="$1" line major minor
  line="$("$bin" version 2>/dev/null || true)"
  [[ "$line" =~ go([0-9]+)\.([0-9]+) ]] || return 1
  major="${BASH_REMATCH[1]}"
  minor="${BASH_REMATCH[2]}"
  if (( major > GO_MIN_MAJOR )); then return 0; fi
  if (( major == GO_MIN_MAJOR && minor >= GO_MIN_MINOR )); then return 0; fi
  return 1
}

find_go() {
  local cand
  if command -v go >/dev/null 2>&1; then
    cand="$(command -v go)"
    if go_ver_ok "$cand"; then
      printf '%s' "$cand"
      return 0
    fi
  fi
  for cand in \
    "$HOME/.local/share/go${GO_TARBALL_VER}/bin/go" \
    "$HOME/.local/share/go1.26.5/bin/go" \
    "$HOME/.local/share/go/bin/go" \
    "$HOME/.local/go/bin/go" \
    /usr/local/go/bin/go
  do
    if [[ -x "$cand" ]] && go_ver_ok "$cand"; then
      printf '%s' "$cand"
      return 0
    fi
  done
  return 1
}

install_go_tarball() {
  local dest="$HOME/.local/share/go${GO_TARBALL_VER}"
  local tmp url
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/kcrm-go.XXXXXX")"
  url="https://go.dev/dl/go${GO_TARBALL_VER}.linux-amd64.tar.gz"
  tui_note "Téléchargement officiel de Go ${GO_TARBALL_VER} (pas de snap, pas de root)."
  tui_note "$url"
  if ! command -v curl >/dev/null 2>&1; then
    err "curl est requis pour installer Go."
    exit 1
  fi
  printf '%s' "$C_SHOW"
  curl -fL --retry 3 --retry-delay 1 -o "$tmp/go.tgz" "$url"
  printf '%s' "$C_HIDE"
  rm -rf "$dest"
  mkdir -p "$HOME/.local/share"
  tar -C "$tmp" -xzf "$tmp/go.tgz"
  mv "$tmp/go" "$dest"
  rm -rf "$tmp"
  if [[ ! -x "$dest/bin/go" ]] || ! go_ver_ok "$dest/bin/go"; then
    err "Go installé mais inutilisable dans $dest"
    exit 1
  fi
  printf '%s' "$dest/bin/go"
}

list_ips() {
  python3 - <<'PY'
import subprocess, ipaddress
raw = subprocess.check_output(["hostname", "-I"], text=True, stderr=subprocess.DEVNULL)
seen = []
for tok in raw.split():
    try:
        ip = ipaddress.ip_address(tok)
    except ValueError:
        continue
    if ip.version != 4 or ip.is_unspecified or ip.is_loopback or ip.is_multicast:
        continue
    s = str(ip)
    if s not in seen:
        seen.append(s)

def kind(s):
    ip = ipaddress.ip_address(s)
    if ip in ipaddress.ip_network("100.64.0.0/10"):
        return 0, "overlay"
    if ip in ipaddress.ip_network("10.0.0.0/8") or ip in ipaddress.ip_network("192.168.0.0/16"):
        return 1, "lan"
    if ip in ipaddress.ip_network("172.16.0.0/12"):
        return 3, "docker"
    return 2, "public"

ranked = sorted(seen, key=lambda s: (kind(s)[0], s))
primary = [s for s in ranked if kind(s)[1] != "docker"]
show = primary or ranked
for s in show:
    print(s, kind(s)[1])
PY
}

probe_port() {
  python3 - "$1" "$2" <<'PY'
import errno, socket, sys
host, port = sys.argv[1], int(sys.argv[2])
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
try:
    s.bind((host, port))
except OSError as e:
    sys.exit(2 if e.errno == errno.EADDRNOTAVAIL else 1)
finally:
    s.close()
sys.exit(0)
PY
}

port_taken() {
  local rc=0
  probe_port "$1" "$2" || rc=$?
  [[ "$rc" -eq 1 ]]
}

find_free_port() {
  local host="$1" p
  for p in $(seq 8740 8899); do
    if probe_port "$host" "$p"; then
      printf '%s' "$p"
      return 0
    fi
  done
  return 1
}

refuse_wildcard() {
  local host="$1"
  case "$host" in
    0.0.0.0|\*|::|"") return 0 ;;
    *) return 1 ;;
  esac
}

UNIT_PATH="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user/k-crm.service"

systemd_user_ok() {
  command -v systemctl >/dev/null 2>&1 || return 1
  [[ -S "${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/systemd/private" ]]
}

unit_body() {
  cat <<EOF
[Unit]
Description=K-CRM
After=network-online.target

[Service]
Type=simple
WorkingDirectory=$ROOT
ExecStart=$BIN serve -listen $LISTEN -data $DATA
Environment=KCRM_SYSTEMD=1
Restart=always
RestartSec=2
UMask=077

[Install]
WantedBy=default.target
EOF
}

try_linger() {
  local linger
  command -v loginctl >/dev/null 2>&1 || return 0
  linger="$(loginctl show-user "$USER" -p Linger --value 2>/dev/null || true)"
  if [[ "$linger" == "yes" ]]; then
    return 0
  fi
  if loginctl enable-linger "$USER" >/dev/null 2>&1; then
    tui_ok "Linger utilisateur activé (survit à la déconnexion)"
    return 0
  fi
  tui_note "Sans linger, le service s'arrête à la déconnexion. En root: loginctl enable-linger $USER"
}

kind_label() {
  case "$1" in
    overlay) printf 'overlay, recommandée' ;;
    lan) printf 'réseau local' ;;
    public) printf 'publique, visible depuis Internet' ;;
    loopback) printf 'cette machine seulement' ;;
    *) printf '%s' "$1" ;;
  esac
}

draw_ip_list() {
  local sel="$1" i=0 ip kind mark
  for ip in "${IPS[@]}"; do
    kind="${KINDS[$i]}"
    if (( i == sel )); then
      printf '    %s›%s  %s%-2d%s  %s%-15s%s  %s\n' \
        "$C_COPPER" "$C_RESET" "$C_COPPER$C_BOLD" "$((i + 1))" "$C_RESET" \
        "$C_FG" "$ip" "$C_RESET" "$(kind_label "$kind")"
    else
      printf '       %s%-2d%s  %s%-15s%s  %s%s%s\n' \
        "$C_MUTE" "$((i + 1))" "$C_RESET" \
        "$C_FG" "$ip" "$C_RESET" "$C_MUTE" "$(kind_label "$kind")" "$C_RESET"
    fi
    i=$((i + 1))
  done
}

choose_listen() {
  local i reply HOST_TRY sel=0
  mapfile -t IP_ROWS < <(list_ips)
  IPS=()
  KINDS=()
  for row in "${IP_ROWS[@]:-}"; do
    [[ -n "$row" ]] || continue
    IPS+=("${row%% *}")
    KINDS+=("${row#* }")
  done
  if ((${#IPS[@]} == 0)); then
    IPS=(127.0.0.1)
    KINDS=(loopback)
  fi

  if [[ "$USE_TUI" -eq 1 ]]; then
    while true; do
      tui_header 3 "Adresse d'écoute"
      tui_note "IP explicite, jamais 0.0.0.0. Les ponts Docker sont masqués."
      printf '\n'
      draw_ip_list "$sel"
      printf '\n  %s↑↓ choisir · entrée valider · ou un numéro%s\n' "$C_MUTE" "$C_RESET"
      reply="$(tui_key || true)"
      case "$reply" in
        UP) (( sel > 0 )) && sel=$((sel - 1)) ;;
        DOWN) (( sel < ${#IPS[@]} - 1 )) && sel=$((sel + 1)) ;;
        ENTER)
          HOST="${IPS[$sel]}"
          break
          ;;
        [1-9])
          i=$((reply))
          if (( i >= 1 && i <= ${#IPS[@]} )); then
            HOST="${IPS[$((i - 1))]}"
            break
          fi
          ;;
        i|I)
          tui_header 3 "Adresse d'écoute"
          HOST="$(ask "IP à binder" "${IPS[$sel]}")"
          break
          ;;
      esac
    done
  else
    say "Adresses utiles (les ponts Docker sont masques):"
    i=1
    for ip in "${IPS[@]}"; do
      say "  $i) $ip ($(kind_label "${KINDS[$((i - 1))]}"))"
      i=$((i + 1))
    done
    reply="$(ask "IP a binder (numero ou adresse)" "${IPS[0]}")"
    if [[ "$reply" =~ ^[0-9]+$ ]]; then
      i=$((reply))
      if (( i < 1 || i > ${#IPS[@]} )); then
        err "le numero $reply n'est pas dans la liste (1-${#IPS[@]})."
        exit 1
      fi
      HOST="${IPS[$((i - 1))]}"
    else
      HOST="$reply"
    fi
  fi

  if refuse_wildcard "$HOST"; then
    err "refus de binder $HOST — passe une IP explicite."
    exit 1
  fi
  PROBE_RC=0
  probe_port "$HOST" 8740 || PROBE_RC=$?
  if [[ "$PROBE_RC" -eq 2 ]]; then
    err "$HOST n'est pas une adresse de cette machine. Reprends avec un numéro de la liste."
    exit 1
  fi
  if ! DEFAULT_PORT="$(find_free_port "$HOST")"; then
    err "aucun port libre entre 8740 et 8899 sur $HOST."
    exit 1
  fi
  if [[ "$USE_TUI" -eq 1 ]]; then
    tui_header 3 "Adresse d'écoute"
    tui_ok "$HOST"
    tui_note "Port libre proposé: $DEFAULT_PORT"
    printf '\n'
  else
    say "Port libre propose: $DEFAULT_PORT"
  fi
  PORT="$(ask "Port" "$DEFAULT_PORT")"
  if [[ ! "$PORT" =~ ^[0-9]+$ ]] || (( PORT < 1 || PORT > 65535 )); then
    err "port invalide: $PORT"
    exit 1
  fi
  LISTEN="${HOST}:${PORT}"
  if port_taken "$HOST" "$PORT"; then
    err "$LISTEN est pris. Le script proposait $DEFAULT_PORT."
    exit 1
  fi
}

need_tty

if [[ "$USE_TUI" -eq 0 ]]; then
  say "K-CRM 0.1.0-beta"
  say "Ce script construit le binaire et pose un service utilisateur. L'install, c'est le wizard dans le navigateur."
  say "Depot: $ROOT"
  pause
fi

if [[ ! -f "$ROOT/go.mod" || ! -d "$ROOT/cmd/k-crm" ]]; then
  err "ce dossier n'est pas le depot K-CRM (go.mod / cmd/k-crm manquants)."
  exit 1
fi

tui_header 1 "Go 1.26+"
if [[ "$USE_TUI" -eq 0 ]]; then
  say ""
  say "1/5  Go"
fi
GO_BIN=""
if GO_BIN="$(find_go)"; then
  tui_ok "$GO_BIN"
  tui_note "$("$GO_BIN" version)"
else
  tui_note "Go ${GO_MIN_MAJOR}.${GO_MIN_MINOR}+ introuvable dans le PATH."
  tui_note "Pas de snap, pas de paquet Ubuntu: téléchargement officiel dans ~/.local/share."
  printf '\n'
  if yesno "Installer Go ${GO_TARBALL_VER} maintenant ?" o; then
    GO_BIN="$(install_go_tarball)"
    tui_ok "$GO_BIN"
    tui_note "$("$GO_BIN" version)"
  else
    err "sans Go, impossible de construire. Relance le script quand tu es pret."
    exit 1
  fi
fi
export PATH="$(dirname "$GO_BIN"):$PATH"
if ! command -v go >/dev/null 2>&1 || ! go_ver_ok "$(command -v go)"; then
  err "Go n'est toujours pas utilisable."
  exit 1
fi
if [[ "$DRY_RUN" -eq 0 ]] && ! grep -q 'go1.26\|/.local/share/go' "$HOME/.profile" 2>/dev/null; then
  printf '\n'
  if yesno "Ajouter Go au PATH dans ~/.profile ?" n; then
    {
      printf '\n# K-CRM Go\n'
      printf 'export PATH="%s:$PATH"\n' "$(dirname "$GO_BIN")"
    } >> "$HOME/.profile"
    tui_ok "Ajouté dans ~/.profile"
  fi
fi
if [[ "$USE_TUI" -eq 0 ]]; then pause; fi

tui_header 2 "Construction du binaire"
if [[ "$USE_TUI" -eq 0 ]]; then
  say ""
  say "2/5  Construction"
fi
export CGO_ENABLED=0
tui_note "go build -o k-crm ./cmd/k-crm"
printf '%s' "$C_SHOW"
"$GO_BIN" build -o "$BIN" ./cmd/k-crm
printf '%s' "$C_HIDE"
chmod 755 "$BIN"
tui_ok "$BIN"
if [[ "$USE_TUI" -eq 0 ]]; then pause; fi

if [[ -n "$LISTEN_FLAG" ]]; then
  tui_header 3 "Adresse d'écoute"
  LISTEN="$LISTEN_FLAG"
  HOST="${LISTEN%:*}"
  PORT="${LISTEN##*:}"
  if refuse_wildcard "$HOST"; then
    err "refus de binder $HOST"
    exit 1
  fi
  tui_ok "$LISTEN"
else
  if [[ "$USE_TUI" -eq 0 ]]; then
    say ""
    say "3/5  Adresse d'ecoute (IP explicite, jamais 0.0.0.0)"
  fi
  choose_listen
fi
tui_ok "Listen $LISTEN"
if [[ "$USE_TUI" -eq 0 ]]; then
  say "Listen: $LISTEN"
  pause
fi

tui_header 4 "Dossier data"
if [[ "$USE_TUI" -eq 0 ]]; then
  say ""
  say "4/5  Donnees (carnet, jeton, sessions). Pas le carnet d'une autre install."
fi
tui_note "Carnet, jeton, sessions. Pas le data d'une autre install."
printf '\n'
if [[ -n "$DATA_FLAG" ]]; then
  DATA="$DATA_FLAG"
else
  DATA="$(ask "Dossier data" "$ROOT/data")"
fi
mkdir -p "$DATA"
chmod 700 "$DATA" 2>/dev/null || true
if [[ -f "$DATA/config.json" ]]; then
  printf '\n'
  tui_note "Attention: $DATA a déjà une install (config.json). Le wizard ne se relancera pas."
  if [[ "$DRY_RUN" -eq 0 ]]; then
    if ! yesno "Continuer quand même ?" n; then
      exit 1
    fi
  fi
fi
tui_ok "$DATA"

case "$ROOT$BIN$LISTEN$DATA" in
  *$'\n'*|*$'\r'*) err "chemin ou listen invalide" ; exit 1 ;;
esac

USE_SYSTEMD=0
if systemd_user_ok; then
  USE_SYSTEMD=1
fi

tui_header 5 "Service"
if [[ "$USE_TUI" -eq 0 ]]; then
  say ""
  say "5/5  Service"
fi
if [[ "$USE_SYSTEMD" -eq 1 ]]; then
  tui_ok "systemd utilisateur"
  tui_note "$UNIT_PATH"
  tui_note "Restart=always. Tu pourras fermer ce terminal."
else
  tui_note "systemd utilisateur indisponible. Lancement dans ce terminal: il s'arrête si tu le fermes."
fi

tui_header 5 "Prêt"
tui_ok "Binaire  $BIN"
tui_ok "Écoute   $LISTEN"
tui_ok "Data     $DATA"
if [[ "$USE_SYSTEMD" -eq 1 ]]; then
  tui_ok "Service  k-crm.service"
fi
printf '\n  %sWizard%s  http://%s/\n' "$C_COPPER$C_BOLD" "$C_RESET" "$LISTEN"
printf '\n  %sIdentifiant, mot de passe et 2FA se saisissent dans le navigateur.%s\n' "$C_MUTE" "$C_RESET"
printf '\n  %sMCP%s\n' "$C_BOLD$C_FG" "$C_RESET"
printf '  hermes mcp add kcrm --command %s --connect-timeout 15 --args mcp -data %s\n' "$BIN" "$DATA"
printf '  %s--args en dernier. Détail dans README.md%s\n' "$C_MUTE" "$C_RESET"

if [[ "$DRY_RUN" -eq 1 ]]; then
  printf '\n  %sDry-run: pas de lancement, unit non écrit.%s\n' "$C_MUTE" "$C_RESET"
  if [[ "$USE_SYSTEMD" -eq 1 ]]; then
    say ""
    say "---- $UNIT_PATH ----"
    unit_body
    say "----"
    say "systemctl --user daemon-reload && systemctl --user enable k-crm.service && systemctl --user restart k-crm.service"
  else
    say "$BIN serve -listen $LISTEN -data $DATA"
  fi
  exit 0
fi

printf '\n'
if [[ "$USE_TUI" -eq 1 ]]; then
  if [[ "$USE_SYSTEMD" -eq 1 ]]; then
    printf '  %sEntrée pour activer le service%s' "$C_MUTE" "$C_RESET"
  else
    printf '  %sEntrée pour lancer le serveur%s' "$C_MUTE" "$C_RESET"
  fi
  printf '%s' "$C_SHOW"
  read -r _
else
  pause
fi

if [[ "$USE_SYSTEMD" -eq 1 ]]; then
  mkdir -p "$(dirname "$UNIT_PATH")"
  unit_body > "$UNIT_PATH"
  systemctl --user daemon-reload
  systemctl --user enable k-crm.service
  systemctl --user restart k-crm.service
  if ! systemctl --user is-active --quiet k-crm.service; then
    err "k-crm.service n'est pas actif"
    systemctl --user status k-crm.service --no-pager -l >&2 || true
    exit 1
  fi
  ok_health=0
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    if command -v curl >/dev/null 2>&1 && curl -fsS --connect-timeout 1 "http://$LISTEN/healthz" >/dev/null 2>&1; then
      ok_health=1
      break
    fi
    sleep 0.3
  done
  try_linger
  tui_ok "Service actif"
  if [[ "$ok_health" -eq 1 ]]; then
    tui_ok "http://$LISTEN/  (healthz 200)"
  else
    tui_note "Service lancé. Ouvre http://$LISTEN/ pour le wizard."
  fi
  printf '%s' "$C_SHOW"
  exit 0
fi

printf '%s' "$C_SHOW"
exec "$BIN" serve -listen "$LISTEN" -data "$DATA"
