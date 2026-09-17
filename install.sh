#!/usr/bin/env bash
# Bootstrap K-CRM: trouve ou installe Go, construit le binaire, lance le wizard.
# L'install produit reste le wizard dans le navigateur. Ce script n'est pas un .deb.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

GO_MIN_MAJOR=1
GO_MIN_MINOR=26
GO_TARBALL_VER="1.26.5"
BIN="$ROOT/k-crm"
NEED_TTY=1

say() { printf '%s\n' "$*"; }
err() { printf 'Erreur: %s\n' "$*" >&2; }
pause() {
  if [[ -t 0 ]]; then
    read -r -p "Entree pour continuer. " _
  fi
}

need_tty() {
  if [[ ! -t 0 || ! -t 1 ]]; then
    err "lance ce script dans un terminal (pas en pipe)."
    exit 1
  fi
}

ask() {
  local prompt="$1" default="${2-}"
  local reply
  if [[ -n "$default" ]]; then
    read -r -p "$prompt [$default] " reply
    printf '%s' "${reply:-$default}"
  else
    read -r -p "$prompt " reply
    printf '%s' "$reply"
  fi
}

yesno() {
  local prompt="$1" default="${2:-n}" reply
  read -r -p "$prompt " reply
  reply="${reply:-$default}"
  case "$reply" in
    o|O|oui|Oui|y|Y|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
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
  say "Telechargement de Go ${GO_TARBALL_VER} (officiel, sans snap, sans root)..."
  say "$url"
  if ! command -v curl >/dev/null 2>&1; then
    err "curl est requis pour installer Go."
    exit 1
  fi
  curl -fL --retry 3 --retry-delay 1 -o "$tmp/go.tgz" "$url"
  rm -rf "$dest"
  mkdir -p "$HOME/.local/share"
  tar -C "$tmp" -xzf "$tmp/go.tgz"
  mv "$tmp/go" "$dest"
  rm -rf "$tmp"
  if [[ ! -x "$dest/bin/go" ]] || ! go_ver_ok "$dest/bin/go"; then
    err "Go installe mais inutilisable dans $dest"
    exit 1
  fi
  say "Go installe dans $dest"
  printf '%s' "$dest/bin/go"
}

list_ips() {
  local ip
  if command -v hostname >/dev/null 2>&1; then
    for ip in $(hostname -I 2>/dev/null || true); do
      case "$ip" in
        0.0.0.0|::|::1) continue ;;
        *:*) continue ;;
        *) printf '%s\n' "$ip" ;;
      esac
    done
  fi
}

# 0 = free, 1 = taken, 2 = IP not on this machine
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

need_tty

say "K-CRM — bootstrap"
say "Ce script construit le binaire. L'install, c'est le wizard dans le navigateur."
say "Depot: $ROOT"
pause

if [[ ! -f "$ROOT/go.mod" || ! -d "$ROOT/cmd/k-crm" ]]; then
  err "ce dossier n'est pas le depot K-CRM (go.mod / cmd/k-crm manquants)."
  exit 1
fi

say ""
say "1/4  Go"
GO_BIN=""
if GO_BIN="$(find_go)"; then
  say "Trouve: $GO_BIN ($("$GO_BIN" version))"
else
  say "Go ${GO_MIN_MAJOR}.${GO_MIN_MINOR}+ introuvable dans le PATH."
  say "Pas de snap, pas de paquet Ubuntu: telechargement officiel dans ~/.local/share."
  if yesno "Installer Go ${GO_TARBALL_VER} maintenant ? [O/n]" o; then
    GO_BIN="$(install_go_tarball)"
    say "OK: $GO_BIN ($("$GO_BIN" version))"
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
if ! grep -q 'go1.26\|/.local/share/go' "$HOME/.profile" 2>/dev/null; then
  if yesno "Ajouter Go au PATH dans ~/.profile pour les prochains terminaux ? [o/N]" n; then
    {
      printf '\n# K-CRM Go\n'
      printf 'export PATH="%s:$PATH"\n' "$(dirname "$GO_BIN")"
    } >> "$HOME/.profile"
    say "Ajoute dans ~/.profile. Ouvre un nouveau terminal, ou: source ~/.profile"
  fi
fi
pause

say ""
say "2/4  Construction"
export CGO_ENABLED=0
say "go build -o k-crm ./cmd/k-crm"
"$GO_BIN" build -o "$BIN" ./cmd/k-crm
chmod 755 "$BIN"
say "Binaire: $BIN"
pause

say ""
say "3/4  Adresse d'ecoute (IP explicite, jamais 0.0.0.0)"
mapfile -t IPS < <(list_ips | awk 'NF && !seen[$0]++')
if ((${#IPS[@]})); then
  say "Adresses vues sur cette machine:"
  local_i=1
  for ip in "${IPS[@]}"; do
    extra=""
    [[ "$ip" == 127.* ]] && extra=" (cette machine seulement)"
    say "  $local_i) $ip$extra"
    local_i=$((local_i + 1))
  done
else
  say "Aucune IP non-loopback vue. 127.0.0.1 ne sera visible que sur cette machine."
  IPS=(127.0.0.1)
fi
DEFAULT_IP="${IPS[0]}"
HOST="$(ask "IP a binder" "$DEFAULT_IP")"
if refuse_wildcard "$HOST"; then
  err "refus de binder $HOST — passe une IP explicite."
  exit 1
fi
PROBE_RC=0
probe_port "$HOST" 8740 || PROBE_RC=$?
if [[ "$PROBE_RC" -eq 2 ]]; then
  err "$HOST n'est pas une adresse de cette machine. Reprends avec une IP de la liste."
  exit 1
fi
if ! DEFAULT_PORT="$(find_free_port "$HOST")"; then
  err "aucun port libre entre 8740 et 8899 sur $HOST."
  exit 1
fi
say "Port libre propose: $DEFAULT_PORT"
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
say "Listen: $LISTEN"
pause

say ""
say "4/4  Donnees (carnet, jeton, sessions). Pas le carnet d'une autre install."
DATA="$(ask "Dossier data" "$ROOT/data")"
mkdir -p "$DATA"
chmod 700 "$DATA" 2>/dev/null || true
if [[ -f "$DATA/config.json" ]]; then
  say "Attention: $DATA a deja une install (config.json). Le wizard ne se relancera pas."
  if ! yesno "Continuer quand meme ? [o/N]" n; then
    exit 1
  fi
fi
say "Data: $DATA"
say ""
say "Le wizard s'ouvre sur: http://${LISTEN}/"
say "Identifiant / mot de passe / 2FA se saisissent dans le navigateur, pas ici."
pause

exec "$BIN" serve -listen "$LISTEN" -data "$DATA"
