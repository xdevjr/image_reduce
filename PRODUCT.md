# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Usuário Linux geral (GNOME, KDE Plasma, XFCE, Cinnamon) que quer economizar espaço em disco automaticamente: solta imagens e vídeos em uma pasta monitorada e o app converte para WebP/WebM em segundo plano, sem interação. Interface e documentação em português (pt-BR).

## Product Purpose

O Image Reduce é um aplicativo de bandeja (system tray) para Linux que monitora uma pasta de entrada, converte automaticamente novas imagens (PNG, JPEG, GIF estático, BMP, TIFF, WebP) para WebP lossy (qualidade 90, preservando alpha) e vídeos (MP4, MKV, MOV, AVI, WebM, etc.) para WebM (AV1 + Opus) via ffmpeg, movendo os resultados para uma pasta de saída. Sucesso = o usuário solta arquivos e recebe versões menores na pasta de saída sem abrir nenhuma ferramenta.

## Positioning

Automação "soltar e esquecer": o diferencial é o watch-folder — o app monitora a pasta e converte sozinho, em segundo plano, com qualidade preservada (WebP lossy 90 com alpha; vídeo AV1 só quando o resultado fica menor). Um concorrente não copiaria facilmente a combinação de automação de pasta + preservação de qualidade + suporte a vídeo + integração nativa de bandeja no Linux.

## Operating Context

- Aplicativo de bandeja (system tray) via fyne.io/systray; janela webview (WebKitGTK 4.1) para histórico e configurações.
- Roda em segundo plano (`image_reduce start`) ou primeiro plano (`image_reduce run`); log e pid em `~/.cache/image_reduce/`.
- Backend GTK3 X11 (via XWayland); DMA-BUF/compositing desabilitados para evitar tela preta.
- Seletor de pastas nativo: zenity (GNOME) ou kdialog (KDE); abertura de pastas via xdg-open.
- Config em `~/.config/image_reduce/config.json`; histórico em JSONL (máx. 500 eventos).
- Pastas padrão: `~/Pictures/image_reduce/in` e `~/Pictures/image_reduce/out`.
- Instalação via `install.sh` (curl | bash): detecta apt/dnf/pacman/zypper, instala deps, garante Go, instala em `~/.local/bin`, adiciona ao PATH e cria atalho `.desktop`.

## Capabilities and Constraints

- Converte imagens para WebP lossy (qualidade 90, alpha preservado); WebP já existentes são recompactados quando menores.
- Converte vídeos para WebM (AV1 + Opus) via ffmpeg (SVT-AV1, fallback libaom-av1), apenas quando o resultado fica menor; requer ffmpeg com encoder AV1.
- GIFs animados, WebPs animados e não-imagens são copiados sem conversão; originais movidos para `processed/` ou excluídos (configurável).
- Padrões de ignorar arquivos (separados por vírgula; `.*` ignora ocultos por padrão).
- Concorrência configurável (padrão 8), qualidade (padrão 90), pausar/retomar monitoramento, limpar histórico.
- Dependências de build: `libgtk-3-dev libwebkit2gtk-4.1-dev libwebp-dev ffmpeg`.
- Fork local `third_party/webview_go` (webkit2gtk-4.1) via replace no go.mod — não atualizar sem verificar compatibilidade.
- UI carregada via `data:text/html;base64,...` (mais robusto no WebKitGTK).
- Interface em pt-BR.

## Brand Commitments

- Nome: "Image Reduce" (marca no cabeçalho da janela: "Image Reduce" com acento na segunda palavra).
- Ícone: `internal/tray/icon.png` (usado na bandeja e no atalho `.desktop`).
- Repositório público: `github.com/xdevjr/image_reduce` (branch `main`).
- Idioma da UI e documentação: português (pt-BR).

## Evidence on Hand

- Repositório: README.md (funcionalidades, instalação, configuração), install.sh, AGENTS.md.
- Código-fonte completo em Go (internal/app, config, converter, history, queue, tray, ui, video, watcher).
- Ícone da bandeja: `internal/tray/icon.png`.
- Sem testemunhos, clientes, benchmarks ou casos de uso publicados — não fabricar.

## Product Principles

1. Automação em segundo plano: o app trabalha sozinho; a interface existe para informar e ajustar, não para exigir atenção.
2. Qualidade preservada: conversão lossy com alpha preservado e vídeo só quando fica menor — nunca sacrificar o resultado por tamanho.
3. Nativo e leve no Linux: bandeja, seletor de pastas nativo, sem GUI pesada.
4. Configuração em tempo real: mudanças aplicadas sem reiniciar o app.
5. Previsibilidade: originais sempre preservados (`processed/`) a menos que o usuário opte por excluir.
