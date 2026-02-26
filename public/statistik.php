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
$pageTitle = "Statistik – KonsensOmat";
require __DIR__ . "/includes/header.php";

$dir = dirname(__DIR__) . "/files/data";
$files = scandir($dir);

$validSurveys = [];
$dailyCount = [];
$weekdayCount = array_fill(1,7,0);
$hourBlocks = [0,0,0,0];
$participantDistribution = [];

$activeSurveys = 0;
$activeValid = 0;
$activeNotValid = 0;
$expiringSoon = 0;

$now = time();
$sevenDays = 7 * 24 * 60 * 60;
$sixDays = 6 * 24 * 60 * 60;

foreach ($files as $file) {

    if (pathinfo($file, PATHINFO_EXTENSION) !== "json") continue;

    $path = $dir . "/" . $file;
    $content = json_decode(file_get_contents($path), true);

    if (!$content || !isset($content["created"])) continue;

    $age = $now - $content["created"];

    if ($age > $sevenDays) {
        unlink($path);
        continue;
    }

    $activeSurveys++;

    if ($age > $sixDays) $expiringSoon++;

    $participants = isset($content["votes"]) ? count($content["votes"]) : 0;

    if ($participants >= 2) {
        $activeValid++;
        $validSurveys[] = $participants;

        $date = date("Y-m-d", $content["created"]);
        if (!isset($dailyCount[$date])) $dailyCount[$date] = 0;
        $dailyCount[$date]++;

        $weekday = date("N", $content["created"]);
        $weekdayCount[$weekday]++;

        $hour = date("G", $content["created"]);
        if ($hour < 6) $hourBlocks[0]++;
        elseif ($hour < 12) $hourBlocks[1]++;
        elseif ($hour < 18) $hourBlocks[2]++;
        else $hourBlocks[3]++;

        if (!isset($participantDistribution[$participants])) {
            $participantDistribution[$participants] = 0;
        }
        $participantDistribution[$participants]++;
    } else {
        $activeNotValid++;
    }
}

$totalValid = count($validSurveys);
$avgParticipants = $totalValid ? round(array_sum($validSurveys)/$totalValid,2) : 0;
$maxParticipants = $totalValid ? max($validSurveys) : 0;

arsort($participantDistribution);
$maxBar = max(array_merge($participantDistribution,$weekdayCount,$hourBlocks,[1]));
?>

<h2>Übersicht über aktive Umfragen</h2>

<div class="card">
<p><strong><?= $activeSurveys ?></strong> aktive Umfragen</p>
<p><strong><?= $activeValid ?></strong> davon Umfragen mit min 2 Benutzer*innen</p>
<p><strong><?= $activeNotValid ?></strong> Umfragen mit weniger als 2 Benutzer*innen</p>
<p><strong><?= $expiringSoon ?></strong> Umfragen laufen bald ab</p>
<p><strong><?= $avgParticipants ?></strong> durchschnittliche Benutzer*innen pro Umfrage</p>
<p><strong><?= $maxParticipants ?></strong> größte Umfrage hat Benutzer*innen</p>
</div>

<h2>Aktivität Heatmap</h2>

<div class="card">
<div class="heatmap">
<?php
for($i=179;$i>=0;$i--){
    $date=date("Y-m-d",strtotime("-$i days"));
    $count=isset($dailyCount[$date])?$dailyCount[$date]:0;

    $class="cell";
    if($count==1)$class.=" l1";
    elseif($count==2)$class.=" l2";
    elseif($count==3)$class.=" l3";
    elseif($count>=4)$class.=" l4";

    echo "<div class='$class' title='$date: $count'></div>";
}
?>
</div>
</div>

<h2>Verteilung nach Benutzer*innenzahl</h2>

<div class="card">
<?php foreach ($participantDistribution as $count => $freq): ?>
<div class="bar-container">
<?= $count ?> Benutzer*innen
<div class="bar" style="width:<?= ($freq/$maxBar)*100 ?>%"></div>
</div>
<?php endforeach; ?>
</div>

<h2>Erstellung nach Wochentag</h2>

<div class="card">
<?php
$days = ["Mo","Di","Mi","Do","Fr","Sa","So"];
foreach ($weekdayCount as $i => $val):
?>
<div class="bar-container">
<?= $days[$i-1] ?>
<div class="bar" style="width:<?= ($val/$maxBar)*100 ?>%"></div>
</div>
<?php endforeach; ?>
</div>

<h2>Nutzung über den Tag</h2>

<div class="card">
<?php
$labels = ["0–6 Uhr","6–12 Uhr","12–18 Uhr","18–24 Uhr"];
foreach ($hourBlocks as $i => $val):
?>
<div class="bar-container">
<?= $labels[$i] ?>
<div class="bar" style="width:<?= ($val/$maxBar)*100 ?>%"></div>
</div>
<?php endforeach; ?>
</div>

<p style="text-align:center;margin-top:30px;">
<a href="index.php">← Zurück zur Startseite</a>
</p>

<?php require __DIR__ . "/includes/footer.php"; ?>
