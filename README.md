# Image Reduce

Aplicativo de bandeja (system tray) para Linux que monitora uma pasta, converte
automaticamente novas imagens para **WebP** (lossy, qualidade 90, preservando
transparência/alpha) e move os arquivos convertidos para uma pasta de saída.

## Funcionalidades

- 🖼️ Monitora uma pasta de entrada e converte imagens (PNG, JPEG, GIF estático,
  BMP, TIFF) para WebP com qualidade 90 (lossy), preservando o canal alpha.
- 🎞️ GIFs animados e arquivos não-imagem são **movidos sem conversão** para a
  pasta de saída.
- 📁 Os originais são movidos para `processed/` dentro da pasta monitorada
  (ou excluídos, conforme configuração).
- ⚙️ Conversões concorrentes configuráveis (padrão: 8).
- 🖥️ Janela de histórico/progresso e configurações, aberta pela bandeja.
- 💾 Configuração persistida em `~/.config/image_reduce/config.json`.

## Dependências de sistema

### Para compilar

```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev libwebp-dev
```

### Para executar

- Um ambiente gráfico com suporte a bandeja (system tray):
  - **GNOME**: instale a extensão
    [AppIndicator and KStatusNotifierItem Support](https://extensions.gnome.org/extension/615/appindicator-and-kstatusnotifieritem-support/).
  - **KDE Plasma / XFCE / Cinnamon**: suporte nativo.
- O GTK3 usa o backend **X11** (via XWayland) para abrir a janela, pois o
  webkit2gtk-4.1 apresenta erro de protocolo com alguns compositores Wayland.
- A aceleração de hardware problemática (GBM/DMA-BUF) é desabilitada
  automaticamente para evitar tela preta/em branco.
- O HTML da janela é carregado via `data:text/html;base64,...` em vez de
  `SetHtml`, o que é mais robusto no WebKitGTK.
  Tudo isso é feito automaticamente pelo app (não requer ação do usuário).

## Compilar

```bash
go build -o image_reduce .
```

## Executar

```bash
./image_reduce
```

O app fica na bandeja. Clique no ícone para abrir a janela (histórico e
configurações) ou sair. A janela também possui um botão **Fechar** no canto
superior direito.

## Tasks (mise)

O projeto inclui um `mise.toml` que fixa a versão do **Go 1.26.5** e define
tarefas prontas:

```bash
mise install      # instala a versão do Go definida no mise.toml
mise run dev      # roda em modo dev (go run .)
mise run build    # compila o binário image_reduce
mise run test     # roda os testes (go test ./...)
```

## Configuração

Arquivo: `~/.config/image_reduce/config.json`

| Chave            | Descrição                                        | Padrão                              |
|------------------|--------------------------------------------------|-------------------------------------|
| `watch_dir`      | Pasta monitorada                                 | `~/Pictures/image_reduce/in`        |
| `output_dir`     | Pasta de saída (WebP e arquivos movidos)         | `~/Pictures/image_reduce/out`       |
| `max_concurrent` | Conversões simultâneas                           | `8`                                 |
| `quality`        | Qualidade WebP (0–100, lossy)                    | `90`                                |
| `delete_original`| Excluir o original em vez de mover para `processed/` | `false`                          |

A configuração também pode ser editada pela janela do app (aba
"Configurações"). Alterações são aplicadas em tempo real.

## Estrutura do projeto

```
main.go                  Bootstrap (backend X11, app, tray, UI)
internal/
  app/                   Orquestrador (config, fila, conversor, watcher)
  config/                Configuração persistida em JSON
  converter/             Conversão WebP + detecção de formato
  history/               Histórico persistido (JSONL, máx. 500 eventos)
  queue/                 Pool de workers dinâmico
  tray/                  Ícone e menu da bandeja (fyne.io/systray)
  ui/                    Janela webview (histórico + configurações)
  watcher/               Monitoramento da pasta (fsnotify)
third_party/webview_go/  Fork local do webview_go (webkit2gtk-4.1)
```

## Nota sobre o fork do `webview_go`

O pacote `github.com/webview/webview_go` fixa `webkit2gtk-4.0` no cgo, mas
distribuições recentes (ex.: Ubuntu 24.04+) fornecem apenas `webkit2gtk-4.1`.
O projeto usa um fork local em `third_party/webview_go` (apontado via
`replace` no `go.mod`) que:

- usa `webkit2gtk-4.1` no `#cgo pkg-config`;
- remove imports do Windows (`mswebview2`, `include`) para permitir build no
  Linux.

## Testes

```bash
go test ./...
```

Cobre: conversão PNG com alpha, exclusão de original, pulo de GIF animado,
pulo de não-imagem e colisão de nomes na saída.
