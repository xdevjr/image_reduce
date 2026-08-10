// Package tray integra o aplicativo à system tray (via D-Bus/AppIndicator).
package tray

import (
	"fyne.io/systray"
)

// Run inicia o menu da system tray em uma goroutine própria.
// onOpen e onQuit são chamados quando os itens de menu são clicados.
func Run(onOpen, onQuit func()) {
	systray.Run(func() {
		systray.SetIcon(iconData)
		systray.SetTitle("Image Reduce")
		systray.SetTooltip("Image Reduce — conversor de imagens para WebP")

		mOpen := systray.AddMenuItem("Abrir janela", "Mostrar histórico e configurações")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Sair", "Encerrar o aplicativo")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					if onOpen != nil {
						onOpen()
					}
				case <-mQuit.ClickedCh:
					if onQuit != nil {
						onQuit()
					}
				}
			}
		}()
	}, func() {
		// onExit: nada a fazer por enquanto.
	})
}