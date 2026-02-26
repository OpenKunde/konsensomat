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

</div>

</body>
</html>
