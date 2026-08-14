package notify

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestImageDataSignature(t *testing.T) {
	v := dbus.MakeVariant(imageData{})
	if got := v.Signature().String(); got != "(iiibiiay)" {
		t.Fatalf("assinatura = %s, esperado (iiibiiay)", got)
	}
}

func TestThumbnailData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "img.png")
	img := image.NewRGBA(image.Rect(0, 0, 256, 128))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{200, 100, 50, 255}), image.Point{}, draw.Src)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()

	data, ok := thumbnailData(path)
	if !ok {
		t.Fatal("thumbnailData falhou para PNG válido")
	}
	// maior lado reduzido para maxThumbSize mantendo a proporção (256x128 → 128x64)
	if data.Width != 128 || data.Height != 64 {
		t.Fatalf("dimensões = %dx%d, esperado 128x64", data.Width, data.Height)
	}
	if data.Channels != 4 || data.BitsPerSample != 8 || !data.HasAlpha {
		t.Fatal("formato de pixels incorreto")
	}
	if data.Rowstride != data.Width*4 {
		t.Fatalf("rowstride = %d, esperado %d", data.Rowstride, data.Width*4)
	}
	if len(data.Data) != int(data.Rowstride*data.Height) {
		t.Fatalf("tamanho dos dados = %d, esperado %d", len(data.Data), data.Rowstride*data.Height)
	}
}

func TestThumbnailDataNonImage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nota.txt")
	if err := os.WriteFile(path, []byte("olá"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := thumbnailData(path); ok {
		t.Fatal("thumbnailData deveria falhar para arquivo não-imagem")
	}
	if _, ok := thumbnailData(""); ok {
		t.Fatal("thumbnailData deveria falhar para caminho vazio")
	}
}

func TestScaleImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	scaled := scaleImage(img, 128)
	b := scaled.Bounds()
	if b.Dx() != 128 || b.Dy() != 64 {
		t.Fatalf("escala = %dx%d, esperado 128x64", b.Dx(), b.Dy())
	}
	// imagem menor que o limite não é alterada
	small := image.NewRGBA(image.Rect(0, 0, 50, 50))
	if got := scaleImage(small, 128); got != small {
		t.Fatal("imagem pequena não deveria ser redimensionada")
	}
}

func TestToImageDataPixelOrder(t *testing.T) {
	// 1x1 com cor conhecida: os bytes devem ser (r, g, b, a) por pixel,
	// como o image.RGBA e a biblioteca de referência esiqveland/notify.
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{200, 100, 50, 255})
	data := toImageData(img)
	if len(data.Data) != 4 {
		t.Fatalf("dados = %d bytes, esperado 4", len(data.Data))
	}
	want := []byte{200, 100, 50, 255}
	for i := range want {
		if data.Data[i] != want[i] {
			t.Fatalf("byte %d = %d, esperado %d (formato deve ser RGBA)", i, data.Data[i], want[i])
		}
	}
}

func TestBadgeIcon(t *testing.T) {
	thumb := image.NewRGBA(image.Rect(0, 0, 128, 128))
	draw.Draw(thumb, thumb.Bounds(), image.NewUniform(color.RGBA{255, 255, 255, 255}), image.Point{}, draw.Src)
	badged := badgeIcon(thumb, typeIcon(KindDone))
	b := badged.Bounds()
	if b.Dx() != 128 || b.Dy() != 128 {
		t.Fatalf("dimensões = %dx%d, esperado 128x128", b.Dx(), b.Dy())
	}
	// canto superior esquerdo permanece branco (sem selo)
	if got := color.RGBAModel.Convert(badged.At(2, 2)).(color.RGBA); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("canto superior esquerdo = %v, esperado branco", got)
	}
	// o canto superior direito deixou de ser branco (selo desenhado)
	if got := color.RGBAModel.Convert(badged.At(110, 10)).(color.RGBA); got == (color.RGBA{255, 255, 255, 255}) {
		t.Fatal("o selo do tipo não foi desenhado no canto superior direito")
	}
}

func TestNotificationImage(t *testing.T) {
	// sem miniatura: retorna apenas o ícone do tipo
	img := notificationImage(Options{Kind: KindError, ImagePath: ""})
	if img == nil {
		t.Fatal("notificationImage retornou nil sem miniatura")
	}
	if b := img.Bounds(); b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("sem miniatura: dimensões = %dx%d, esperado %dx%d", b.Dx(), b.Dy(), iconSize, iconSize)
	}
	// com miniatura: retorna a miniatura com o selo (dimensões da miniatura)
	dir := t.TempDir()
	path := filepath.Join(dir, "img.png")
	src := image.NewRGBA(image.Rect(0, 0, 256, 128))
	draw.Draw(src, src.Bounds(), image.NewUniform(color.RGBA{200, 100, 50, 255}), image.Point{}, draw.Src)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, src); err != nil {
		t.Fatal(err)
	}
	f.Close()
	img = notificationImage(Options{Kind: KindDone, ImagePath: path})
	if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 64 {
		t.Fatalf("com miniatura: dimensões = %dx%d, esperado 128x64", b.Dx(), b.Dy())
	}
}

func TestTypeIcon(t *testing.T) {
	for _, kind := range []Kind{KindDone, KindError, KindSkipped} {
		img := typeIcon(kind)
		b := img.Bounds()
		if b.Dx() != iconSize || b.Dy() != iconSize {
			t.Fatalf("ícone %v = %dx%d, esperado %dx%d", kind, b.Dx(), b.Dy(), iconSize, iconSize)
		}
		// um ponto no topo do círculo deve ter a cor do tipo (o centro é
		// coberto pelo glifo em alguns tipos)
		c := color.RGBAModel.Convert(img.At(iconSize/2, 6)).(color.RGBA)
		want := kindColor(kind)
		if c != want {
			t.Fatalf("ícone %v: cor = %v, esperado %v", kind, c, want)
		}
	}
}