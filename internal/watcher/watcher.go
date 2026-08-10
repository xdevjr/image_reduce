// Package watcher monitora a pasta de entrada e enfileira arquivos novos.
package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"image_reduce/internal/config"
	"image_reduce/internal/queue"
)

// Watcher observa a pasta de entrada usando fsnotify.
type Watcher struct {
	cfg   *config.Config
	queue *queue.Queue
	fw    *fsnotify.Watcher
	done  chan struct{}
}

// New cria um Watcher para a pasta configurada.
func New(cfg *config.Config, q *queue.Queue) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{cfg: cfg, queue: q, fw: fw, done: make(chan struct{})}, nil
}

// Start inicia o monitoramento e faz um scan inicial da pasta.
func (w *Watcher) Start() error {
	if err := os.MkdirAll(w.cfg.WatchDir, 0o755); err != nil {
		return err
	}
	if err := w.fw.Add(w.cfg.WatchDir); err != nil {
		return err
	}
	go w.loop()
	w.scanExisting()
	return nil
}

// Stop encerra o monitoramento.
func (w *Watcher) Stop() {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
	w.fw.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-w.fw.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			if w.ignored(ev.Name) {
				continue
			}
			info, err := os.Stat(ev.Name)
			if err != nil || info.IsDir() {
				continue
			}
			if !w.waitStable(ev.Name) {
				continue
			}
			w.queue.Enqueue(ev.Name)
		case _, ok := <-w.fw.Errors:
			if !ok {
				return
			}
		}
	}
}

// ignored retorna true para a subpasta processed/ e para a própria raiz.
func (w *Watcher) ignored(path string) bool {
	rel, err := filepath.Rel(w.cfg.WatchDir, path)
	if err != nil {
		return true
	}
	if rel == "." {
		return true
	}
	return rel == "processed" || strings.HasPrefix(rel, "processed"+string(filepath.Separator))
}

// waitStable aguarda o arquivo parar de crescer (cópia concluída).
func (w *Watcher) waitStable(path string) bool {
	var lastSize int64 = -1
	stable := 0
	for i := 0; i < 50; i++ { // máx. ~10s
		info, err := os.Stat(path)
		if err != nil {
			return false
		}
		if info.Size() == lastSize && lastSize >= 0 {
			stable++
			if stable >= 2 {
				return true
			}
		} else {
			stable = 0
		}
		lastSize = info.Size()
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// scanExisting enfileira arquivos já presentes na pasta no boot.
func (w *Watcher) scanExisting() {
	entries, err := os.ReadDir(w.cfg.WatchDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(w.cfg.WatchDir, e.Name())
		if w.ignored(p) {
			continue
		}
		w.queue.Enqueue(p)
	}
}