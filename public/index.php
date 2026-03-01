<?php
/*
 * KonsensOmat
 * Copyright (C) 2026 OpenKunde
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, version 3.
 *
 * See <https://www.gnu.org/licenses/>.
 */

session_start();

if (empty($_SESSION["token"])) {
    $_SESSION["token"] = bin2hex(random_bytes(32));
}

$dataDir = dirname(__DIR__) . "/files/data/";
$expiryDays = 7;

function randomId($length = 6) {
    return substr(bin2hex(random_bytes(6)), 0, $length);
}

function loadPoll($id, $dataDir, $expiryDays) {
    $file = $dataDir . $id . ".json";
    if (!file_exists($file)) return null;

    $data = json_decode(file_get_contents($file), true);

    if (time() - $data["created"] > ($expiryDays * 86400)) {
        unlink($file);
        return null;
    }

    return $data;
}

function savePoll($id, $data, $dataDir) {
    file_put_contents($dataDir . $id . ".json", json_encode($data, JSON_PRETTY_PRINT));
}

function deletePoll($id, $dataDir) {
    $file = $dataDir . $id . ".json";
    if (file_exists($file)) unlink($file);
}

$id = $_GET["id"] ?? null;
$poll = $id ? loadPoll($id, $dataDir, $expiryDays) : null;

if ($_SERVER["REQUEST_METHOD"] === "POST") {

    if (!isset($_POST["token"]) || $_POST["token"] !== $_SESSION["token"]) {
        die("Ungültiger CSRF-Token.");
    }

    if (isset($_POST["create"])) {
        $id = randomId();
        $poll = [
            "created" => time(),
            "question" => htmlspecialchars($_POST["question"]),
            "options" => array_values(array_filter(array_map("trim", explode("\n", $_POST["options"])))),
            "votes" => []
        ];
        savePoll($id, $poll, $dataDir);
        header("Location: ?id=" . $id);
        exit;
    }

    if (isset($_POST["vote"])) {
        $poll = loadPoll($id, $dataDir, $expiryDays);

        $poll["votes"][] = [
            "name" => htmlspecialchars($_POST["name"]),
            "votes" => $_POST["votes"],
            "comments" => $_POST["comments"] ?? []
        ];

        savePoll($id, $poll, $dataDir);
        header("Location: ?id=" . $id);
        exit;
    }

    if (isset($_POST["delete"])) {
        deletePoll($id, $dataDir);
        header("Location: /");
        exit;
    }
}

/* ===== Dynamischer Page Title ===== */
if ($poll && !empty($poll["question"])) {
    $pageTitle = "🤖 " . htmlspecialchars($poll["question"]) . " – KonsensOmat";
} else {
    $pageTitle = "🤖 KonsensOmat";
}

require __DIR__ . "/includes/header.php";
?>

<?php if (!$id): ?>

<div class="card">
<form method="post">
<input type="hidden" name="token" value="<?= $_SESSION["token"] ?>">

<label>Für welche Frage soll ein Konsens gesucht werden?</label>
<input type="text" name="question" required>

<label>Optionen (eine pro Zeile)</label>
<textarea name="options" rows="5" required></textarea>

<p class="notice">
Hinweis: Die Umfrage wird automatisch nach <?= $expiryDays ?> Tagen gelöscht.
</p>

<button type="submit" name="create">Konsens finden</button>
</form>
</div>

<?php elseif ($poll): ?>

<div class="card">
<h2><?= htmlspecialchars($poll["question"]) ?></h2>

<form method="post">
<input type="hidden" name="token" value="<?= $_SESSION["token"] ?>">
<input type="text" name="name" placeholder="Dein Name oder Pseudonym" required>

<p><strong>Wie fühlst du dich mit den folgenden Vorschlägen?</strong></p>

<?php foreach ($poll["options"] as $i => $opt): ?>
<div class="option-row">
<div><strong><?= htmlspecialchars($opt) ?></strong></div>

<div class="emoji-group">
<span data-value="0">🥳</span>
<span data-value="1">🙂</span>
<span data-value="2">😐</span>
<span data-value="3">🙁</span>
<span data-value="4">😢</span>
<input type="hidden" name="votes[<?= $i ?>]" value="2">
</div>

<textarea class="comment-field" name="comments[<?= $i ?>]" placeholder="Kommentar bei starkem Widerstand"></textarea>
</div>
<?php endforeach; ?>

<button type="submit" name="vote">Abstimmen</button>
</form>
</div>

<?php
$totals = array_fill(0, count($poll["options"]), 0);
$comments = [];

foreach ($poll["votes"] as $vote) {
    foreach ($vote["votes"] as $i => $v) {
        $totals[$i] += intval($v);
        if ($v == 4 && !empty($vote["comments"][$i])) {
            $comments[$i][] = $vote["name"] . ": " . $vote["comments"][$i];
        }
    }
}

$sorted = $totals;
asort($sorted);
$winnerIndex = array_key_first($sorted);

$participantCount = count($poll["votes"]);

$results = [];
foreach ($poll["options"] as $i => $opt) {
    $results[] = [
        "index" => $i,
        "text" => $opt,
        "total" => $totals[$i],
        "comments" => $comments[$i] ?? []
    ];
}

usort($results, function($a, $b) {
    return $a["total"] <=> $b["total"];
});
?>

<div class="card">
<h2>Ergebnis</h2>
<h4>
(<?= $participantCount ?> <?= $participantCount == 1 ? 'Person hat' : 'Personen haben' ?> sich beteiligt)
</h4>

<?php foreach ($results as $r): ?>
<div class="result-row <?= $r["index"] == $winnerIndex ? 'winner' : '' ?>">
<strong><?= htmlspecialchars($r["text"]) ?></strong><br>
Widerstand: <?= $r["total"] ?>

<?php if (!empty($r["comments"])): ?>
    <?php foreach ($r["comments"] as $c): ?>
        <div class="comment-entry"><?= htmlspecialchars($c) ?></div>
    <?php endforeach; ?>
<?php endif; ?>

</div>
<?php endforeach; ?>

<button id="shareBtn">Link teilen</button>

<form method="post" onsubmit="return confirm('Möchtest du diese Umfrage wirklich löschen?');">
<input type="hidden" name="token" value="<?= $_SESSION["token"] ?>">
<button type="submit" name="delete" class="delete">Umfrage löschen</button>
</form>

</div>

<?php endif; ?>

<script>
document.querySelectorAll(".emoji-group").forEach(group => {
    const spans = group.querySelectorAll("span");
    const hidden = group.querySelector("input[type=hidden]");
    const commentField = group.parentElement.querySelector(".comment-field");

    spans.forEach(span => {
        span.addEventListener("click", () => {
            spans.forEach(s => s.classList.remove("selected"));
            span.classList.add("selected");
            hidden.value = span.dataset.value;
            commentField.style.display = (span.dataset.value == "4") ? "block" : "none";
        });
    });
});

const shareBtn = document.getElementById("shareBtn");
if (shareBtn) {
    shareBtn.addEventListener("click", async () => {
        const url = window.location.href;
        if (navigator.share) {
            navigator.share({ title: "KonsensOmat", url: url });
        } else {
            await navigator.clipboard.writeText(url);
            alert("Link kopiert");
        }
    });
}
</script>

<?php require __DIR__ . "/includes/footer.php"; ?>
