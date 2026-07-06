package main

import "net/http"

/* ===== Statische Infoseiten ===== */

func (s *server) handleInfo(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "info.html", basePage{PageTitle: pageTitle("Was ist systemisches Konsensieren?")})
}

func (s *server) handleImpressum(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "impressum.html", basePage{PageTitle: pageTitle("Impressum")})
}

func (s *server) handleDatenschutz(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "datenschutz.html", basePage{PageTitle: pageTitle("Datenschutz")})
}
