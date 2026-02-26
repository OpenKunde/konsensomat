# Der KonsensOmat

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
- Automatische Löschung nach spätestens 7 Tagen
- Keine externen Dienste oder Tracker

---

## Voraussetzungen

- PHP 8.0 oder höher
- Schreibrechte für `files/data`

---

## Lokale Nutzung

Repository klonen:

```bash
git clone https://github.com/OpenKunde/konsensomat.git
cd konsensomat/public
```

PHP-Version prüfen:

```bash
php -v
```

Lokalen Server starten:

```bash
php -S 0.0.0.0:8000
```

Im Browser öffnen:

```
http://localhost:8000
```

---

## Deployment

Bei Webhosting den Webroot auf `public/` setzen und Schreibrechte für `files/data` vergeben.

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
