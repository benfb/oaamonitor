const backgroundPlugin = {
  id: "customCanvasBackgroundColor",
  beforeDraw: (chart, args, options) => {
    const { ctx } = chart;
    ctx.save();
    ctx.globalCompositeOperation = "destination-over";
    ctx.fillStyle = options.color || "#99ffff";
    ctx.fillRect(0, 0, chart.width, chart.height);
    ctx.restore();
  },
};

const prefersDarkMode =
  window.matchMedia &&
  window.matchMedia("(prefers-color-scheme: dark)").matches;

if (prefersDarkMode) {
  Chart.defaults.color = "#ffffff";
}

function formatStats(stats) {
  return stats.map((stat) => ({
    date: stat.date,
    oaa: stat.oaa,
  }));
}

function createChart(ctx, playerName, position, stats) {
  const formatted = formatStats(stats);

  return new Chart(ctx, {
    type: "line",
    data: {
      labels: formatted.map((stat) => stat.date),
      datasets: [
        {
          label: "OAA",
          data: formatted.map((stat) => stat.oaa),
          backgroundColor: "rgba(75, 192, 192, 0.5)",
          borderColor: "rgba(75, 192, 192, 1)",
          borderWidth: 1,
          fill: false,
          pointRadius: 5,
          pointHoverRadius: 7,
        },
      ],
    },
    options: {
      scales: {
        x: {
          type: "time",
          time: {
            unit: "day",
            tooltipFormat: "MMM d, yyyy",
          },
          grid: {
            color: prefersDarkMode ? "#323232" : "#E5E5E5",
          },
        },
        y: {
          ticks: {
            stepSize: 1,
          },
          grid: {
            color: prefersDarkMode ? "#323232" : "#E5E5E5",
          },
        },
      },
      plugins: {
        customCanvasBackgroundColor: {
          color: prefersDarkMode ? "#181818" : "white",
        },
        title: {
          display: true,
          text:
            position && position !== "N/A"
              ? `${playerName} (${position}) OAA Over Time`
              : `${playerName} OAA Over Time`,
          font: {
            size: 20,
          },
        },
      },
    },
    plugins: [backgroundPlugin],
  });
}

document.addEventListener("DOMContentLoaded", () => {
  const data = window.playerPageData || {};
  const seasons = Array.isArray(data.seasons) ? data.seasons : [];
  const selectEl = document.getElementById("seasonSelect");
  const savantLink = document.getElementById("savantLink");
  const downloadButton = document.getElementById("downloadChart");
  const ctx = document.getElementById("playerChart").getContext("2d");

  if (!seasons.length || !selectEl) {
    return;
  }

  const urlParams = new URLSearchParams(window.location.search);
  const paramSeason = urlParams.get("season");

  const getSeasonKey = (season) => String(season);

  const resolveSeason = () => {
    if (paramSeason && seasons.includes(Number(paramSeason))) {
      return Number(paramSeason);
    }
    return data.selectedSeason || seasons[0];
  };

  let currentSeason = resolveSeason();
  selectEl.value = String(currentSeason);

  const getStatsForSeason = (season) => {
    const key = getSeasonKey(season);
    return data.playerStatsBySeason?.[key] || [];
  };

  const getPositionForSeason = (season) => {
    const key = getSeasonKey(season);
    return data.playerPositions?.[key] || "N/A";
  };

  const updateSavantLink = (season) => {
    if (!savantLink) return;
    const base =
      "https://baseballsavant.mlb.com/visuals/statcast-infield-defense";
    const params = new URLSearchParams({
      type: "Fielder",
      playerId: String(data.playerID),
      startYear: String(season),
      endYear: String(season),
      result: "",
      direction: "",
      normalize: "undefined",
      roles: "",
      esrGT: "0",
      esrLT: "1",
      evGT: "0",
      evLT: "125",
      distGT: "0",
      distLT: "200",
      batside: "",
      viz: "intercept_fielder_starting_position_",
    });
    savantLink.href = `${base}?${params.toString()}`;
  };

  let chart = createChart(
    ctx,
    data.playerName,
    getPositionForSeason(currentSeason),
    getStatsForSeason(currentSeason),
  );
  updateSavantLink(currentSeason);

  const updateSeason = (season, pushState = true) => {
    currentSeason = Number(season);
    const stats = getStatsForSeason(currentSeason);
    const position = getPositionForSeason(currentSeason);

    chart.data.labels = stats.map((stat) => stat.date);
    chart.data.datasets[0].data = stats.map((stat) => stat.oaa);
    chart.options.plugins.title.text =
      position && position !== "N/A"
        ? `${data.playerName} (${position}) OAA Over Time`
        : `${data.playerName} OAA Over Time`;
    chart.update();

    updateSavantLink(currentSeason);

    if (pushState) {
      const params = new URLSearchParams(window.location.search);
      params.set("season", currentSeason);
      const newRelativePathQuery =
        window.location.pathname + "?" + params.toString();
      window.history.replaceState(null, "", newRelativePathQuery);
    }
  };

  if (selectEl) {
    selectEl.addEventListener("change", (event) => {
      updateSeason(event.target.value);
    });
  }

  if (downloadButton) {
    downloadButton.addEventListener("click", () => {
      const link = document.createElement("a");
      link.href = chart.toBase64Image();
      link.download = `${data.playerName}_OAA_Chart.png`;
      link.click();
    });
  }
});
