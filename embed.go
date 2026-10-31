package main

import "embed"

// assets is the built frontend. `wails dev` and `wails build` populate
// frontend/dist automatically (see wails.json frontend:build).
//
//go:embed all:frontend/dist
var assets embed.FS
