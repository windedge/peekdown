// Package assets holds the pictures the binary draws itself, beside the
// files mygo packs: go:embed cannot reach outside its own package, so the
// app's icon lives here and mygo.json points at it too.
package assets

import _ "embed"

// AppIcon is the app's mark as a 1024x1024 PNG: a dark rounded square with
// the white P and the downward peek that names the app.
//
//go:embed app_icon.png
var AppIcon []byte
