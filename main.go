// image_reduce: converte imagens para WebP automaticamente a partir de uma
// pasta monitorada, rodando na system tray.
package main

import (
	"log"
	"os"

	"image_reduce/internal/app"
	"image_reduce/internal/config"
	"image_reduce/internal/tray"
	"image_reduce/internal/ui"
)

func main() {
	// O GTK3/webkit2gtk-4.1 pode apresentar falhas de renderização em alguns
	// ambientes Wayland. Forçamos o backend X11 e desabilitamos aceleração
	// de hardware problemática (GBM/DMA-BUF) para garantir que a janela
	// webview abra corretamente.
	os.Setenv("GDK_BACKEND", "x11")
	os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
	os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	a, err := app.New(cfg)
	if err != nil {
		log.Fatalf("app: %v", err)
	}
	if err := a.Start(); err != nil {
		log.Fatalf("start: %v", err)
	}
	log.Printf("image_reduce rodando. Monitorando %s -> %s", cfg.WatchDir, cfg.OutputDir)

	u := ui.New(a)
	go tray.Run(u.Open, u.Quit)
	u.Run()
}