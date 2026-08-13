package watcher

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"image_reduce/internal/config"
	"image_reduce/internal/queue"
)

func TestWatcherPause(t *testing.T) {
	cfg := &config.Config{WatchDir: t.TempDir(), MaxConcurrent: 1}
	q := queue.New(func(string) {}, 1)
	defer q.Close()

	w, err := New(cfg, q)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	if w.Paused() {
		t.Fatal("watcher não deveria começar pausado")
	}
	w.SetPaused(true)
	if !w.Paused() {
		t.Fatal("watcher deveria estar pausado")
	}
	w.SetPaused(false)
	if w.Paused() {
		t.Fatal("watcher deveria ter retomado")
	}
}

func TestWatcherDedupEvents(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	var got []string
	q := queue.New(func(p string) {
		mu.Lock()
		got = append(got, p)
		mu.Unlock()
	}, 1)
	defer q.Close()

	cfg := &config.Config{WatchDir: dir}
	w, err := New(cfg, q)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	go w.loop()

	p := filepath.Join(dir, "foto.png")
	if err := os.WriteFile(p, []byte("conteúdo"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Um arquivo gera Create + Write; deve enfileirar apenas uma vez.
	w.fw.Events <- fsnotify.Event{Name: p, Op: fsnotify.Create}
	w.fw.Events <- fsnotify.Event{Name: p, Op: fsnotify.Write}

	waitCount := func(want int) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			n := len(got)
			mu.Unlock()
			if n >= want {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		mu.Lock()
		n := len(got)
		mu.Unlock()
		t.Fatalf("esperado %d enfileiramento(s), obteve %d", want, n)
	}

	waitCount(1)

	// Após renomeação (arquivo tratado), o mesmo nome pode ser enfileirado
	// novamente.
	w.fw.Events <- fsnotify.Event{Name: p, Op: fsnotify.Rename}
	w.fw.Events <- fsnotify.Event{Name: p, Op: fsnotify.Create}
	waitCount(2)
}
