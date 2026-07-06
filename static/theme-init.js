(function () {
    "use strict";
    var stored = localStorage.getItem("theme");
    var theme = stored || "auto";
    if (theme !== "auto") {
        document.documentElement.setAttribute("data-theme", theme);
    }
})();
