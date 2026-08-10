package converter

import (
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"

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

	// gif movido sem conversão para a saída
	if _, err := os.Stat(filepath.Join(cfg.OutputDir, "anim.gif")); err != nil {
		t.Fatalf("gif não movido para saída: %v", err)
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
		t.Fatalf("arquivo não-imagem não movido: %v", err)
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