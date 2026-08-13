// image_reduce: converte imagens para WebP automaticamente a partir de uma
// pasta monitorada, rodando na system tray.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"image_reduce/internal/app"
	"image_reduce/internal/config"
	"image_reduce/internal/tray"
	"image_reduce/internal/ui"
)

// caminho (relativo à home) do log e do arquivo de pid usados pelo `start`
const cacheBase = ".cache/image_reduce"

// cacheDir retorna o diretório de cache do app (~/.cache/image_reduce).
func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, cacheBase), nil
}

// startDaemon inicia o app em segundo plano quando executado com `start`, sem
// prender o terminal: reexecuta o próprio binário em uma nova sessão (Setsid),
// redirecionando a saída para um log, grava o pid e encerra o processo atual.
// O processo filho (IMAGE_REDUCE_DETACHED=1) segue para a execução normal.
func startDaemon() {
	if os.Getenv("IMAGE_REDUCE_DETACHED") == "1" {
		return // já é o processo em segundo plano: segue execução normal
	}

	dir, err := cacheDir()
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("start: %v", err)
	}
	logPath := filepath.Join(dir, "image_reduce.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	defer f.Close()

	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("start: %v", err)
	}
	// O filho é relançado com `run` para executar em primeiro plano.
	cmd := exec.Command(exe, "run")
	cmd.Env = append(os.Environ(), "IMAGE_REDUCE_DETACHED=1")
	cmd.Stdin = nil
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		log.Fatalf("start: não foi possível iniciar em segundo plano: %v", err)
	}

	pidPath := filepath.Join(dir, "image_reduce.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644); err != nil {
		log.Printf("start: aviso: não foi possível gravar o pid: %v", err)
	}

	fmt.Printf("image_reduce rodando em segundo plano (pid %d).\n", cmd.Process.Pid)
	fmt.Printf("Log: %s\nPara parar: image_reduce stop\n", logPath)
	os.Exit(0)
}

// stopDaemon encerra o processo em segundo plano iniciado por `start`, usando
// o pid gravado em ~/.cache/image_reduce/image_reduce.pid.
func stopDaemon() {
	dir, err := cacheDir()
	if err != nil {
		log.Fatalf("stop: %v", err)
	}
	pidPath := filepath.Join(dir, "image_reduce.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("image_reduce não está rodando em segundo plano.")
		os.Exit(0)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		log.Fatalf("stop: pid inválido em %s: %v", pidPath, err)
	}

	// Se o processo já não existe, apenas limpa o pid e sai.
	if syscall.Kill(pid, 0) != nil {
		fmt.Printf("image_reduce (pid %d) não está mais em execução.\n", pid)
		_ = os.Remove(pidPath)
		os.Exit(0)
	}

	fmt.Printf("Encerrando image_reduce (pid %d)...\n", pid)
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		log.Fatalf("stop: não foi possível encerrar o processo %d: %v", pid, err)
	}

	// Aguarda até ~2s pelo encerramento; se continuar vivo, força SIGKILL.
	for i := 0; i < 20; i++ {
		if syscall.Kill(pid, 0) != nil {
			_ = os.Remove(pidPath)
			fmt.Println("image_reduce encerrado.")
			os.Exit(0)
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	_ = os.Remove(pidPath)
	fmt.Println("image_reduce encerrado (SIGKILL).")
	os.Exit(0)
}

// printHelp exibe a ajuda de uso do binário.
func printHelp() {
	fmt.Print(`image_reduce — converte imagens para WebP automaticamente

Uso:
  image_reduce run     Executa em primeiro plano (prende o terminal)
  image_reduce start   Inicia em segundo plano (não prende o terminal)
  image_reduce stop    Encerra a execução em segundo plano
  image_reduce help    Mostra esta ajuda

Quando executado sem comando, esta ajuda é exibida.
`)
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "run":
			// sem ação: segue para a execução em primeiro plano abaixo
		case "start":
			startDaemon()
		case "stop":
			stopDaemon()
		case "help", "-h", "--help":
			printHelp()
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "comando desconhecido: %s\n\n", os.Args[1])
			printHelp()
			os.Exit(2)
		}
	} else {
		// Sem comando: mostra a ajuda.
		printHelp()
		os.Exit(0)
	}

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
	go tray.Run(u.Toggle, u.OpenHistory, u.OpenConfig, u.Quit)
	u.Run()
}
