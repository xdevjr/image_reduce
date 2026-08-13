// Package video converte vídeos para WebM (AV1 + Opus) usando o ffmpeg.
package video

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// extensions reconhecidas como vídeo (case-insensitive).
var extensions = map[string]struct{}{
	".mp4": {}, ".mov": {}, ".mkv": {}, ".avi": {}, ".m4v": {},
	".wmv": {}, ".flv": {}, ".webm": {}, ".mpg": {}, ".mpeg": {},
	".3gp": {}, ".mts": {}, ".m2ts": {},
}

// IsVideo informa se o arquivo tem extensão de vídeo.
func IsVideo(path string) bool {
	_, ok := extensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

var (
	detectOnce sync.Once
	ffmpegPath string
	encoder    string
	detectErr  error
)

// detectEncoder descobre o ffmpeg e o melhor encoder AV1 disponível
// (SVT-AV1 primeiro, senão libaom-av1). Roda apenas uma vez.
func detectEncoder() (string, error) {
	detectOnce.Do(func() {
		ffmpegPath, detectErr = exec.LookPath("ffmpeg")
		if detectErr != nil {
			detectErr = fmt.Errorf("ffmpeg não encontrado; instale com: sudo apt install ffmpeg")
			return
		}
		out, err := exec.Command(ffmpegPath, "-hide_banner", "-encoders").Output()
		if err != nil {
			detectErr = err
			return
		}
		text := string(out)
		for _, enc := range []string{"libsvtav1", "libaom-av1"} {
			if strings.Contains(text, " "+enc+" ") {
				encoder = enc
				return
			}
		}
		detectErr = fmt.Errorf("nenhum encoder AV1 (libsvtav1/libaom-av1) encontrado no ffmpeg")
	})
	if detectErr != nil {
		return "", detectErr
	}
	return encoder, nil
}

// Available informa se há ffmpeg com encoder AV1 utilizável no sistema.
func Available() bool {
	_, err := detectEncoder()
	return err == nil
}

// IsIncompleteFile informa se o erro do ffmpeg indica um arquivo ainda em
// escrita/cópia (ex.: "moov atom not found") em vez de um vídeo corrompido.
func IsIncompleteFile(err error) bool {
	msg := err.Error()
	for _, frag := range []string{
		"moov atom not found",
		"Invalid data found when processing input",
		"Error opening input",
	} {
		if strings.Contains(msg, frag) {
			return true
		}
	}
	return false
}

// Convert converte src para WebM (AV1 + Opus) gravando em dst.
func Convert(src, dst string, crf float64, preset int) error {
	enc, err := detectEncoder()
	if err != nil {
		return err
	}
	if crf < 1 {
		crf = 1
	}
	if crf > 63 {
		crf = 63
	}

	args := []string{"-y", "-hide_banner", "-loglevel", "error",
		"-i", src, "-sn", "-dn"}
	switch enc {
	case "libsvtav1":
		if preset < 0 {
			preset = 0
		}
		if preset > 13 {
			preset = 13
		}
		args = append(args, "-c:v", "libsvtav1",
			"-crf", fmt.Sprintf("%.0f", crf),
			"-preset", fmt.Sprintf("%d", preset))
	case "libaom-av1":
		if preset < 0 {
			preset = 0
		}
		if preset > 8 {
			preset = 8
		}
		args = append(args, "-c:v", "libaom-av1",
			"-crf", fmt.Sprintf("%.0f", crf),
			"-cpu-used", fmt.Sprintf("%d", preset))
	}
	args = append(args, "-c:a", "libopus", "-b:a", "96k", dst)

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
