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
	"image_reduce/internal/fsutil"
	"image_reduce/internal/history"
	"image_reduce/internal/video"
)

// Snapshot é uma cópia imutável das opções usadas por uma conversão.
type Snapshot struct {
	OutputDir      string
	ProcessedDir   string
	Quality        float64
	DeleteOriginal bool
	VideoEnabled   bool
	VideoCRF       float64
	VideoPreset    int
}

// Converter processa arquivos: converte imagens para WebP e trata originais.
type Converter struct {
	mu      sync.RWMutex
	snap    Snapshot
	history *history.Store
	events  chan<- *history.Event
	// onIncomplete é chamado quando um vídeo falha por arquivo incompleto
	// após as tentativas, permitindo que o watcher re-enfileire o arquivo
	// quando ele terminar de ser copiado/baixado.
	onIncomplete func(path string)
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

// SetOnIncomplete registra um callback chamado quando uma conversão de vídeo
// falha por arquivo incompleto (ainda sendo copiado/baixado) após as
// tentativas de conversão.
func (c *Converter) SetOnIncomplete(fn func(path string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onIncomplete = fn
}

// notifyIncomplete chama o callback registrado (se houver) para que o
// watcher libere o nome do arquivo e o re-enfileire quando ele terminar
// de ser copiado/baixado.
func (c *Converter) notifyIncomplete(path string) {
	c.mu.RLock()
	fn := c.onIncomplete
	c.mu.RUnlock()
	if fn != nil {
		fn(path)
	}
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
		VideoEnabled:   cfg.VideoEnabled,
		VideoCRF:       cfg.VideoCRF,
		VideoPreset:    cfg.VideoPreset,
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

	if video.IsVideo(path) {
		c.processVideo(ev, path, snap)
		return
	}

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
		if format == "webp" {
			animated, err := isAnimatedWebP(f)
			if err != nil {
				c.moveAsIs(ev, path, snap, "not an image")
				return
			}
			if animated {
				c.moveAsIs(ev, path, snap, "animated webp")
				return
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				c.fail(ev, err)
				return
			}
		}
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

	// Se o arquivo já era WebP e a recompressão não reduziu o tamanho,
	// mantém o original como está (já otimizado).
	if format == "webp" && int64(buf.Len()) >= ev.SizeIn {
		c.moveAsIs(ev, path, snap, "webp already optimized")
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

// processVideo converte um vídeo para WebM (AV1 + Opus) quando reduz o tamanho.
func (c *Converter) processVideo(ev *history.Event, src string, snap Snapshot) {
	if !snap.VideoEnabled {
		c.moveAsIs(ev, src, snap, "video conversion disabled")
		return
	}
	tmp, err := os.CreateTemp(snap.OutputDir, "video-*.webm")
	if err != nil {
		c.fail(ev, err)
		return
	}
	tmpPath := tmp.Name()
	tmp.Close()

	if err := convertVideoWithRetry(src, tmpPath, snap.VideoCRF, snap.VideoPreset); err != nil {
		os.Remove(tmpPath)
		if video.IsIncompleteFile(err) {
			// Arquivo ainda incompleto após as tentativas: libera o nome no
			// watcher para que seja re-enfileirado quando terminar de chegar.
			c.notifyIncomplete(src)
		}
		c.fail(ev, fmt.Errorf("falha na conversão de vídeo: %w", err))
		return
	}
	info, err := os.Stat(tmpPath)
	if err != nil || info.Size() == 0 {
		os.Remove(tmpPath)
		c.fail(ev, fmt.Errorf("conversão de vídeo produziu arquivo vazio"))
		return
	}
	// Mantém o original se a recompressão não reduziu o tamanho.
	if info.Size() >= ev.SizeIn {
		os.Remove(tmpPath)
		c.moveAsIs(ev, src, snap, "video already optimized")
		return
	}
	outPath := uniquePath(filepath.Join(snap.OutputDir, stem(filepath.Base(src))+".webm"))
	if err := moveFile(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		c.fail(ev, err)
		return
	}
	ev.Status = history.StatusDone
	ev.Output = outPath
	ev.SizeOut = info.Size()
	c.emit(ev)

	if err := c.handleOriginal(src, snap); err != nil {
		errEv := &history.Event{
			ID:        newID(),
			File:      filepath.Base(src),
			Source:    src,
			Status:    history.StatusError,
			Error:     "falha ao tratar original: " + err.Error(),
			Timestamp: time.Now(),
		}
		c.emit(errEv)
	}
}

// convertVideoWithRetry tenta a conversão, aguardando o arquivo terminar de
// ser copiado/baixado quando o ffmpeg reportar arquivo incompleto (ex.:
// "moov atom not found"). Retorna o erro final se o arquivo persistir
// inválido após as tentativas.
func convertVideoWithRetry(src, dst string, crf float64, preset int) error {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Aguarda o arquivo parar de crescer antes de tentar de novo.
			fsutil.WaitStable(src, 30*time.Second)
		}
		lastErr = video.Convert(src, dst, crf, preset)
		if lastErr == nil {
			return nil
		}
		if !video.IsIncompleteFile(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

// moveAsIs copia um arquivo sem conversão para a pasta de saída e trata o
// original como nos arquivos convertidos (move para processed/ ou apaga).
func (c *Converter) moveAsIs(ev *history.Event, src string, snap Snapshot, reason string) {
	dest := uniquePath(filepath.Join(snap.OutputDir, filepath.Base(src)))
	if err := copyFile(src, dest); err != nil {
		c.fail(ev, err)
		return
	}
	ev.Status = history.StatusSkipped
	ev.Reason = reason
	ev.Output = dest
	c.emit(ev)

	if err := c.handleOriginal(src, snap); err != nil {
		errEv := &history.Event{
			ID:        newID(),
			File:      filepath.Base(src),
			Source:    src,
			Status:    history.StatusError,
			Error:     "falha ao tratar original: " + err.Error(),
			Timestamp: time.Now(),
		}
		c.emit(errEv)
	}
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
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "webp", nil
	}
	return "", fmt.Errorf("formato desconhecido")
}

// isAnimatedWebP verifica o bit de animação no chunk VP8X do cabeçalho WebP.
func isAnimatedWebP(r io.Reader) (bool, error) {
	head := make([]byte, 21)
	n, err := io.ReadFull(r, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return false, err
	}
	head = head[:n]
	// Cabeçalho RIFF (12 bytes) + fourcc "VP8X" (4) + tamanho do chunk (4)
	// + primeiro byte do payload (flags), onde o bit 0x04 indica animação.
	if len(head) < 21 || string(head[12:16]) != "VP8X" {
		return false, nil
	}
	return head[20]&0x04 != 0, nil
}

// uniquePath retorna um caminho livre, com sufixo numérico se necessário.
func uniquePath(path string) string {
	p := path
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 1; ; i++ {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
		p = fmt.Sprintf("%s-%d%s", base, i, ext)
	}
}

// writeUnique escreve data em path, adicionando sufixo numérico se já existir.
func writeUnique(path string, data []byte) (string, error) {
	p := uniquePath(path)
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
