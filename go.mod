module image_reduce

go 1.26.5

require (
	fyne.io/systray v1.12.2
	github.com/chai2010/webp v1.4.0
	github.com/fsnotify/fsnotify v1.10.1
	github.com/godbus/dbus/v5 v5.1.0
	github.com/webview/webview_go v0.0.0-20240831120633-6173450d4dd6
	golang.org/x/image v0.44.0
)

require golang.org/x/sys v0.15.0 // indirect

replace github.com/webview/webview_go => ./third_party/webview_go
