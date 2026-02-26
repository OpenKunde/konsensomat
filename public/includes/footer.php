<div class="footer">
<a href="impressum.php">Impressum</a> |
<a href="datenschutz.php">Datenschutz</a> |
<a href="#" id="infoLink">Was ist systemisches Konsensieren?</a>
</div>

<div class="modal-overlay" id="infoModal">
<div class="modal">
<button class="modal-close" id="modalClose">&times;</button>
<h2>Was ist systemisches Konsensieren?</h2>
<p>
Systemisches Konsensieren (SK) ist ein Entscheidungsverfahren, das den <strong>Widerstand</strong>
gegen Vorschläge misst – statt nur die Zustimmung. So findet die Gruppe die Lösung,
die von allen am ehesten mitgetragen werden kann.
</p>
<h3>Wie funktioniert es?</h3>
<p>
Jede Person bewertet jeden Vorschlag danach, wie viel Widerstand sie dagegen empfindet –
von keinem Widerstand (🥳) bis zu starkem Widerstand (😢).
Die Widerstandswerte werden pro Vorschlag aufsummiert.
<strong>Der Vorschlag mit dem geringsten Gesamtwiderstand gewinnt.</strong>
</p>
<h3>Warum nicht einfach abstimmen?</h3>
<p>
Bei klassischen Mehrheitsentscheidungen kann eine knappe Mehrheit Lösungen durchsetzen,
die fast die Hälfte der Gruppe ablehnt. Beim systemischen Konsensieren werden dagegen
die Bedenken aller berücksichtigt. Das Ergebnis ist ein echter Konsens – eine Lösung,
mit der möglichst wenige ein Problem haben.
</p>
<h3>Über den KonsensOmat</h3>
<p>
Der KonsensOmat ist ein einfaches Werkzeug, das systemisches Konsensieren digital umsetzt.
Erstelle eine Frage mit mehreren Vorschlägen, teile den Link mit deiner Gruppe und findet
gemeinsam die Lösung mit dem geringsten Widerstand.
</p>
</div>
</div>

<script>
(function() {
    var overlay = document.getElementById("infoModal");
    var link = document.getElementById("infoLink");
    var closeBtn = document.getElementById("modalClose");

    link.addEventListener("click", function(e) {
        e.preventDefault();
        overlay.classList.add("active");
    });

    closeBtn.addEventListener("click", function() {
        overlay.classList.remove("active");
    });

    overlay.addEventListener("click", function(e) {
        if (e.target === overlay) {
            overlay.classList.remove("active");
        }
    });
})();
</script>

<div class="open-source-hinweis">
  <a href="https://github.com/OpenKunde/konsensomat" target="_blank" rel="noopener">
    <span>Open Source (AGPLv3) – Quellcode auf GitHub</span>
    <svg height="18" width="18" viewBox="0 0 16 16" aria-hidden="true">
      <path fill="currentColor"
        d="M8 0C3.58 0 0 3.58 0 8a8 8 0 0 0 5.47 7.59c.4.07.55-.17.55-.38
        0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13
        -.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66
        .07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95
        0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82
        .64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82
        .44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15
        0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48
        0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8 8 0 0 0 16 8
        c0-4.42-3.58-8-8-8Z"/>
    </svg>
  </a>
</div>
</p>
<div class="open-source-hinweis">
  <a href="https://github.com/OpenKunde/konsensomat" target="_blank" rel="noopener">
    <span>Open Source (AGPLv3) – Quellcode auf GitHub</span>
    <svg height="18" width="18" viewBox="0 0 16 16" aria-hidden="true">
      <path fill="currentColor"
        d="M8 0C3.58 0 0 3.58 0 8a8 8 0 0 0 5.47 7.59c.4.07.55-.17.55-.38
        0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13
        -.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66
        .07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95
        0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82
        .64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82
        .44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15
        0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48
        0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8 8 0 0 0 16 8
        c0-4.42-3.58-8-8-8Z"/>
    </svg>
  </a>
</div>

</body>
</html>
