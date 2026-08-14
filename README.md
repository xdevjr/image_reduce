# Image Reduce

Aplicativo de bandeja (system tray) para Linux que monitora uma pasta, converte
automaticamente novas imagens para **WebP** (lossy, qualidade 90, preservando
transparência/alpha) e move os arquivos convertidos para uma pasta de saída.

## Funcionalidades

- 🖼️ Monitora uma pasta de entrada e converte imagens (PNG, JPEG, GIF estático,
  BMP, TIFF e **WebP**) para WebP com qualidade 90 (lossy), preservando o canal
  alpha. Arquivos que já são WebP são recompactados quando isso reduz o tamanho.
- 🚫 Padrões de arquivos ignorados (ex.: `*.rar`, nome do arquivo), separados
  por vírgula — por padrão, `.*` ignora arquivos ocultos.
- 🎞️ GIFs animados, WebPs animados e arquivos não-imagem são **copiados sem
  conversão** para a pasta de saída; os originais também vão para `processed/`.
- 🎬 Vídeos (MP4, MKV, MOV, AVI, WebM, etc.) são convertidos para **WebM
  (AV1 + Opus)** via ffmpeg (SVT-AV1, com fallback para libaom-av1), apenas
  quando o resultado fica menor que o original.
- 📁 Os originais são movidos para `processed/` dentro da pasta monitorada
  (ou excluídos, conforme configuração).
- ⚙️ Conversões concorrentes configuráveis (padrão: 8).
- 🖥️ Janela de histórico/progresso e configurações, aberta pela bandeja.
- ⏯️ Botão para **pausar/retomar o monitoramento** manualmente (aba Histórico).
  Ao retomar, arquivos que chegaram durante a pausa são processados.
- � Na aba Configurações, botões **Selecionar…** abrem o seletor de pasta
  nativo (**zenity** no GNOME ou **kdialog** no KDE) para escolher as pastas
  monitorada e de saída.- 📂 Na aba Histórico, botões **Abrir pasta monitorada** e **Abrir pasta de
  saída** abrem as pastas no gerenciador de arquivos (via `xdg-open`), e o
  botão **Limpar histórico** apaga todos os eventos registrados.- �💾 Configuração persistida em `~/.config/image_reduce/config.json`.

## Instalação

Instale o binário e as dependências necessárias com um único comando (requer
`curl` e `bash`):

```bash
curl -fsSL https://raw.githubusercontent.com/xdevjr/image_reduce/main/install.sh | bash
```

O script `install.sh` (na raiz do repositório):

- detecta o gerenciador de pacotes (**apt**, **dnf**, **pacman**, **zypper**) e
  instala as dependências de build/runtime quando necessário
  (`libgtk-3-dev libwebkit2gtk-4.1-dev libwebp-dev ffmpeg git curl`);
- garante um Go compatível (baixa o Go 1.26.5 para `~/.local/go` se o instalado
  for antigo ou ausente);
- clona o repositório (shallow), compila e instala o binário em
  `~/.local/bin` (pasta oculta na home do usuário);
- adiciona `~/.local/bin` ao `PATH` **automaticamente** no shell do usuário
  (`.bashrc`, `.zshrc` ou `config.fish` do fish) — basta abrir um novo
  terminal para usar o comando `image_reduce`;
- cria um **atalho no launcher** (`.desktop`) com ícone, em
  `~/.local/share/applications/image_reduce.desktop`, que inicia o app em
  segundo plano (`image_reduce start`) — basta procurar por **Image Reduce**
  no menu de aplicativos.

Para desinstalar (remove o binário, a linha do PATH e o atalho `.desktop`):

```bash
curl -fsSL https://raw.githubusercontent.com/xdevjr/image_reduce/main/install.sh | bash -s -- --uninstall
```

### Variáveis de ambiente

| Variável     | Padrão                | Descrição                                       |
|--------------|-----------------------|-------------------------------------------------|
| `BIN_DIR`    | `~/.local/bin`        | Pasta oculta na home onde o binário é instalado |
| `REPO`       | `xdevjr/image_reduce` | Repositório GitHub (owner/repo)                 |
| `BRANCH`     | `main`                | Branch a instalar                               |
| `GO_VERSION` | `1.26.5`              | Versão do Go baixada se necessário              |

Exemplo instalando em outra pasta (também adicionada ao `PATH`):

```bash
BIN_DIR="$HOME/.bin" \
  curl -fsSL https://raw.githubusercontent.com/xdevjr/image_reduce/main/install.sh | bash
```

## Dependências de sistema

### Para compilar

```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev libwebp-dev ffmpeg
```

### Para executar

- Um ambiente gráfico com suporte a bandeja (system tray):
  - **GNOME**: instale a extensão
    [AppIndicator and KStatusNotifierItem Support](https://extensions.gnome.org/extension/615/appindicator-and-kstatusnotifieritem-support/).
  - **KDE Plasma / XFCE / Cinnamon**: suporte nativo.
- Para o seletor de pastas da janela de configurações:
  - **GNOME**: `zenity` (`sudo apt install zenity`)
  - **KDE**: `kdialog` (geralmente já instalado)
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

O binário aceita os subcomandos abaixo. Sem comando, exibe a ajuda.

| Comando              | Efeito                                                  |
|----------------------|---------------------------------------------------------|
| `image_reduce`       | Mostra a ajuda de uso                                   |
| `image_reduce run`   | Roda em primeiro plano, prendendo o terminal            |
| `image_reduce start` | Inicia em **segundo plano** — o comando retorna na hora |
| `image_reduce stop`  | Encerra o processo iniciado por `image_reduce start`    |
| `image_reduce reset` | Restaura a configuração para os valores padrão          |
| `image_reduce help`  | Mostra a ajuda de uso                                   |

```bash
image_reduce run
```

O app fica na bandeja. Clique com o **botão esquerdo** no ícone para
alternar (abrir/fechar) a janela de histórico e configurações; o menu (botão
direito) permite abrir diretamente o **Histórico** ou as **Configurações**,
ou sair. A janela possui um botão **Fechar** no canto superior direito.

### Iniciar/parar sem prender o terminal

Inicia e encerra o processo em segundo plano:

```bash
image_reduce start
# → image_reduce rodando em segundo plano (pid 12345).
#   Log: ~/.cache/image_reduce/image_reduce.log
#   Para parar: image_reduce stop

image_reduce stop
# → Encerrando image_reduce (pid 12345)... image_reduce encerrado.
```

A saída do processo em segundo plano é gravada no log
`~/.cache/image_reduce/image_reduce.log` e o pid fica em
`~/.cache/image_reduce/image_reduce.pid`. Também é possível parar pela
bandeja (menu → **Sair**).

Com o mise, os atalhos `mise run start` e `mise run stop` fazem o mesmo.

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
| `ignore_patterns`| Padrões separados por vírgula para ignorar arquivos (curingas `*`/`?`, casam com o nome) | `.*` (arquivos ocultos) |
| `video_enabled`  | Converter vídeos para WebM (AV1 + Opus)        | `true`                              |
| `video_crf`      | Qualidade do AV1 (1–63, menor = melhor)         | `32`                                |
| `video_preset`   | Velocidade do encoder (0–13)                    | `6`                                 |
| `notifications_enabled` | Notificações do sistema (D-Bus)          | `true`                              |
| `notify_on_done` | Notificar ao concluir conversão                  | `true`                              |
| `notify_on_error`| Notificar ao ocorrer erro                        | `true`                              |
| `notify_on_skipped` | Notificar ao ignorar arquivo                 | `false`                             |

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
  video/                 Conversão de vídeos para WebM (AV1 + Opus) via ffmpeg
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
