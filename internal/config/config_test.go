package config

import "testing"

func TestIgnorePatterns(t *testing.T) {
	cfg := &Config{IgnorePatterns: ".*, *.rar, foto.png"}
	cases := []struct {
		path string
		want bool
	}{
		{"/tmp/in/.gitignore", true},
		{"/tmp/in/arquivo.rar", true},
		{"/tmp/in/foto.png", true},
		{"/tmp/in/foto.jpg", false},
		{"/tmp/in/arquivo.RAR", false},
	}
	for _, c := range cases {
		if got := cfg.IsIgnored(c.path); got != c.want {
			t.Errorf("IsIgnored(%q) = %v, esperado %v", c.path, got, c.want)
		}
	}
}

func TestDefaultIgnoresHiddenFiles(t *testing.T) {
	cfg := Default()
	if !cfg.IsIgnored(".bashrc") {
		t.Fatal("configuração padrão deveria ignorar arquivos ocultos")
	}
	if cfg.IsIgnored("foto.png") {
		t.Fatal("configuração padrão não deveria ignorar arquivos comuns")
	}
}

func TestVideoDefaults(t *testing.T) {
	cfg := Default()
	if !cfg.VideoEnabled {
		t.Fatal("conversão de vídeo deveria estar habilitada por padrão")
	}
	if cfg.VideoCRF != 32 {
		t.Fatalf("CRF padrão = %v, esperado 32", cfg.VideoCRF)
	}
	cfg.VideoCRF = 0
	cfg.VideoPreset = 99
	cfg.EnsureDefaults()
	if cfg.VideoCRF != 32 {
		t.Fatalf("CRF inválido não corrigido: %v", cfg.VideoCRF)
	}
	if cfg.VideoPreset != 6 {
		t.Fatalf("preset inválido não corrigido: %d", cfg.VideoPreset)
	}
}

func TestNotificationDefaults(t *testing.T) {
	cfg := Default()
	if !cfg.NotificationsEnabled {
		t.Fatal("notificações deveriam estar habilitadas por padrão")
	}
	if !cfg.NotifyOnDone {
		t.Fatal("notificação de conclusão deveria estar habilitada por padrão")
	}
	if !cfg.NotifyOnError {
		t.Fatal("notificação de erro deveria estar habilitada por padrão")
	}
	if cfg.NotifyOnSkipped {
		t.Fatal("notificação de arquivo ignorado deveria estar desligada por padrão")
	}
}

func TestHasJSONKey(t *testing.T) {
	if !hasJSONKey([]byte(`{"notifications_enabled": true}`), "notifications_enabled") {
		t.Fatal("chave existente não foi detectada")
	}
	if hasJSONKey([]byte(`{"watch_dir": "/tmp"}`), "notifications_enabled") {
		t.Fatal("chave ausente foi detectada como presente")
	}
	if hasJSONKey([]byte(`não é json`), "notifications_enabled") {
		t.Fatal("JSON inválido deveria retornar false")
	}
}
