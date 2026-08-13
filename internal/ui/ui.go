// Package ui gerencia a janela webview (histórico e configurações).
package ui

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/webview/webview_go"

	"image_reduce/internal/app"
	"image_reduce/internal/config"
	"image_reduce/internal/history"
)

//go:embed assets/index.html
var htmlFS embed.FS

// Tipos de comando enviados pela tray para o loop da janela.
const (
	// CmdOpen abre a janela (em uma aba específica, se indicada).
	CmdOpen = iota
	// CmdQuit encerra o aplicativo.
	CmdQuit
)

// Command é um comando enviado pela tray para o loop da janela.
type Command struct {
	Kind int
	Tab  string
}

// State é o estado inicial enviado ao front-end.
type State struct {
	Config   config.Config    `json:"config"`
	History  []*history.Event `json:"history"`
	Watching bool             `json:"watching"`
	Tab      string           `json:"tab"`
}

// UI coordena a janela webview e os comandos da tray.
type UI struct {
	a       *app.App
	cmds    chan Command
	loop    chan Command
	open    atomic.Bool
	openTab string
	mu      sync.Mutex
	w       webview.WebView
}

// New cria uma UI ligada ao App.
func New(a *app.App) *UI {
	return &UI{
		a:    a,
		cmds: make(chan Command, 1),
		loop: make(chan Command, 1),
	}
}

// Open é chamado pela tray para abrir a janela na aba indicada
// ("history" ou "config").
func (u *UI) Open(tab string) {
	select {
	case u.cmds <- Command{Kind: CmdOpen, Tab: tab}:
	default:
	}
}

// OpenHistory abre a janela na aba Histórico.
func (u *UI) OpenHistory() { u.openDelayed("history") }

// OpenConfig abre a janela na aba Configurações.
func (u *UI) OpenConfig() { u.openDelayed("config") }

// openDelayed abre a janela após um pequeno atraso. Ao acionar pelo menu da
// barra, o popup (xdg_popup) ainda detém o grab de ponteiro do compositor no
// momento do clique; abrir a janela imediatamente faz o primeiro clique nela
// ser consumido pelo grab que está sendo desfeito (observado no niri/XWayland).
// O atraso deixa o popup fechar antes de a janela aparecer.
func (u *UI) openDelayed(tab string) {
	time.AfterFunc(300*time.Millisecond, func() { u.Open(tab) })
}

// Toggle alterna a visibilidade da janela: abre se fechada, fecha se aberta.
func (u *UI) Toggle() {
	if u.open.Load() {
		u.mu.Lock()
		w := u.w
		u.mu.Unlock()
		if w != nil {
			w.Terminate()
		}
		return
	}
	u.Open("history")
}

// Quit é chamado pela tray para encerrar o aplicativo.
func (u *UI) Quit() {
	select {
	case u.cmds <- Command{Kind: CmdQuit}:
	default:
	}
}

// Run roda o loop principal na main thread (exigência do webview).
func (u *UI) Run() {
	go u.dispatcher()
	for {
		select {
		case cmd := <-u.loop:
			switch cmd.Kind {
			case CmdOpen:
				u.openWindow(cmd.Tab)
			case CmdQuit:
				return
			}
		}
	}
}

// dispatcher lê comandos da tray e os encaminha ao loop da main thread.
func (u *UI) dispatcher() {
	for cmd := range u.cmds {
		switch cmd.Kind {
		case CmdOpen:
			if !u.open.Load() {
				u.loop <- cmd
			} else {
				// Janela já aberta: troca para a aba pedida e traz ao foco.
				u.mu.Lock()
				w := u.w
				u.mu.Unlock()
				if w != nil {
					w.Eval("switchTab(" + strconv.Quote(cmd.Tab) + ")")
					w.Dispatch(func() { w.Present() })
				}
			}
		case CmdQuit:
			u.mu.Lock()
			w := u.w
			u.mu.Unlock()
			if w != nil {
				// Fecha a janela se estiver aberta; Run() retorna e o loop
				// processa o CmdQuit em seguida.
				w.Terminate()
			}
			u.loop <- Command{Kind: CmdQuit}
		}
	}
}

func (u *UI) openWindow(tab string) {
	u.open.Store(true)
	defer u.open.Store(false)
	u.openTab = tab

	html, err := htmlFS.ReadFile("assets/index.html")
	if err != nil {
		return
	}

	w := webview.New(false)
	u.mu.Lock()
	u.w = w
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.w = nil
		u.mu.Unlock()
		w.Destroy()
	}()

	w.SetTitle("Image Reduce")
	w.SetSize(760, 560, webview.HintNone)
	// Navegar via data URL base64 evita problemas de renderização que ocorrem
	// com SetHtml em algumas versões do WebKitGTK.
	w.Navigate("data:text/html;base64," + base64.StdEncoding.EncodeToString(html))
	// Dá foco à janela assim que o loop GTK iniciar. Sem isso, ao abrir pelo
	// menu da tray a janela pode ficar sem foco e os botões só respondem
	// após um clique dentro dela.
	w.Dispatch(func() { w.Present() })
	if err := w.Bind("getState", u.getState); err != nil {
		return
	}
	if err := w.Bind("saveConfig", u.saveConfig); err != nil {
		return
	}
	if err := w.Bind("clearHistory", u.clearHistory); err != nil {
		return
	}
	if err := w.Bind("setWatcherPaused", u.setWatcherPaused); err != nil {
		return
	}
	if err := w.Bind("closeWindow", func() { w.Terminate() }); err != nil {
		return
	}
	if err := w.Bind("selectFolder", func() string {
		// zenity/kdialog rodam em processo separado, sem bloquear a main
		// thread do GTK (evita o deadlock de gtk_dialog_run no Dispatch).
		return selectFolder()
	}); err != nil {
		return
	}
	if err := w.Bind("openFolderBinding", func(path string) bool {
		return openFolder(path)
	}); err != nil {
		return
	}
	if err := w.Bind("openFileBinding", func(path string) bool {
		return openFile(path)
	}); err != nil {
		return
	}
	if err := w.Bind("showInFolderBinding", func(path string) bool {
		return showInFolder(path)
	}); err != nil {
		return
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		u.pushEvents(w, done)
	}()

	// Reforça o pedido de foco com retentativas: alguns compositores (ex.:
	// niri/XWayland) só aceitam o foco após a janela estar plenamente visível.
	for _, delay := range []time.Duration{300 * time.Millisecond, 800 * time.Millisecond} {
		time.AfterFunc(delay, func() {
			select {
			case <-done:
				return
			default:
			}
			w.Dispatch(func() { w.Present() })
		})
	}

	w.Run()
	close(done)
	wg.Wait()
}

// pushEvents encaminha eventos de progresso do App para o JavaScript.
func (u *UI) pushEvents(w webview.WebView, done chan struct{}) {
	for {
		select {
		case ev := <-u.a.Events():
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			w.Eval("window.pushEvent(" + string(data) + ")")
		case <-done:
			return
		}
	}
}
func (u *UI) getState() State {
	return State{
		Config:   *u.a.Config(),
		History:  u.a.History(),
		Watching: !u.a.WatcherPaused(),
		Tab:      u.openTab,
	}
}

func (u *UI) saveConfig(cfg config.Config) error {
	return u.a.SaveConfig(&cfg)
}

func (u *UI) clearHistory() {
	u.a.ClearHistory()
}

func (u *UI) setWatcherPaused(paused bool) {
	u.a.SetWatcherPaused(paused)
}
