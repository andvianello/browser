// Copyright 2020 Eurac Research. All rights reserved.

package http

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"text/template"
	"time"

	"gitlab.inf.unibz.it/lter/browser"
	"gitlab.inf.unibz.it/lter/browser/static"
	"golang.org/x/net/xsrftoken"
)

func (h *Handler) handleSeries() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Expected POST request", http.StatusMethodNotAllowed)
			return
		}

		if !xsrftoken.Valid(r.FormValue("token"), h.key, "", "") {
			Error(w, browser.ErrInvalidToken, http.StatusForbidden)
			return
		}

		m, err := parseMessage(r)
		if err != nil {
			Error(w, err, http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		b, err := h.db.SeriesV1(ctx, m)
		if errors.Is(err, browser.ErrDataNotFound) {
			Error(w, err, http.StatusBadRequest)
			return
		}
		if err != nil {
			Error(w, err, http.StatusInternalServerError)
			return
		}

		filename := fmt.Sprintf("LTSER_IT25_Matsch_Mazia_%d.csv", time.Now().Unix())
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Description", "File Transfer")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)

		csv.NewWriter(w).WriteAll(b)
	}
}

func (h *Handler) handleCodeTemplate() http.HandlerFunc {
	var (
		tmpl struct {
			python, rlang *template.Template
		}
		err error
	)

	tmpl.python, err = static.ParseTextTemplates(nil, "python.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	tmpl.rlang, err = static.ParseTextTemplates(nil, "r.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Expected POST request", http.StatusMethodNotAllowed)
			return
		}

		if !xsrftoken.Valid(r.FormValue("token"), h.key, "", "") {
			Error(w, browser.ErrInvalidToken, http.StatusForbidden)
			return
		}

		var (
			t   *template.Template
			ext string
		)
		switch r.FormValue("language") {
		case "python":
			t = tmpl.python
			ext = "py"
		case "r":
			t = tmpl.rlang
			ext = "r"
		default:
			Error(w, browser.ErrInternal, http.StatusInternalServerError)
			return
		}

		m, err := parseMessage(r)
		if err != nil {
			Error(w, err, http.StatusInternalServerError)
			return
		}

		ctx := r.Context()
		stmt := h.db.Query(ctx, m)

		filename := fmt.Sprintf("LTSER_IT25_Matsch_Mazia_%d.%s", time.Now().Unix(), ext)
		w.Header().Set("Content-Description", "File Transfer")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		err = t.Execute(w, struct {
			Query    string
			Database string
		}{
			Query:    stmt.Query,
			Database: stmt.Database,
		})
		if err != nil {
			Error(w, err, http.StatusInternalServerError)
		}
	}
}

// parseForm parses form values from the given http.Request and returns
// an request. It performs basic validation for given dates.
func parseMessage(r *http.Request) (*browser.Message, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	start, err := time.Parse("2006-01-02", r.FormValue("startDate"))
	if err != nil {
		return nil, fmt.Errorf("could not parse start date %v", err)
	}

	end, err := time.Parse("2006-01-02", r.FormValue("endDate"))
	if err != nil {
		return nil, fmt.Errorf("could not parse end date %v", err)
	}

	if end.After(time.Now()) {
		return nil, errors.New("error: end date is in the future")
	}

	if r.Form["measurements"] == nil {
		return nil, errors.New("at least one measurement must be given")
	}

	if r.Form["stations"] == nil {
		return nil, errors.New("at least one station must be given")
	}

	return &browser.Message{
		Measurements: r.Form["measurements"],
		Stations:     r.Form["stations"],
		Landuse:      r.Form["landuse"],
		Start:        start,
		End:          end,
	}, nil
}