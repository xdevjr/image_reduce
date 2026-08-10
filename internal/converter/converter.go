// Package converter implementa o pipeline de conversão de imagens para WebP.
package converter

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chai2010/webp"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"

	"image_reduce/internal/config"
	"image_reduce/internal/history"
)

// Snapshot é uma cópia imutável das opções usadas por uma conversão.
type Snapshot struct {
	OutputDir      string
	ProcessedDir   string
	Quality        float64
	DeleteOriginal bool
}

// Converter processa arquivos: converte imagens para WebP e trata originais.
type Converter struct {
	mu      sync.RWMutex
	snap    Snapshot
	history *history.Store
	events  chan<- *history.Event
}

var idCounter int64

func newID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), atomic.AddInt64(&idCounter, 1))
}

// New cria um Converter ligado ao histórico e ao canal de eventos.
func New(cfg *config.Config, h *history.Store, events chan<- *history.Event) *Converter {
	c := &Converter{history: h, events: events}
	c.SetConfig(cfg)
	return c
}

// SetConfig atualiza as opções usadas pelas próximas conversões.
func (c *Converter) SetConfig(cfg *config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snap = Snapshot{
		OutputDir:      cfg.OutputDir,
		ProcessedDir:   cfg.ProcessedDir(),
		Quality:        cfg.Quality,
		DeleteOriginal: cfg.DeleteOriginal,
	}
}

// Process converte (ou move) um único arquivo.
func (c *Converter) Process(path string) {
	c.mu.RLock()
	snap := c.snap
	c.mu.RUnlock()

	ev := &history.Event{
		ID:        newID(),
		File:      filepath.Base(path),
		Source:    path,
		Status:    history.StatusConverting,
		Timestamp: time.Now(),
	}
	if info, err := os.Stat(path); err == nil {
		ev.SizeIn = info.Size()
	}
	c.emit(ev)
	start := time.Now()
	defer func() { ev.Duration = time.Since(start).Round(time.Millisecond).String() }()

	f, err := os.Open(path)
	if err != nil {
		c.fail(ev, err)
		return
	}
	defer f.Close()

	format, err := detectFormat(f)
	if err != nil {
		c.moveAsIs(ev, path, snap, "not an image")
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		c.fail(ev, err)
		return
	}

	var img image.Image
	if format == "gif" {
		g, err := gif.DecodeAll(f)
		if err != nil {
			c.moveAsIs(ev, path, snap, "not an image")
			return
		}
		if len(g.Image) > 1 {
			c.moveAsIs(ev, path, snap, "animated gif")
			return
		}
		img = g.Image[0]
	} else {
		img, _, err = image.Decode(f)
		if err != nil {
			c.moveAsIs(ev, path, snap, "not an image")
			return
		}
	}

	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, &webp.Options{Quality: float32(snap.Quality)}); err != nil {
		c.fail(ev, err)
		return
	}

	outPath, err := writeUnique(filepath.Join(snap.OutputDir, stem(filepath.Base(path))+".webp"), buf.Bytes())
	if err != nil {
		c.fail(ev, err)
		return
	}

	ev.Status = history.StatusDone
	ev.Output = outPath
	ev.SizeOut = int64(buf.Len())
	c.emit(ev)

	if err := c.handleOriginal(path, snap); err != nil {
		errEv := &history.Event{
			ID:        newID(),
			File:      filepath.Base(path),
			Source:    path,
			Status:    history.StatusError,
			Error:     "falha ao tratar original: " + err.Error(),
			Timestamp: time.Now(),
		}
		c.emit(errEv)
	}
}

// handleOriginal apaga ou move o original conforme a configuração.
func (c *Converter) handleOriginal(src string, snap Snapshot) error {
	if snap.DeleteOriginal {
		return os.Remove(src)
	}
	dest := filepath.Join(snap.ProcessedDir, filepath.Base(src))
	return moveFile(src, dest)
}

// moveAsIs move um arquivo sem conversão para a pasta de saída.
func (c *Converter) moveAsIs(ev *history.Event, src string, snap Snapshot, reason string) {
	dest := filepath.Join(snap.OutputDir, filepath.Base(src))
	if err := moveFile(src, dest); err != nil {
		c.fail(ev, err)
		return
	}
	ev.Status = history.StatusSkipped
	ev.Reason = reason
	ev.Output = dest
	c.emit(ev)
}

func (c *Converter) emit(ev *history.Event) {
	c.history.Add(ev)
	if c.events != nil {
		select {
		case c.events <- ev:
		default:
		}
	}
}

func (c *Converter) fail(ev *history.Event, err error) {
	ev.Status = history.StatusError
	ev.Error = err.Error()
	c.emit(ev)
}

// detectFormat identifica o formato pela assinatura (magic bytes).
func detectFormat(r io.Reader) (string, error) {
	head := make([]byte, 12)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	head = head[:n]
	switch {
	case len(head) >= 8 && string(head[:8]) == "\x89PNG\r\n\x1a\n":
		return "png", nil
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return "jpeg", nil
	case len(head) >= 6 && (string(head[:6]) == "GIF87a" || string(head[:6]) == "GIF89a"):
		return "gif", nil
	case len(head) >= 2 && head[0] == 'B' && head[1] == 'M':
		return "bmp", nil
	case len(head) >= 4 && ((head[0] == 'I' && head[1] == 'I' && head[2] == 0x2A && head[3] == 0x00) ||
		(head[0] == 'M' && head[1] == 'M' && head[2] == 0x00 && head[3] == 0x2A)):
		return "tiff", nil
	}
	return "", fmt.Errorf("formato desconhecido")
}

// writeUnique escreve data em path, adicionando sufixo numérico se já existir.
func writeUnique(path string, data []byte) (string, error) {
	p := path
	ext := filepath.Ext(p)
	stem := strings.TrimSuffix(p, ext)
	for i := 1; ; i++ {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			break
		}
		p = fmt.Sprintf("%s-%d%s", stem, i, ext)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

// moveFile move src para dest, com fallback de cópia+remoção entre filesystems.
func moveFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	if err := copyFile(src, dest); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func stem(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}