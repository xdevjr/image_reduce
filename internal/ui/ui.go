// Package ui gerencia a janela webview (histórico e configurações).
package ui

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"sync"
	"sync/atomic"

	"github.com/webview/webview_go"

	"image_reduce/internal/app"
	"image_reduce/internal/config"
	"image_reduce/internal/history"
)

//go:embed assets/index.html
var htmlFS embed.FS

// Command é um comando enviado pela tray para o loop da janela.
type Command int

const (
	// CmdOpen abre a janela de histórico/configurações.
	CmdOpen Command = iota
	// CmdQuit encerra o aplicativo.
	CmdQuit
)

// State é o estado inicial enviado ao front-end.
type State struct {
	Config  config.Config    `json:"config"`
	History []*history.Event `json:"history"`
}

// UI coordena a janela webview e os comandos da tray.
type UI struct {
	a    *app.App
	cmds chan Command
	loop chan Command
	open atomic.Bool
	mu   sync.Mutex
	w    webview.WebView
}

// New cria uma UI ligada ao App.
func New(a *app.App) *UI {
	return &UI{
		a:    a,
		cmds: make(chan Command, 1),
		loop: make(chan Command, 1),
	}
}

// Open é chamado pela tray para abrir a janela.
func (u *UI) Open() {
	select {
	case u.cmds <- CmdOpen:
	default:
	}
}

// Quit é chamado pela tray para encerrar o aplicativo.
func (u *UI) Quit() {
	select {
	case u.cmds <- CmdQuit:
	default:
	}
}

// Run roda o loop principal na main thread (exigência do webview).
func (u *UI) Run() {
	go u.dispatcher()
	for {
		select {
		case cmd := <-u.loop:
			switch cmd {
			case CmdOpen:
				u.openWindow()
			case CmdQuit:
				return
			}
		}
	}
}

// dispatcher lê comandos da tray e os encaminha ao loop da main thread.
func (u *UI) dispatcher() {
	for cmd := range u.cmds {
		switch cmd {
		case CmdOpen:
			if !u.open.Load() {
				u.loop <- CmdOpen
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
			u.loop <- CmdQuit
		}
	}
}

func (u *UI) openWindow() {
	u.open.Store(true)
	defer u.open.Store(false)

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
	if err := w.Bind("getState", u.getState); err != nil {
		return
	}
	if err := w.Bind("saveConfig", u.saveConfig); err != nil {
		return
	}
	if err := w.Bind("clearHistory", u.clearHistory); err != nil {
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

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		u.pushEvents(w, done)
	}()

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
		Config:  *u.a.Config(),
		History: u.a.History(),
	}
}

func (u *UI) saveConfig(cfg config.Config) error {
	return u.a.SaveConfig(&cfg)
}

func (u *UI) clearHistory() {
	u.a.ClearHistory()
}