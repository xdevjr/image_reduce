package fsutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWaitStable verifica que WaitStable retorna true quando o arquivo para
// de crescer e false quando continua crescendo além do limite.
func TestWaitStable(t *testing.T) {
	dir := t.TempDir()

	// Arquivo estável desde o início: detectado rapidamente.
	p := filepath.Join(dir, "estavel.bin")
	if err := os.WriteFile(p, []byte("dados"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !WaitStable(p, 2*time.Second) {
		t.Fatal("arquivo estável deveria ser detectado")
	}

	// Arquivo que cresce continuamente: não estabiliza dentro do limite.
	f, err := os.Create(filepath.Join(dir, "crescendo.bin"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				f.Write([]byte("x"))
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()
	if WaitStable(f.Name(), 300*time.Millisecond) {
		close(stop)
		<-done
		t.Fatal("arquivo em crescimento não deveria estabilizar")
	}
	close(stop)
	<-done
}