//go:build linux

// Seletor de pasta nativo via zenity/kdialog (processos externos).
// Roda em um processo separado, então não bloqueia a main thread do GTK
// (diferente de gtk_dialog_run, que causava deadlock dentro do webview).
package ui

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// selectFolder abre o seletor de pasta nativo e retorna o caminho escolhido,
// ou "" se o usuário cancelar ou nenhum seletor estiver disponível.
func selectFolder() string {
	// Tenta zenity (GNOME)
	if out, err := exec.Command("zenity", "--file-selection", "--directory",
		"--title=Selecionar pasta").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	// Tenta kdialog (KDE)
	if out, err := exec.Command("kdialog", "--getexistingdirectory",
		"Selecionar pasta").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// openFolder abre a pasta no gerenciador de arquivos padrão via xdg-open.
// Retorna false se o caminho for inválido ou o comando falhar.
func openFolder(path string) bool {
	if path == "" {
		return false
	}
	return exec.Command("xdg-open", path).Start() == nil
}

// openFile abre o arquivo no aplicativo padrão via xdg-open.
// Retorna false se o caminho for inválido ou o comando falhar.
func openFile(path string) bool {
	if path == "" {
		return false
	}
	return exec.Command("xdg-open", path).Start() == nil
}

// showInFolder abre o gerenciador de arquivos com o arquivo selecionado.
// Tenta os gerenciadores mais comuns (GNOME, KDE, XFCE, LXQt); se nenhum
// estiver disponível, abre apenas a pasta via xdg-open.
func showInFolder(path string) bool {
	if path == "" {
		return false
	}
	dir := filepath.Dir(path)
	for _, cand := range [][]string{
		{"nautilus", "--select", path},
		{"dolphin", "--select", path},
		{"thunar", path},
		{"pcmanfm", "--select", path},
	} {
		if _, err := exec.LookPath(cand[0]); err != nil {
			continue
		}
		if exec.Command(cand[0], cand[1:]...).Start() == nil {
			return true
		}
	}
	return exec.Command("xdg-open", dir).Start() == nil
}
