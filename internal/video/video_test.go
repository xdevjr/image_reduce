package video

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// generateTestVideo cria um MP4 de 1s usando o lavfi do ffmpeg.
func generateTestVideo(path string) error {
	cmd := exec.Command("ffmpeg", "-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", path)
	return cmd.Run()
}

func TestIsVideo(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"video.mp4", true},
		{"filme.MKV", true},
		{"clipe.webm", true},
		{"gravação.mov", true},
		{"foto.png", false},
		{"nota.txt", false},
	}
	for _, c := range cases {
		if got := IsVideo(c.name); got != c.want {
			t.Errorf("IsVideo(%q) = %v, esperado %v", c.name, got, c.want)
		}
	}
}

func TestConvertWebM(t *testing.T) {
	if !Available() {
		t.Skip("ffmpeg com encoder AV1 não disponível no ambiente")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "in.mp4")
	// Gera um vídeo de teste de 1s (64x64) com ffmpeg usando lavfi.
	// Depende de um encoder H.264 disponível; senão, pula o teste.
	if err := generateTestVideo(src); err != nil {
		t.Skipf("não foi possível gerar vídeo de teste: %v", err)
	}
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "out.webm")
	if err := Convert(src, dst, 50, 10); err != nil {
		t.Fatalf("conversão falhou: %v", err)
	}
	out, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("webm não criado: %v", err)
	}
	if out.Size() == 0 {
		t.Fatal("webm criado vazio")
	}
	if out.Size() >= info.Size() {
		t.Fatalf("webm deveria ser menor que o original: %d >= %d", out.Size(), info.Size())
	}
}

func TestIsIncompleteFile(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"moov atom ausente", fmt.Errorf("exit status 183: moov atom not found"), true},
		{"dados inválidos na entrada", fmt.Errorf("exit status 183: Invalid data found when processing input"), true},
		{"erro ao abrir entrada", fmt.Errorf("exit status 183: Error opening input file x.mp4"), true},
		{"erro de encoder", fmt.Errorf("exit status 1: encoder not found"), false},
		{"arquivo não encontrado", fmt.Errorf("exit status 1: No such file or directory"), false},
	}
	for _, c := range cases {
		if got := IsIncompleteFile(c.err); got != c.want {
			t.Errorf("IsIncompleteFile(%q) = %v, esperado %v", c.err, got, c.want)
		}
	}
}
