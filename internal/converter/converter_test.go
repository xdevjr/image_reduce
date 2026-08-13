package converter

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/chai2010/webp"

	"image_reduce/internal/config"
	"image_reduce/internal/history"
)

func newTestCfg(t *testing.T, deleteOriginal bool) (*config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(in, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		WatchDir:       in,
		OutputDir:      out,
		MaxConcurrent:  2,
		Quality:        90,
		DeleteOriginal: deleteOriginal,
	}
	return cfg, dir
}

func writePNG(t *testing.T, path string, withAlpha bool) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			a := uint8(255)
			if withAlpha {
				a = 128
			}
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: a})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestConvertPNGWithAlpha(t *testing.T) {
	cfg, dir := newTestCfg(t, false)
	src := filepath.Join(cfg.WatchDir, "test.png")
	writePNG(t, src, true)

	h := history.New(filepath.Join(dir, "history.jsonl"))
	events := make(chan *history.Event, 10)
	c := New(cfg, h, events)
	c.Process(src)

	// .webp criado na pasta de saída
	webpPath := filepath.Join(cfg.OutputDir, "test.webp")
	if _, err := os.Stat(webpPath); err != nil {
		t.Fatalf("webp não criado: %v", err)
	}
	// original movido para processed/
	if _, err := os.Stat(filepath.Join(cfg.WatchDir, "processed", "test.png")); err != nil {
		t.Fatalf("original não movido para processed/: %v", err)
	}
	// evento done registrado
	var done bool
	for _, e := range h.List() {
		if e.Status == history.StatusDone {
			done = true
		}
	}
	if !done {
		t.Fatal("evento done não encontrado")
	}
}

func TestDeleteOriginal(t *testing.T) {
	cfg, dir := newTestCfg(t, true)
	src := filepath.Join(cfg.WatchDir, "test.png")
	writePNG(t, src, false)

	h := history.New(filepath.Join(dir, "history.jsonl"))
	c := New(cfg, h, nil)
	c.Process(src)

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("original deveria ter sido apagado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.OutputDir, "test.webp")); err != nil {
		t.Fatalf("webp não criado: %v", err)
	}
}

func TestSkipAnimatedGif(t *testing.T) {
	cfg, dir := newTestCfg(t, false)
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}}
	frames := []*image.Paletted{
		image.NewPaletted(image.Rect(0, 0, 2, 2), pal),
		image.NewPaletted(image.Rect(0, 0, 2, 2), pal),
	}
	src := filepath.Join(cfg.WatchDir, "anim.gif")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(f, &gif.GIF{Image: frames, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	f.Close()

	h := history.New(filepath.Join(dir, "history.jsonl"))
	c := New(cfg, h, nil)
	c.Process(src)

	// gif copiado sem conversão para a saída
	if _, err := os.Stat(filepath.Join(cfg.OutputDir, "anim.gif")); err != nil {
		t.Fatalf("gif não copiado para saída: %v", err)
	}
	// original movido para processed/
	if _, err := os.Stat(filepath.Join(cfg.WatchDir, "processed", "anim.gif")); err != nil {
		t.Fatalf("original não movido para processed/: %v", err)
	}
	// não deve gerar webp
	if _, err := os.Stat(filepath.Join(cfg.OutputDir, "anim.webp")); !os.IsNotExist(err) {
		t.Fatal("gif animado não deveria gerar webp")
	}
	var skipped bool
	for _, e := range h.List() {
		if e.Status == history.StatusSkipped && e.Reason == "animated gif" {
			skipped = true
		}
	}
	if !skipped {
		t.Fatal("evento skipped (animated gif) não encontrado")
	}
}

func TestSkipNonImage(t *testing.T) {
	cfg, dir := newTestCfg(t, false)
	src := filepath.Join(cfg.WatchDir, "nota.txt")
	if err := os.WriteFile(src, []byte("olá mundo"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := history.New(filepath.Join(dir, "history.jsonl"))
	c := New(cfg, h, nil)
	c.Process(src)

	if _, err := os.Stat(filepath.Join(cfg.OutputDir, "nota.txt")); err != nil {
		t.Fatalf("arquivo não-imagem não copiado para saída: %v", err)
	}
	// original movido para processed/ (cópia na pasta de processados)
	if _, err := os.Stat(filepath.Join(cfg.WatchDir, "processed", "nota.txt")); err != nil {
		t.Fatalf("original não movido para processed/: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("original não deveria permanecer na pasta monitorada")
	}
	var skipped bool
	for _, e := range h.List() {
		if e.Status == history.StatusSkipped && e.Reason == "not an image" {
			skipped = true
		}
	}
	if !skipped {
		t.Fatal("evento skipped (not an image) não encontrado")
	}
}

func TestUniqueNameCollision(t *testing.T) {
	cfg, dir := newTestCfg(t, false)
	// pré-cria um webp com o mesmo nome
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "test.webp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(cfg.WatchDir, "test.png")
	writePNG(t, src, false)

	h := history.New(filepath.Join(dir, "history.jsonl"))
	c := New(cfg, h, nil)
	c.Process(src)

	if _, err := os.Stat(filepath.Join(cfg.OutputDir, "test-1.webp")); err != nil {
		t.Fatalf("webp com sufixo não criado: %v", err)
	}
}

func TestConvertWebP(t *testing.T) {
	cfg, dir := newTestCfg(t, false)
	cfg.Quality = 50

	// imagem ruidosa codificada em WebP qualidade 100 (original "pesado")
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	rnd := rand.New(rand.NewSource(42))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(rnd.Intn(256)),
				G: uint8(rnd.Intn(256)),
				B: uint8(rnd.Intn(256)),
				A: 255,
			})
		}
	}
	var srcBuf bytes.Buffer
	if err := webp.Encode(&srcBuf, img, &webp.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(cfg.WatchDir, "foto.webp")
	if err := os.WriteFile(src, srcBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	h := history.New(filepath.Join(dir, "history.jsonl"))
	c := New(cfg, h, nil)
	c.Process(src)

	// webp recompactado na saída deve ser menor que o original
	out := filepath.Join(cfg.OutputDir, "foto.webp")
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("webp não criado na saída: %v", err)
	}
	if int64(len(data)) >= int64(srcBuf.Len()) {
		t.Fatalf("webp recompactado deveria ser menor: %d >= %d", len(data), srcBuf.Len())
	}
	// original movido para processed/
	if _, err := os.Stat(filepath.Join(cfg.WatchDir, "processed", "foto.webp")); err != nil {
		t.Fatalf("original não movido para processed/: %v", err)
	}
	// evento done registrado
	var done bool
	for _, e := range h.List() {
		if e.Status == history.StatusDone {
			done = true
		}
	}
	if !done {
		t.Fatal("evento done não encontrado")
	}
}

func TestSkipAnimatedWebP(t *testing.T) {
	cfg, dir := newTestCfg(t, false)
	// cabeçalho WebP com chunk VP8X e flag de animação (0x04) ativa
	buf := make([]byte, 30)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(buf)-8))
	copy(buf[8:12], "WEBP")
	copy(buf[12:16], "VP8X")
	binary.LittleEndian.PutUint32(buf[16:20], 10)
	buf[20] = 0x04 // flags: bit de animação
	src := filepath.Join(cfg.WatchDir, "anim.webp")
	if err := os.WriteFile(src, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	h := history.New(filepath.Join(dir, "history.jsonl"))
	c := New(cfg, h, nil)
	c.Process(src)

	if _, err := os.Stat(filepath.Join(cfg.OutputDir, "anim.webp")); err != nil {
		t.Fatalf("webp animado não copiado para a saída: %v", err)
	}
	// original movido para processed/
	if _, err := os.Stat(filepath.Join(cfg.WatchDir, "processed", "anim.webp")); err != nil {
		t.Fatalf("original não movido para processed/: %v", err)
	}
	var skipped bool
	for _, e := range h.List() {
		if e.Status == history.StatusSkipped && e.Reason == "animated webp" {
			skipped = true
		}
	}
	if !skipped {
		t.Fatal("evento skipped (animated webp) não encontrado")
	}
}

func TestVideoInvalidFile(t *testing.T) {
	cfg, dir := newTestCfg(t, false)
	cfg.VideoEnabled = true
	cfg.VideoCRF = 32
	cfg.VideoPreset = 6
	src := filepath.Join(cfg.WatchDir, "filme.mp4")
	if err := os.WriteFile(src, []byte("dados inválidos"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := history.New(filepath.Join(dir, "history.jsonl"))
	c := New(cfg, h, nil)
	c.Process(src)

	// original não deve ser movido nem apagado em caso de falha
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("original não deveria ter sido removido: %v", err)
	}
	var hasError bool
	for _, e := range h.List() {
		if e.Status == history.StatusError && e.Error != "" {
			hasError = true
		}
	}
	if !hasError {
		t.Fatal("evento error não encontrado")
	}
}
