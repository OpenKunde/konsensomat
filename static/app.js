(function () {
    "use strict";

    /* ===== Farbschema umschalten ===== */
    var toggle = document.getElementById("themeToggle");
    if (toggle) {
        var systemPrefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;

        function currentTheme() {
            var explicit = document.documentElement.getAttribute("data-theme");
            if (explicit) return explicit;
            return systemPrefersDark ? "dark" : "light";
        }

        toggle.addEventListener("click", function () {
            var next = currentTheme() === "dark" ? "light" : "dark";
            document.documentElement.setAttribute("data-theme", next);
            localStorage.setItem("theme", next);
        });
    }

    /* ===== Dynamische Vorschlagsliste (Konsens erstellen) ===== */
    var optionsList = document.getElementById("optionsList");
    var addOptionBtn = document.getElementById("addOption");
    var optionCounter = optionsList ? optionsList.querySelectorAll(".option-input").length : 0;

    function updateRemoveButtons() {
        var rows = optionsList.querySelectorAll(".option-input");
        rows.forEach(function (row) {
            var removeBtn = row.querySelector(".remove-option");
            if (!removeBtn) return;
            removeBtn.disabled = rows.length <= 2;
        });
    }

    if (optionsList && addOptionBtn) {
        addOptionBtn.addEventListener("click", function () {
            optionCounter++;
            var row = document.createElement("div");
            row.className = "option-input";
            row.innerHTML =
                '<input type="text" name="options[]" placeholder="Vorschlag ' + optionCounter + '">' +
                '<button type="button" class="remove-option" aria-label="Vorschlag entfernen">×</button>';
            optionsList.appendChild(row);
            row.querySelector("input").focus();
            updateRemoveButtons();
        });

        optionsList.addEventListener("click", function (event) {
            var removeBtn = event.target.closest(".remove-option");
            if (!removeBtn) return;
            var rows = optionsList.querySelectorAll(".option-input");
            if (rows.length <= 2) return;
            removeBtn.closest(".option-input").remove();
            updateRemoveButtons();
        });

        updateRemoveButtons();
    }

    /* ===== Widerstands-Bewertung (Abstimmen) ===== */
    document.querySelectorAll(".emoji-group").forEach(function (group) {
        var spans = group.querySelectorAll("span");
        var hidden = group.querySelector("input[type=hidden]");
        var commentField = group.parentElement.querySelector(".comment-field");

        spans.forEach(function (span) {
            span.addEventListener("click", function () {
                spans.forEach(function (s) { s.classList.remove("selected"); });
                span.classList.add("selected");
                hidden.value = span.dataset.value;
                commentField.style.display = (span.dataset.value === "4") ? "block" : "none";
            });
        });
    });

    /* ===== Link teilen ===== */
    var shareBtn = document.getElementById("shareBtn");
    if (shareBtn) {
        shareBtn.addEventListener("click", async function () {
            var url = window.location.href;
            if (navigator.share) {
                navigator.share({ title: "KonsensOmat", url: url });
            } else {
                await navigator.clipboard.writeText(url);
                alert("Link kopiert");
            }
        });
    }

    /* ===== Admin-Link teilen ===== */
    var ownerShareBtn = document.getElementById("ownerShareBtn");
    if (ownerShareBtn) {
        ownerShareBtn.addEventListener("click", async function () {
            var url = new URL(ownerShareBtn.dataset.ownerLink, window.location.href).href;
            if (navigator.share) {
                navigator.share({ title: "KonsensOmat – Verwaltung", url: url });
            } else {
                await navigator.clipboard.writeText(url);
                alert("Admin-Link kopiert");
            }
        });
    }

    /* ===== Löschen bestätigen ===== */
    var deleteForm = document.getElementById("deleteForm");
    if (deleteForm) {
        deleteForm.addEventListener("submit", function (event) {
            if (!confirm("Möchtest du diese Umfrage wirklich löschen?")) {
                event.preventDefault();
            }
        });
    }
})();
