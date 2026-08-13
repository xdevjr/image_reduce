// Package watcher monitora a pasta de entrada e enfileira arquivos novos.
package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"image_reduce/internal/config"
	"image_reduce/internal/queue"
)

// Watcher observa a pasta de entrada usando fsnotify.
type Watcher struct {
	cfg     *config.Config
	queue   *queue.Queue
	fw      *fsnotify.Watcher
	done    chan struct{}
	mu      sync.RWMutex
	paused  bool
	pending map[string]struct{}
}

// New cria um Watcher para a pasta configurada.
func New(cfg *config.Config, q *queue.Queue) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		cfg:     cfg,
		queue:   q,
		fw:      fw,
		done:    make(chan struct{}),
		pending: make(map[string]struct{}),
	}, nil
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

// Paused informa se o monitoramento está pausado.
func (w *Watcher) Paused() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.paused
}

// SetPaused pausa ou retoma o monitoramento. Ao retomar, arquivos que
// chegaram durante a pausa são enfileirados por um novo scan da pasta.
func (w *Watcher) SetPaused(paused bool) {
	w.mu.Lock()
	changed := w.paused != paused
	w.paused = paused
	w.mu.Unlock()
	if changed && !paused {
		go w.scanExisting()
	}
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
			// Renomeação/remoção libera o nome (arquivo já tratado pelo
			// conversor), permitindo que o mesmo nome seja enfileirado
			// novamente no futuro.
			if ev.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
				w.forgetPending(ev.Name)
				continue
			}
			if w.Paused() {
				continue
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write) == 0 {
				continue
			}
			if w.ignored(ev.Name) {
				continue
			}
			info, err := os.Stat(ev.Name)
			if err != nil || info.IsDir() {
				// Arquivo sumiu antes de enfileirar: libera o nome.
				w.forgetPending(ev.Name)
				continue
			}
			// Um mesmo arquivo gera vários eventos (Create + Write);
			// enfileira apenas a primeira vez.
			if !w.markPending(ev.Name) {
				continue
			}
			if !w.waitStable(ev.Name) {
				w.forgetPending(ev.Name)
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

// ignored retorna true para a subpasta processed/, para a própria raiz e para
// arquivos que casem com os padrões de ignorar da configuração.
func (w *Watcher) ignored(path string) bool {
	rel, err := filepath.Rel(w.cfg.WatchDir, path)
	if err != nil {
		return true
	}
	if rel == "." {
		return true
	}
	if rel == "processed" || strings.HasPrefix(rel, "processed"+string(filepath.Separator)) {
		return true
	}
	return w.cfg.IsIgnored(rel)
}

// markPending registra o arquivo como já enfileirado. Retorna false se ele
// já estiver pendente (eventos duplicados de Create/Write).
func (w *Watcher) markPending(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.pending[path]; ok {
		return false
	}
	w.pending[path] = struct{}{}
	return true
}

// forgetPending libera o nome do arquivo (processado, removido ou instável).
func (w *Watcher) forgetPending(path string) {
	w.mu.Lock()
	delete(w.pending, path)
	w.mu.Unlock()
}

// waitStable aguarda o arquivo parar de crescer (cópia concluída).
// O limite maior acomoda cópias de vídeos grandes.
func (w *Watcher) waitStable(path string) bool {
	var lastSize int64 = -1
	stable := 0
	for i := 0; i < 100; i++ { // máx. ~20s
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
		if !w.markPending(p) {
			continue
		}
		w.queue.Enqueue(p)
	}
}
