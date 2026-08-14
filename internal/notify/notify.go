// Package notify envia notificações do sistema via D-Bus
// (org.freedesktop.Notifications), o protocolo padrão de notificações
// dos desktops Linux (GNOME, KDE, XFCE, etc.).
package notify

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	_ "image/gif"
	_ "image/jpeg"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/chai2010/webp"
	"github.com/godbus/dbus/v5"
	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
)

const (
	// appName identifica o aplicativo no daemon de notificações.
	appName = "image_reduce"
	// callTimeout limita a espera pela resposta do daemon de notificações,
	// evitando que a goroutine fique presa se o daemon não responder.
	callTimeout = 3 * time.Second
	// displayTimeout é o tempo (ms) que a notificação permanece visível.
	displayTimeout = 5000
	// maxThumbSize é o maior lado (px) da miniatura enviada na notificação.
	maxThumbSize = 128
	// iconSize é o tamanho (px) do ícone colorido por tipo.
	iconSize = 64
	// badgeSize é o tamanho (px) do selo do tipo sobre a miniatura.
	badgeSize = 28
)

// Kind identifica o tipo de notificação (define ícone e cor).
type Kind int

const (
	// KindDone notifica uma conversão concluída.
	KindDone Kind = iota
	// KindError notifica uma falha na conversão.
	KindError
	// KindSkipped notifica um arquivo ignorado.
	KindSkipped
)

// Options descreve uma notificação a ser exibida.
type Options struct {
	Title string
	Body  string
	Kind  Kind
	// ImagePath, se informado e for uma imagem, gera a miniatura exibida
	// na notificação.
	ImagePath string
}

// Notifier envia notificações do sistema de forma best-effort. A conexão
// D-Bus é feita sob demanda e reutilizada; falhas (sem session bus, sem
// daemon de notificações) são silenciosas e nunca interrompem o pipeline.
type Notifier struct {
	mu        sync.Mutex
	conn      *dbus.Conn
	iconPaths map[Kind]string
}

// New cria um Notifier sem conexão ativa (a conexão é estabelecida na
// primeira notificação).
func New() *Notifier {
	return &Notifier{iconPaths: make(map[Kind]string)}
}

// Notify envia uma notificação descrita por opts. A chamada é assíncrona:
// retorna imediatamente e nunca bloqueia o chamador; erros são descartados.
func (n *Notifier) Notify(opts Options) {
	go n.send(opts)
}

func (n *Notifier) send(opts Options) {
	conn := n.sessionConn()
	if conn == nil {
		return
	}
	err := n.notify(conn, opts, true)
	if err == nil {
		return
	}
	// O daemon rejeitou a notificação com hints (miniatura/ícone): tenta
	// uma notificação simples para garantir que o aviso chegue ao usuário.
	log.Printf("notify: falha com hints (%v); reenviando sem hints", err)
	conn = n.sessionConn() // reconecta (a conexão anterior foi descartada)
	if conn == nil {
		return
	}
	if err := n.notify(conn, opts, false); err != nil {
		log.Printf("notify: falha mesmo sem hints: %v", err)
	}
}

// notify envia a notificação via D-Bus. Quando fancy é true, inclui a
// miniatura (image-data) e o ícone colorido por tipo; caso contrário,
// envia uma notificação simples. Em falha, descarta a conexão para
// reconexão na próxima tentativa.
func (n *Notifier) notify(conn *dbus.Conn, opts Options, fancy bool) error {
	hints := map[string]dbus.Variant{}
	icon := ""
	if fancy {
		// Sempre envia uma imagem (miniatura com o selo do tipo, ou apenas
		// o ícone quando não há miniatura) — assim o tipo aparece mesmo em
		// daemons que não renderizam o app_icon (ex.: noctalia).
		hints["image-data"] = dbus.MakeVariant(toImageData(notificationImage(opts)))
		icon = n.iconPath(opts.Kind)
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.CallWithContext(ctx, "org.freedesktop.Notifications.Notify", 0,
		appName,               // nome do aplicativo
		uint32(0),             // id de notificação a substituir (0 = nova)
		icon,                  // ícone colorido por tipo (ou vazio no fallback)
		opts.Title,            // título
		opts.Body,             // corpo
		[]string{},            // ações (nenhuma)
		hints,                 // hints (miniatura, quando disponível)
		int32(displayTimeout), // tempo de exibição em ms
	)
	if call.Err != nil {
		// Conexão possivelmente morta: descarta para reconectar na próxima.
		n.drop()
		return call.Err
	}
	return nil
}

// sessionConn retorna a conexão com o session bus, conectando sob demanda.
func (n *Notifier) sessionConn() *dbus.Conn {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn != nil {
		return n.conn
	}
	// IMPORTANTE: WithContext define a VIDA da conexão — um contexto com
	// timeout fecharia a conexão automaticamente após o prazo, matando as
	// notificações seguintes (erro "connection closed by user"). Usamos
	// Background() para a conexão durar enquanto o app viver.
	conn, err := dbus.SessionBusPrivate(dbus.WithContext(context.Background()))
	if err != nil {
		return nil
	}
	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return nil
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil
	}
	n.conn = conn
	return conn
}

// drop fecha e descarta a conexão atual, forçando reconexão na próxima
// notificação.
func (n *Notifier) drop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}
}

// iconPath retorna o caminho do ícone colorido do tipo, gerando e
// cacheando um PNG em disco na primeira vez (o daemon lê o arquivo).
// Retorna "" se não for possível gerar o arquivo.
func (n *Notifier) iconPath(kind Kind) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if p, ok := n.iconPaths[kind]; ok {
		return p
	}
	p := filepath.Join(os.TempDir(), fmt.Sprintf("image_reduce_%s.png", kindName(kind)))
	if err := writePNG(p, typeIcon(kind)); err == nil {
		n.iconPaths[kind] = p
		return p
	}
	return ""
}

// thumbnailImage decodifica a imagem em path e a reduz para no máximo
// maxThumbSize. Retorna ok=false se o arquivo não for uma imagem.
func thumbnailImage(path string) (image.Image, bool) {
	if path == "" {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, false
	}
	return scaleImage(img, maxThumbSize), true
}

// thumbnailData decodifica a imagem em path e retorna no formato image-data
// (RGBA) do hint de notificações. Retorna ok=false se não for uma imagem.
func thumbnailData(path string) (imageData, bool) {
	img, ok := thumbnailImage(path)
	if !ok {
		return imageData{}, false
	}
	return toImageData(img), true
}

// imageData é a estrutura (iiibiiay) do hint image-data do spec de
// notificações: largura, altura, rowstride, alpha, bits, canais e pixels.
type imageData struct {
	Width         int32
	Height        int32
	Rowstride     int32
	HasAlpha      bool
	BitsPerSample int32
	Channels      int32
	Data          []byte
}

// scaleImage reduz a imagem mantendo a proporção, com o maior lado em
// maxSize. Usa reamostragem Catmull-Rom (x/image/draw) para qualidade.
func scaleImage(img image.Image, maxSize int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxSize && h <= maxSize {
		return img
	}
	scale := float64(maxSize) / float64(max(w, h))
	nw := int(float64(w) * scale)
	nh := int(float64(h) * scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

// toImageData converte uma imagem para RGBA (r, g, b, a por pixel), o
// formato esperado pelo hint image-data — mesmo formato do image.RGBA e da
// biblioteca de referência esiqveland/notify (que envia img.Pix direto).
func toImageData(img image.Image) imageData {
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return imageData{
		Width:         int32(rgba.Rect.Dx()),
		Height:        int32(rgba.Rect.Dy()),
		Rowstride:     int32(rgba.Stride),
		HasAlpha:      true,
		BitsPerSample: 8,
		Channels:      4,
		Data:          rgba.Pix,
	}
}

// notificationImage monta a imagem exibida na notificação: a miniatura do
// arquivo com o ícone do tipo como um selo no canto superior direito. Sem
// miniatura, usa apenas o ícone do tipo. Sempre retorna uma imagem válida.
func notificationImage(opts Options) image.Image {
	icon := typeIcon(opts.Kind)
	thumb, ok := thumbnailImage(opts.ImagePath)
	if !ok {
		return icon
	}
	return badgeIcon(thumb, icon)
}

// badgeIcon desenha o ícone do tipo como um selo no canto superior direito
// da miniatura, garantindo que o tipo da notificação fique visível mesmo em
// daemons que ignoram o app_icon.
func badgeIcon(thumb, icon image.Image) image.Image {
	b := thumb.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), thumb, b.Min, draw.Src)
	pad := 4
	size := badgeSize
	if b.Dx() < badgeSize*2 || b.Dy() < badgeSize*2 {
		size = max(10, min(b.Dx(), b.Dy())/3)
	}
	badge := scaleImage(icon, size)
	bb := badge.Bounds()
	pos := image.Point{X: b.Dx() - bb.Dx() - pad, Y: pad}
	draw.Draw(dst, image.Rect(pos.X, pos.Y, pos.X+bb.Dx(), pos.Y+bb.Dy()), badge, bb.Min, draw.Over)
	return dst
}

// typeIcon gera um ícone colorido (círculo + glifo) para o tipo de
// notificação, usando as cores do design system do app.
func typeIcon(kind Kind) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, iconSize, iconSize))
	drawCircle(img, iconSize/2, iconSize/2, iconSize/2-2, kindColor(kind))
	switch kind {
	case KindDone:
		drawCheck(img)
	case KindError:
		drawX(img)
	case KindSkipped:
		drawWarning(img)
	}
	return img
}

// kindColor retorna a cor do tipo, alinhada ao design system do app.
func kindColor(kind Kind) color.RGBA {
	switch kind {
	case KindDone:
		return color.RGBA{0x46, 0xc1, 0x7a, 0xff} // Grow Green
	case KindError:
		return color.RGBA{0xff, 0x5c, 0x5c, 0xff} // Ember Red
	case KindSkipped:
		return color.RGBA{0xf0, 0xb4, 0x29, 0xff} // Amber
	}
	return color.RGBA{0x4f, 0x8c, 0xff, 0xff} // Arc Blue
}

// kindName retorna o sufixo usado no nome do arquivo do ícone.
func kindName(kind Kind) string {
	switch kind {
	case KindDone:
		return "done"
	case KindError:
		return "error"
	case KindSkipped:
		return "skipped"
	}
	return "info"
}

// drawCircle preenche um círculo de raio r centrado em (cx, cy).
func drawCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
}

// drawLine desenha um segmento de reta com espessura t entre dois pontos.
func drawLine(img *image.RGBA, x0, y0, x1, y1, t int, c color.RGBA) {
	steps := max(abs(x1-x0), abs(y1-y0)) * 2
	if steps == 0 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		x := x0 + (x1-x0)*i/steps
		y := y0 + (y1-y0)*i/steps
		for dy := -t / 2; dy <= t/2; dy++ {
			for dx := -t / 2; dx <= t/2; dx++ {
				img.Set(x+dx, y+dy, c)
			}
		}
	}
}

// drawCheck desenha um "✓" branco (concluído).
func drawCheck(img *image.RGBA) {
	c := color.RGBA{255, 255, 255, 255}
	drawLine(img, 16, 34, 28, 46, 5, c)
	drawLine(img, 28, 46, 48, 20, 5, c)
}

// drawX desenha um "✕" branco (erro).
func drawX(img *image.RGBA) {
	c := color.RGBA{255, 255, 255, 255}
	drawLine(img, 18, 18, 46, 46, 5, c)
	drawLine(img, 46, 18, 18, 46, 5, c)
}

// drawWarning desenha um "!" branco (ignorado).
func drawWarning(img *image.RGBA) {
	c := color.RGBA{255, 255, 255, 255}
	drawLine(img, 32, 18, 32, 40, 5, c)
	drawCircle(img, 32, 48, 3, c)
}

// writePNG grava a imagem em um arquivo PNG.
func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}