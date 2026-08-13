#!/usr/bin/env bash
#
# install.sh — Instala o Image Reduce no Linux.
#
# Uso (cole no terminal):
#   curl -fsSL https://raw.githubusercontent.com/xdevjr/image_reduce/main/install.sh | bash
#
# Variáveis de ambiente (opcionais):
#   REPO        Repositório GitHub (owner/repo). Padrão: xdevjr/image_reduce
#   BRANCH      Branch a instalar. Padrão: main
#   BIN_DIR     Pasta oculta na home onde o binário será instalado.
#               Padrão: ~/.local/bin
#   GO_VERSION  Versão do Go a instalar se necessário. Padrão: 1.26.5
#
# O que o script faz:
#   1. Detecta o gerenciador de pacotes e instala as dependências de
#      build/runtime (libgtk-3, webkit2gtk-4.1, libwebp, ffmpeg, git, curl).
#   2. Garante um Go >= 1.21 (baixa o Go 1.26.5 para ~/.local/go se preciso).
#   3. Clona o repositório (shallow), compila e instala o binário em BIN_DIR
#      (~/.local/bin) e adiciona essa pasta ao PATH automaticamente no shell
#      (.bashrc, .zshrc ou config.fish do fish).
#
set -euo pipefail

# ---------------------------------------------------------------------------
# Configuração
# ---------------------------------------------------------------------------
REPO="${REPO:-xdevjr/image_reduce}"
BRANCH="${BRANCH:-main}"
GO_VERSION="${GO_VERSION:-1.26.5}"
MIN_GO="1.21" # Go >= 1.21 faz toolchain auto-switch para a versão do go.mod

# O binário é sempre instalado em uma pasta oculta dentro da home do usuário
# (padrão XDG) e o PATH é ajustado automaticamente no shell.
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Cores para output (desativadas quando não há terminal)
if [ -t 1 ]; then
  C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'; C_RED=$'\033[31m'; C_BOLD=$'\033[1m'; C_RESET=$'\033[0m'
else
  C_GREEN=""; C_YELLOW=""; C_RED=""; C_BOLD=""; C_RESET=""
fi

info() { printf '%s[*]%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '%s[!]%s %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
die()  { printf '%s[erro]%s %s\n' "$C_RED" "$C_RESET" "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
maybe_sudo() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    die "Preciso de privilégios de root (sudo) para instalar dependências."
  fi
}

detect_pkg_manager() {
  if   command -v apt-get >/dev/null 2>&1; then echo "apt"
  elif command -v dnf     >/dev/null 2>&1; then echo "dnf"
  elif command -v pacman  >/dev/null 2>&1; then echo "pacman"
  elif command -v zypper  >/dev/null 2>&1; then echo "zypper"
  else echo ""; fi
}

# version_ge <atual> <minimo> — compara versões no formato X.Y (ex.: 1.22 >= 1.21)
version_ge() {
  [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$2" ]
}

# ---------------------------------------------------------------------------
# Dependências do sistema
# ---------------------------------------------------------------------------
install_system_deps() {
  # Se as libs de build já existem (pkg-config) e git/curl estão presentes,
  # não há nada a instalar.
  if pkg-config --exists gtk+-3.0 webkit2gtk-4.1 libwebp 2>/dev/null \
     && command -v git >/dev/null 2>&1 \
     && command -v curl >/dev/null 2>&1; then
    info "Dependências de build já presentes. Pulando instalação de pacotes."
    return 0
  fi

  local pm
  pm="$(detect_pkg_manager)"
  case "$pm" in
    apt)
      info "Instalando dependências (apt): libgtk-3-dev libwebkit2gtk-4.1-dev libwebp-dev ffmpeg git curl"
      maybe_sudo apt-get update -y
      maybe_sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev libwebp-dev ffmpeg git curl
      ;;
    dnf)
      info "Instalando dependências (dnf): gtk3-devel webkit2gtk4.1-devel libwebp-devel ffmpeg git curl"
      maybe_sudo dnf install -y gtk3-devel webkit2gtk4.1-devel libwebp-devel ffmpeg git curl
      ;;
    pacman)
      info "Instalando dependências (pacman): gtk3 webkit2gtk-4.1 libwebp ffmpeg git curl"
      maybe_sudo pacman -Sy --noconfirm gtk3 webkit2gtk-4.1 libwebp ffmpeg git curl
      ;;
    zypper)
      info "Instalando dependências (zypper): gtk3-devel webkit2gtk-4_1-devel libwebp-devel ffmpeg git curl"
      maybe_sudo zypper --non-interactive install gtk3-devel webkit2gtk-4_1-devel libwebp-devel ffmpeg git curl
      ;;
    *)
      warn "Gerenciador de pacotes não detectado. Instale manualmente:"
      warn "  libgtk-3-dev libwebkit2gtk-4.1-dev libwebp-dev ffmpeg git curl"
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Go
# ---------------------------------------------------------------------------
ensure_go() {
  if command -v go >/dev/null 2>&1; then
    local v
    v="$(go version | sed -n 's/.*go\([0-9][0-9]*\.[0-9][0-9]*\).*/\1/p')"
    if [ -n "$v" ] && version_ge "$v" "$MIN_GO"; then
      info "Go encontrado: $(go version) (toolchain auto-switch para $GO_VERSION)"
      return 0
    fi
    warn "Go instalado ($v) é mais antigo que $MIN_GO. Instalando Go $GO_VERSION."
  else
    info "Go não encontrado. Instalando Go $GO_VERSION."
  fi

  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "Arquitetura não suportada para instalar Go: $arch" ;;
  esac

  local go_root="$HOME/.local/go"
  local tarball="$TMPDIR/go${GO_VERSION}.linux-${arch}.tar.gz"
  info "Baixando https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" -o "$tarball"
  mkdir -p "$HOME/.local"
  rm -rf "$go_root"
  tar -C "$HOME/.local" -xzf "$tarball"
  export PATH="$go_root/bin:$PATH"
  export GOROOT="$go_root"
  info "Go $GO_VERSION instalado em $go_root"
}

# ---------------------------------------------------------------------------
# Build e instalação
# ---------------------------------------------------------------------------
build_and_install() {
  local src="$TMPDIR/image_reduce"
  info "Clonando $REPO (branch $BRANCH)..."
  git clone --quiet --depth 1 --branch "$BRANCH" "https://github.com/$REPO.git" "$src"

  info "Compilando (go build)..."
  ( cd "$src" && go build -o image_reduce . )

  mkdir -p "$BIN_DIR" 2>/dev/null || maybe_sudo mkdir -p "$BIN_DIR"
  if ! install -m 0755 "$src/image_reduce" "$BIN_DIR/image_reduce" 2>/dev/null; then
    maybe_sudo install -m 0755 "$src/image_reduce" "$BIN_DIR/image_reduce"
  fi
  info "Binário instalado em $BIN_DIR/image_reduce"
}

# ---------------------------------------------------------------------------
# PATH — adiciona BIN_DIR ao PATH automaticamente no shell do usuário
# ---------------------------------------------------------------------------
shell_rc_file() {
  local sh
  sh="$(basename "${SHELL:-}")"
  case "$sh" in
    zsh)  echo "$HOME/.zshrc" ;;
    fish) echo "$HOME/.config/fish/config.fish" ;;
    *)    echo "$HOME/.bashrc" ;;
  esac
}

ensure_path() {
  # Já está no PATH? Nada a fazer.
  case ":$PATH:" in
    *":$BIN_DIR:"*)
      info "$BIN_DIR já está no PATH."
      return 0
      ;;
  esac

  local rc line
  rc="$(shell_rc_file)"
  case "$(basename "${SHELL:-}")" in
    fish)
      # fish_add_path é idempotente e resolve o PATH no login
      line="fish_add_path \"$BIN_DIR\""
      ;;
    *)
      line="export PATH=\"$BIN_DIR:\$PATH\""
      ;;
  esac

  if [ -f "$rc" ] && grep -qF -- "$line" "$rc"; then
    info "PATH já configurado em $rc"
    return 0
  fi

  mkdir -p "$(dirname "$rc")"
  printf '\n# Adicionado pelo instalador do Image Reduce\n%s\n' "$line" >> "$rc"
  info "PATH configurado em $rc"
}

# ---------------------------------------------------------------------------
# Desinstalação
# ---------------------------------------------------------------------------
# Desinstalação
# ---------------------------------------------------------------------------
remove_path_from_rc() {
  local rc line
  rc="$(shell_rc_file)"
  case "$(basename "${SHELL:-}")" in
    fish) line="fish_add_path \"$BIN_DIR\"" ;;
    *)    line="export PATH=\"$BIN_DIR:\$PATH\"" ;;
  esac

  if [ -f "$rc" ] \
     && { grep -qF -- "$line" "$rc" || grep -qF -- "# Adicionado pelo instalador do Image Reduce" "$rc"; }; then
    grep -vxF -- "$line" "$rc" \
      | grep -vxF -- "# Adicionado pelo instalador do Image Reduce" > "$rc.tmp"
    mv "$rc.tmp" "$rc"
    info "Linha do PATH removida de $rc"
  fi
}

uninstall() {
  if [ -f "$BIN_DIR/image_reduce" ]; then
    rm -f "$BIN_DIR/image_reduce" 2>/dev/null || maybe_sudo rm -f "$BIN_DIR/image_reduce"
    info "Binário removido de $BIN_DIR/image_reduce"
  else
    warn "Nenhum binário encontrado em $BIN_DIR/image_reduce"
  fi
  remove_path_from_rc
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  if [ "${1:-}" = "--uninstall" ]; then
    uninstall
    exit 0
  fi

  echo
  echo "$C_BOLD  Image Reduce — instalador$C_RESET"
  echo
  install_system_deps
  ensure_go
  build_and_install
  ensure_path
  echo
  info "Instalação concluída!"
  info "Binário: $BIN_DIR/image_reduce"
  case ":$PATH:" in
    *":$BIN_DIR:"*) info "Comando 'image_reduce' disponível no PATH atual." ;;
    *) info "Abra um novo terminal (ou rode 'source $HOME/.bashrc') para usar 'image_reduce'." ;;
  esac
  echo
}

main "$@"