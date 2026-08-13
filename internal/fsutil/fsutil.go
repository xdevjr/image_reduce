// Package fsutil reúne utilitários de sistema de arquivos compartilhados
// entre os pacotes do aplicativo.
package fsutil

import (
	"os"
	"time"
)

// WaitStable aguarda o arquivo parar de crescer (cópia/baixamento concluído),
// verificando o tamanho a cada 200ms. Retorna true se o tamanho ficou estável
// por duas verificações consecutivas dentro do limite de tempo.
func WaitStable(path string, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	var lastSize int64 = -1
	stable := 0
	for time.Now().Before(deadline) {
		info, err := os.Stat(path)
		if err != nil {
			return false
		}
		if info.Size() == lastSize && lastSize >= 0 {
			stable++
			if stable >= 2 {
				return true
			}
		} else {
			stable = 0
		}
		lastSize = info.Size()
		time.Sleep(200 * time.Millisecond)
	}
	return false
}