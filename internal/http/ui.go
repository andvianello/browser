// Copyright 2020 Eurac Research. All rights reserved.

package http

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"gitlab.inf.unibz.it/lter/browser"
	"gitlab.inf.unibz.it/lter/browser/static"

	"golang.org/x/net/xsrftoken"
	"gopkg.in/russross/blackfriday.v2"
)

func (h *Handler) handleIndex() http.HandlerFunc {
	funcMap := template.FuncMap{
		"T":         translate,
		"Is":        isRole,
		"HasSuffix": strings.HasSuffix,
		"Last": func(i, l int) bool {
			return i == (l - 1)
		},
	}

	tmpl, err := static.ParseTemplates(template.New("base.tmpl").Funcs(funcMap), "base.tmpl", "index.tmpl", "nav.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := browser.UserFromContext(ctx)

		data, err := h.metadata.Stations(ctx, &browser.Message{})
		if err != nil {
			Error(w, err, http.StatusInternalServerError)
			return
		}

		err = tmpl.Execute(w, struct {
			Data          browser.Stations
			User          *browser.User
			Language      string
			Path          string
			AnalyticsCode string
			Token         string
			StartDate     string
			EndDate       string
		}{
			data,
			user,
			languageFromCookie(r),
			r.URL.Path,
			h.analytics,
			xsrftoken.Generate(h.key, user.Username, ""),
			time.Now().AddDate(0, -6, 0).Format("2006-01-02"),
			time.Now().Format("2006-01-02"),
		})
		if err != nil {
			Error(w, err, http.StatusInternalServerError)
		}
	}
}

func (h *Handler) handleMarkdownPage() http.HandlerFunc {
	funcMap := template.FuncMap{
		"T":  translate,
		"Is": isRole,
	}

	tmpl, err := static.ParseTemplates(template.New("base.tmpl").Funcs(funcMap), "base.tmpl", "page.tmpl", "nav.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) != ".md" {
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
			return
		}

		lang := languageFromCookie(r)
		// TODO(m): should we preload parsed pages?
		p, err := static.File(filepath.Join("pages", lang, filepath.Base(r.URL.Path)))
		if err != nil {
			http.NotFound(w, r)
			return
		}

		err = tmpl.Execute(w, struct {
			User          *browser.User
			Language      string
			Path          string
			AnalyticsCode string
			Content       template.HTML
		}{
			browser.UserFromContext(r.Context()),
			lang,
			r.URL.Path,
			h.analytics,
			template.HTML(blackfriday.Run([]byte(p))),
		})
		if err != nil {
			Error(w, err, http.StatusInternalServerError)
		}
	}
}

// TODO: extract to middleware?
func handleLanguage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := "en"

		switch r.URL.Path[len("/l/"):] {
		case "de":
			l = "de"
		case "it":
			l = "it"
		}

		http.SetCookie(w, &http.Cookie{
			Name:  languageCookieName,
			Value: l,
			Path:  "/",
		})

		ref := "/"
		refURL, err := url.Parse(r.Referer())
		if err == nil && refURL.Path != "" {
			ref = refURL.Path
		}

		w.Header().Set("Cache-Control", "no-cache, private, max-age=0")
		w.Header().Set("Expires", time.Unix(0, 0).Format(http.TimeFormat))
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("X-Accel-Expires", "0")

		http.Redirect(w, r, ref, http.StatusSeeOther)
	}
}

func isRole(r browser.Role, s string) bool {
	return r == browser.NewRole(s)
}

func languageFromCookie(r *http.Request) string {
	c, err := r.Cookie(languageCookieName)
	if err != nil {
		return "en"
	}
	return c.Value
}

func translate(key, lang string) string {
	j, err := static.File(filepath.Join("locale", fmt.Sprintf("%s.json", lang)))
	if err != nil {
		return key
	}

	var m map[string]string
	if err := json.Unmarshal([]byte(j), &m); err != nil {
		log.Printf("translation: %v\n", err)
		return key
	}

	v, ok := m[key]
	if !ok {
		return key
	}

	return v
}