// Copyright 2020 Eurac Research. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package assets

import (
	"embed"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

// Files contains all files.
//go:embed *
var Files embed.FS

var (
	//go:embed public/*
	public embed.FS
	fs     = http.FS(public)
)

// Public serves public content to HTTP.
func Public(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path[1:], "static/")
	f, err := fs.Open(filepath.Join("public", p))
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	s, err := f.Stat()
	if err != nil {
		log.Println(err)
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, filepath.Base(p), s.ModTime(), f)
}
