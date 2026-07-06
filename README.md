

# Der KonsensOmat

> **Fork-Hinweis:** Dies ist ein Fork von [OpenKunde/konsensomat](https://github.com/OpenKunde/konsensomat) (PHP). Diese Version wurde vollständig nach Go portiert und seitdem eigenständig weiterentwickelt.

KonsensOmat ist ein schlankes Open-Source-Tool zur Entscheidungsfindung nach dem Prinzip des
Systemic Consensing (Systemisches Konsensieren, SK):
https://en.wikipedia.org/wiki/Systemic_Consensing

Statt Mehrheiten zu zählen, wird der Widerstand gegenüber Vorschlägen bewertet.
Die Option mit dem geringsten Gesamtwiderstand gilt als tragfähigste Lösung.

**Hinweis:** Das Tool ist derzeit nur auf Deutsch verfügbar.

---

## Eigenschaften

- Bewertung nach dem Widerstandsprinzip (SK)
- Anonyme Teilnahme (Pseudonyme möglich)
- Keine Registrierung, keine Datenbank
- Speicherung als einfache Dateien
- Laufzeit pro Umfrage von der Erstellerin/vom Ersteller wählbar, automatische Löschung danach (Obergrenze konfigurierbar, Standard: 7 Tage)
- Optionaler Passwortschutz pro Umfrage
- Nur Erstellerin/Ersteller (und optional Admins) können eine Umfrage löschen – nicht jede Person mit dem Link
- Keine externen Dienste oder Tracker
- Light- und Dark-Mode (folgt der Systemeinstellung, manuell umschaltbar)
- Öffentliche Statistik-Seite (`/statistik`) mit Aktivitäts-Heatmap der letzten 180 Tage
- Rate-Limiting gegen wiederholtes Passwort-Raten (Umfrage- und Admin-Login)
- Sicherheits-Header (CSP, X-Frame-Options, HSTS, …) und CSRF-Schutz für alle Formulare
- Infoseiten zu systemischem Konsensieren, Impressum und Datenschutz

---

## Voraussetzungen

- Go 1.26 oder höher
- Schreibrechte für `files/data`

---

## Lokale Nutzung

Repository klonen:

```bash
git clone https://github.com/rkl110/konsensomat.git
cd konsensomat
```

Go-Version prüfen:

```bash
go version
```

Lokalen Server starten:

```bash
go run .
```

Im Browser öffnen:

```
http://localhost:8080
```

Optional lässt sich die Konfiguration über Umgebungsvariablen anpassen:

```bash
KONSENSOMAT_ADDR=:8080 KONSENSOMAT_DATA_DIR=files/data KONSENSOMAT_EXPIRY_DAYS=14 go run .
```

| Variable                     | Standard      | Bedeutung                                         |
|-------------------------------|---------------|----------------------------------------------------|
| `KONSENSOMAT_ADDR`            | `:8080`       | Adresse, auf der der Server lauscht                |
| `KONSENSOMAT_DATA_DIR`        | `files/data`  | Verzeichnis für die Umfrage-JSON-Dateien           |
| `KONSENSOMAT_EXPIRY_DAYS`     | `7`           | Maximale und vorausgewählte Laufzeit einer Umfrage in Tagen (max. 365) |
| `KONSENSOMAT_ADMIN_PASSWORD`  | *(leer = aus)*| Admin-Passwort für `/admin` (siehe [Zugriffsschutz & Löschen](#zugriffsschutz--löschen)) |
| `KONSENSOMAT_TRUSTED_PROXIES` | *(leer)*      | Vertrauenswürdige Reverse-Proxy-IPs/-CIDRs (siehe [Hinter einem Reverse-Proxy betreiben](#hinter-einem-reverse-proxy-betreiben)) |

### Konfiguration über `.env`

Statt Umgebungsvariablen von Hand zu setzen, kann eine `.env`-Datei im Arbeitsverzeichnis abgelegt werden (siehe `.env.example`):

```bash
cp .env.example .env
```

```
KONSENSOMAT_ADDR=:8080
KONSENSOMAT_DATA_DIR=files/data
KONSENSOMAT_EXPIRY_DAYS=7
#KONSENSOMAT_ADMIN_PASSWORD=
#KONSENSOMAT_TRUSTED_PROXIES
```

Bereits gesetzte echte Umgebungsvariablen haben immer Vorrang vor der `.env`-Datei. Die `.env` selbst wird nicht versioniert (siehe `.gitignore`).

### Tests ausführen

```bash
go test ./...
```

Mit Race-Detector (empfohlen, da Rate-Limiter und Statistik-Cache nebenläufig zugreifen):

```bash
go test -race ./...
```

---

## Zugriffsschutz & Löschen

**Löschrecht:** Beim Erstellen einer Umfrage wird im Browser der Erstellerin/des Erstellers automatisch ein Berechtigungs-Cookie gesetzt. Damit – und nur damit – kann die eigene Umfrage jederzeit wieder gelöscht werden. Wer nur den Link kennt, kann eine Umfrage **nicht** löschen, auch wenn kein Passwort gesetzt ist.

Zusätzlich kann beim Erstellen ein optionales Passwort vergeben werden. Ist eines gesetzt, müssen alle anderen Besucher*innen es einmalig eingeben, um die Umfrage überhaupt zu sehen oder abzustimmen – das Passwort gewährt aber **kein** Löschrecht, auch nicht nach dem Entsperren.

Eine Umfrage kann also, ob mit oder ohne Passwort, immer nur von der Erstellerin/dem Ersteller selbst (in ihrem/seinem Browser) oder von einem Admin gelöscht werden.

**Admin-Link teilen:** Auf der Umfrageseite sieht die Erstellerin/der Ersteller einen zusätzlichen Admin-Link (mit eingebettetem Berechtigungs-Token). Wird dieser Link in einem anderen Browser/Gerät geöffnet, gilt auch dieser fortan als Ersteller*in der Umfrage – nützlich, um die Verwaltung mit Mitorganisator*innen zu teilen, ohne mit ihnen den Umfrage-Link samt Passwort teilen zu müssen. Wer diesen Link kennt, kann die Umfrage löschen und ihr Passwort ändern.

**Passwort, Frage/Vorschläge und Laufzeit nachträglich ändern:** Wer verwalten darf (Erstellerin/Ersteller oder Admin), sieht auf der Umfrageseite einen (standardmäßig eingeklappten, per Klick aufklappbaren) Verwaltungsbereich, über den sich jederzeit das Passwort setzen/ändern/entfernen, die Frage samt Vorschlägen korrigieren und die verbleibende Laufzeit verlängern oder verkürzen lässt (weiterhin begrenzt auf `KONSENSOMAT_EXPIRY_DAYS`).

**Statistik-Seite:** `/statistik` ist ohne Anmeldung für alle einsehbar und zeigt ausschließlich aggregierte Nutzungszahlen (aktive Umfragen, davon gültig/ungültig, bald ablaufend, durchschnittliche/maximale Teilnehmerzahl) sowie eine Aktivitäts-Heatmap: eine Zelle pro Tag der letzten 180 Tage, eingefärbt nach der Anzahl an diesem Tag erstellter gültiger Umfragen (≥ 2 Teilnehmer*innen). Es werden dabei nie Inhalte oder Links einzelner Umfragen preisgegeben.

**Admins:** Ist `KONSENSOMAT_ADMIN_PASSWORD` gesetzt, können sich Admins unter `/admin` anmelden und danach jede Umfrage einsehen, verwalten und löschen – unabhängig von deren eigenem Passwort. Auf der Statistik-Seite (`/statistik`) sehen angemeldete Admins zusätzlich eine Liste aller aktiven Umfragen (Frage, Teilnehmerzahl, Ablaufdatum) mit direktem Link zur Verwaltung – ohne Admin-Login bleiben Umfragen weiterhin nur über ihren eigenen Link erreichbar. Das ist als Moderations-Werkzeug gedacht (z.B. um eine gemeldete, missbräuchliche Umfrage zu entfernen), nicht als reguläres Nutzerkonto. Ohne gesetztes Admin-Passwort ist die Funktion vollständig deaktiviert.

**Laufzeit:** Beim Erstellen wählt die Erstellerin/der Ersteller, wie viele Tage die Umfrage laufen soll (maximal `KONSENSOMAT_EXPIRY_DAYS`, das ist auch die Vorauswahl). Nach Ablauf wird die Umfrage automatisch gelöscht – unabhängig von Löschrecht oder Passwort.

Es gibt bewusst kein Nutzerkonto und keine Passwort-Wiederherstellung – gehen sowohl das Berechtigungs-Cookie als auch ein gesetztes Passwort verloren und ist kein Admin-Passwort konfiguriert, lässt sich die Umfrage nur noch über ihre automatische Löschung nach Ablauf der gewählten Laufzeit entfernen.

**Brute-Force-Schutz:** Falsche Passwort-Versuche (Umfrage- oder Admin-Passwort, über Web-UI oder API) werden pro Client-IP gezählt; nach 10 Fehlversuchen innerhalb von 5 Minuten wird diese IP für den jeweiligen Endpunkt vorübergehend gesperrt (HTTP 429). Erfolgreiche Versuche und Anfragen ganz ohne Passwort zählen nicht mit.

---

## API

Neben der Weboberfläche lässt sich die gesamte Anwendung auch über eine JSON-API steuern. Die API folgt demselben Zugriffsmodell wie die Weboberfläche (siehe [Zugriffsschutz & Löschen](#zugriffsschutz--löschen)). Es gibt bewusst keinen Endpunkt, der alle Umfragen auflistet, da Umfragen nur über ihren Link erreichbar sein sollen.

| Methode | Pfad                     | Beschreibung                          |
|---------|--------------------------|----------------------------------------|
| POST    | `/api/polls`             | Neue Umfrage erstellen                 |
| GET     | `/api/polls/{id}`        | Umfrage inkl. Ergebnis abrufen         |
| POST    | `/api/polls/{id}/votes`  | Abstimmen                              |
| DELETE  | `/api/polls/{id}`        | Umfrage löschen                        |
| GET     | `/api/stats`             | Aggregierte Nutzungsstatistik          |

Beispiel – Umfrage erstellen (optional mit `password` und `durationDays`, Standard/Maximum ist `KONSENSOMAT_EXPIRY_DAYS`):

```bash
curl -X POST http://localhost:8080/api/polls \
  -H "Content-Type: application/json" \
  -d '{"question":"Wohin geht die Firmenfeier?","options":["Strand","Berge","Stadt"],"password":"geheim","durationDays":3}'
```

Die Antwort enthält einmalig ein `ownerToken` – das solltest du dir aufheben, denn nur damit (oder als Admin) lässt sich die Umfrage später wieder löschen:

```json
{"id":"a1b2c3","question":"...", "ownerToken":"…", "hasPassword":true, "expiresAt":1234567890, "...": "..."}
```

Ist eine Umfrage passwortgeschützt, muss das Passwort bei `GET`/`votes` per Header `X-Poll-Password: geheim` oder Query-Parameter `?password=geheim` mitgeschickt werden. Für `DELETE` reicht das Passwort dagegen nicht aus – dafür ist immer der Owner-Token nötig (per `X-Owner-Token`-Header oder `?owner=`-Parameter), es sei denn, ein Admin-Passwort ist konfiguriert; dieses wirkt über `X-Admin-Password` bzw. `?adminPassword=` für beliebige Umfragen.

Beispiel – abstimmen (ein Wert je Option, 0 = kein Widerstand … 4 = starker Widerstand; `comments` optional, wird nur bei Wert `4` angezeigt):

```bash
curl -X POST "http://localhost:8080/api/polls/<id>/votes?password=geheim" \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","votes":[0,4,2],"comments":["","Zu weit weg",""]}'
```

Beispiel – löschen mit Owner-Token:

```bash
curl -X DELETE "http://localhost:8080/api/polls/<id>?owner=<ownerToken>"
```

Alle API-Antworten sind JSON, Fehler haben die Form `{"error": "..."}`, und `Access-Control-Allow-Origin: *` ist gesetzt, damit die API auch aus dem Browser heraus von anderen Origins angesprochen werden kann.

---

## Deployment

Statisches Binary bauen (Templates und Assets werden per `go:embed` eingebettet):

```bash
go build -o konsensomat .
./konsensomat
```

Schreibrechte für `files/data` vergeben (bzw. `KONSENSOMAT_DATA_DIR` auf ein beschreibbares Verzeichnis setzen).

### Logs

Die Anwendung loggt ausschließlich nach stdout/stderr (kein Log-File) - normale Betriebsmeldungen (Start, eine Zeile pro Request mit Client-IP, Methode, Pfad, Status und Dauer) gehen nach stdout, Fehler nach stderr. Das lässt sich mit den üblichen Bordmitteln trennen bzw. einsammeln, z.B. `docker compose logs -f` oder ein Log-Collector, der stdout/stderr eines Containers ausliest.

### Hinter einem Reverse-Proxy betreiben

KonsensOmat terminiert selbst kein TLS und ist für den Betrieb hinter einem Reverse-Proxy (nginx, Caddy, Traefik, …) ausgelegt: Der Proxy nimmt HTTPS entgegen und leitet per HTTP an KonsensOmat weiter, üblicherweise mit den Headern `X-Forwarded-Proto` und `X-Forwarded-For`.

`X-Forwarded-Proto: https` wird automatisch erkannt (steuert u.a. das `Secure`-Attribut der Cookies) - hier ist keine Konfiguration nötig.

Für `X-Forwarded-For` (die tatsächliche Client-IP, die Rate-Limiting und Zugriffs-Logs verwenden) muss der Proxy explizit als vertrauenswürdig eingetragen werden, per `KONSENSOMAT_TRUSTED_PROXIES` in der `.env`-Datei oder als Umgebungsvariable:

```
KONSENSOMAT_TRUSTED_PROXIES=127.0.0.1,10.0.0.0/8
```

Kommagetrennte Liste aus einzelnen IPs und/oder CIDR-Bereichen (IPv4 und IPv6). Ohne diese Angabe (Standard) wird `X-Forwarded-For` ignoriert und immer die direkt verbindende IP verwendet - das ist bewusst die sichere Grundeinstellung, denn dieser Header lässt sich von jedem beliebig setzen, der den Server direkt erreicht. Erst wenn eine Anfrage nachweislich von einer der eingetragenen Proxy-Adressen kommt, wird die von ihr gesetzte Client-IP übernommen (bei mehreren verketteten vertrauenswürdigen Proxies wird von rechts nach links die erste nicht selbst vertrauenswürdige Adresse verwendet).

### Über Docker

```bash
cp .env.example .env      # falls noch nicht geschehen
make docker-build         # baut das Linux-Binary und das Docker-Image (konsensomat:latest)
docker compose up
```

`docker-compose.yml` bindet `./files/data` ins Image, damit Umfragen einen Container-Neustart überleben, und lädt die Konfiguration aus `.env` (siehe [Konfiguration über `.env`](#konfiguration-über-env)).

Das Standard-`Dockerfile` baut auf `scratch` auf (kein Betriebssystem, keine Shell - nur das Binary), läuft nicht als root. Wer stattdessen einen `HEALTHCHECK` braucht (z.B. für Orchestrierung), nutzt die etwas größere Alpine-Variante:

```bash
make docker-build-alpine
```

Weitere `make`-Ziele (`make` ohne Argument bzw. ein Blick ins `Makefile` zeigen alle): `build-windows`, `build-rpi64`, `build-rpi32`, `build-mac` (macOS/Apple Silicon) für andere Zielplattformen, `test`, `run` (= `go run .`) und `clean`.

---

## Screenshots


**Abb. 1: Desktop-Ansicht**

![Abb. 1: Desktop-Ansicht](screenshots/desktop.png)

**Abb. 2: Mobile-Ansicht**

![Abb. 2: Mobile-Ansicht](screenshots/mobile.png)

---

## Lizenz

GNU Affero General Public License v3 (AGPLv3).
Siehe Datei `LICENSE`.
