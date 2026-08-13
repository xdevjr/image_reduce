// Package tray integra o aplicativo à system tray (via D-Bus/AppIndicator).
package tray

import (
	"fyne.io/systray"
)

// Run inicia o menu da system tray em uma goroutine própria.
// onToggle é chamado no clique com botão esquerdo no ícone; onOpenHistory,
// onOpenConfig e onQuit são chamados quando os itens de menu são clicados.
func Run(onToggle, onOpenHistory, onOpenConfig, onQuit func()) {
	systray.Run(func() {
		systray.SetIcon(iconData)
		systray.SetTitle("Image Reduce")
		systray.SetTooltip("Image Reduce — conversor de imagens para WebP")
		// Clique com botão esquerdo alterna a janela (abre/fecha).
		systray.SetOnTapped(func() {
			if onToggle != nil {
				onToggle()
			}
		})

		mHistory := systray.AddMenuItem("Histórico", "Mostrar histórico de conversões")
		mConfig := systray.AddMenuItem("Configurações", "Abrir as configurações do aplicativo")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Sair", "Encerrar o aplicativo")

		go func() {
			for {
				select {
				case <-mHistory.ClickedCh:
					if onOpenHistory != nil {
						onOpenHistory()
					}
				case <-mConfig.ClickedCh:
					if onOpenConfig != nil {
						onOpenConfig()
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
