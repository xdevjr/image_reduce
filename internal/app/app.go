// Package app orquestra config, histórico, fila, conversor e watcher.
package app

import (
	"sync"

	"image_reduce/internal/config"
	"image_reduce/internal/converter"
	"image_reduce/internal/history"
	"image_reduce/internal/queue"
	"image_reduce/internal/watcher"
)

// App é o orquestrador central do aplicativo.
type App struct {
	mu      sync.Mutex
	cfg     *config.Config
	history *history.Store
	queue   *queue.Queue
	conv    *converter.Converter
	watcher *watcher.Watcher
	events  chan *history.Event
}

// New monta o App a partir da configuração.
func New(cfg *config.Config) (*App, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	histPath, err := config.HistoryPath()
	if err != nil {
		return nil, err
	}
	h := history.New(histPath)
	events := make(chan *history.Event, 256)
	a := &App{cfg: cfg, history: h, events: events}
	a.conv = converter.New(cfg, h, events)
	a.queue = queue.New(a.conv.Process, cfg.MaxConcurrent)
	w, err := watcher.New(cfg, a.queue)
	if err != nil {
		return nil, err
	}
	a.watcher = w
	return a, nil
}

// Start inicia o monitoramento da pasta de entrada.
func (a *App) Start() error {
	return a.watcher.Start()
}

// Stop encerra o monitoramento e a fila.
func (a *App) Stop() {
	a.watcher.Stop()
	a.queue.Close()
}

// WatcherPaused informa se o monitoramento está pausado.
func (a *App) WatcherPaused() bool {
	a.mu.Lock()
	w := a.watcher
	a.mu.Unlock()
	if w == nil {
		return false
	}
	return w.Paused()
}

// SetWatcherPaused pausa ou retoma o monitoramento manualmente.
func (a *App) SetWatcherPaused(paused bool) {
	a.mu.Lock()
	w := a.watcher
	a.mu.Unlock()
	if w != nil {
		w.SetPaused(paused)
	}
}

// Events retorna o canal de eventos de progresso/histórico.
func (a *App) Events() <-chan *history.Event { return a.events }

// History retorna o histórico atual.
func (a *App) History() []*history.Event { return a.history.List() }

// Config retorna a configuração ativa.
func (a *App) Config() *config.Config { return a.cfg }

// ClearHistory apaga o histórico persistido.
func (a *App) ClearHistory() { a.history.Clear() }

// SaveConfig aplica e persiste uma nova configuração em runtime.
func (a *App) SaveConfig(cfg *config.Config) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	cfg.EnsureDefaults()
	watchChanged := cfg.WatchDir != a.cfg.WatchDir
	if cfg.MaxConcurrent != a.cfg.MaxConcurrent {
		a.queue.SetMax(cfg.MaxConcurrent)
	}
	*a.cfg = *cfg
	a.conv.SetConfig(a.cfg)
	if err := a.cfg.Save(); err != nil {
		return err
	}
	if err := a.cfg.EnsureDirs(); err != nil {
		return err
	}
	if watchChanged {
		a.watcher.Stop()
		w, err := watcher.New(a.cfg, a.queue)
		if err != nil {
			return err
		}
		a.watcher = w
		if err := a.watcher.Start(); err != nil {
			return err
		}
	}
	return nil
}
