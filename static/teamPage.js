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

const colorPalette = [
  "#FF6384",
  "#36A2EB",
  "#FFCE56",
  "#4BC0C0",
  "#9966FF",
  "#FF9F40",
  "#FF6B6B",
  "#4ECDC4",
  "#45B7D1",
  "#F7464A",
  "#46BFBD",
  "#FDB45C",
  "#949FB1",
  "#4D5360",
  "#1ABC9C",
  "#2ECC71",
  "#3498DB",
  "#9B59B6",
  "#E67E22",
  "#E74C3C",
  "#95A5A6",
  "#34495E",
  "#D35400",
  "#8E44AD",
];

const prefersDarkMode =
  window.matchMedia &&
  window.matchMedia("(prefers-color-scheme: dark)").matches;

if (prefersDarkMode) {
  Chart.defaults.color = "#ffffff";
}

function buildDatasets(teamStats) {
  const players = new Map();

  teamStats.forEach((stat) => {
    const existing = players.get(stat.player_id) || {
      name: stat.name,
      entries: [],
    };
    existing.entries.push({
      x: stat.date,
      y: stat.oaa,
    });
    players.set(stat.player_id, existing);
  });

  return Array.from(players.entries()).map(([playerId, value], index) => ({
    label: value.name,
    playerId,
    data: value.entries,
    fill: false,
    borderWidth: 1,
    pointRadius: 3,
    pointHoverRadius: 5,
    borderColor: colorPalette[index % colorPalette.length],
    backgroundColor: colorPalette[index % colorPalette.length],
  }));
}

function createTeamChart(ctx, teamName, stats) {
  const isMobile = window.innerWidth < 768;

  return new Chart(ctx, {
    type: "line",
    data: {
      datasets: buildDatasets(stats),
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      aspectRatio: isMobile ? 1 : 2,
      layout: {
        padding: {
          top: 10,
          right: 20,
          bottom: 10,
          left: 10,
        },
      },
      scales: {
        x: {
          type: "time",
          time: {
            unit: "day",
            tooltipFormat: "MMM d, yyyy",
            displayFormats: {
              day: "MMM d",
            },
          },
          grid: {
            color: prefersDarkMode ? "#323232" : "#E5E5E5",
          },
          ticks: {
            maxRotation: 45,
            minRotation: 45,
            autoSkip: true,
            maxTicksLimit: isMobile ? 5 : 10,
            font: {
              size: isMobile ? 10 : 12,
            },
          },
        },
        y: {
          ticks: {
            stepSize: 1,
            font: {
              size: isMobile ? 10 : 12,
            },
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
          text: `${teamName} OAA Over Time`,
          font: {
            size: isMobile ? 16 : 20,
          },
        },
        legend: {
          position: "top",
          labels: {
            boxWidth: isMobile ? 10 : 40,
            font: {
              size: isMobile ? 10 : 12,
            },
            usePointStyle: true,
            pointStyle: "circle",
          },
          onClick: (_event, legendItem, legend) => {
            const dataset = legend.chart.data.datasets[legendItem.datasetIndex];
            if (!dataset?.playerId) {
              return;
            }
            window.open(`/player/${dataset.playerId}`, "_blank");
          },
        },
        datalabels: {
          display: false,
        },
      },
      onClick: (event, elements, chart) => {
        if (!elements.length) {
          return;
        }
        const datasetIndex = elements[0].datasetIndex;
        const dataset = chart.data.datasets[datasetIndex];
        if (dataset?.playerId) {
          window.open(`/player/${dataset.playerId}`, "_blank");
        }
      },
    },
    plugins: [backgroundPlugin, ChartDataLabels],
  });
}

let sparklineCharts = [];

function destroySparklines() {
  sparklineCharts.forEach((chart) => chart.destroy());
  sparklineCharts = [];
}

function renderTeamTable(tbody, players, season) {
  if (!tbody) return;

  destroySparklines();
  tbody.innerHTML = "";

  players.forEach((player) => {
    const row = document.createElement("tr");

    const nameCell = document.createElement("td");
    const link = document.createElement("a");
    link.href = `/player/${player.PlayerID}?season=${season}`;
    link.textContent = player.Name;
    nameCell.appendChild(link);

    const positionCell = document.createElement("td");
    positionCell.textContent = player.Position;

    const oaaCell = document.createElement("td");
    oaaCell.textContent = player.LatestOAA;

    const sparklineCell = document.createElement("td");
    const canvas = document.createElement("canvas");
    canvas.width = 160;
    canvas.height = 40;
    sparklineCell.appendChild(canvas);

    row.appendChild(nameCell);
    row.appendChild(positionCell);
    row.appendChild(oaaCell);
    row.appendChild(sparklineCell);

    tbody.appendChild(row);

    const ctx = canvas.getContext("2d");
    const history = Array.isArray(player.OAAHistory) ? player.OAAHistory : [];
    const chart = new Chart(ctx, {
      type: "line",
      data: {
        labels: history.map((point) => point.Date),
        datasets: [
          {
            data: history.map((point) => point.OAA),
            fill: false,
            borderWidth: 1,
            borderColor: "#4BC0C0",
            pointRadius: 0,
            pointHoverRadius: 0,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: { enabled: false },
        },
        scales: {
          x: { display: false },
          y: { display: false },
        },
      },
    });

    sparklineCharts.push(chart);
  });
}

document.addEventListener("DOMContentLoaded", () => {
  const data = window.teamPageData || {};
  const seasons = Array.isArray(data.seasons) ? data.seasons : [];
  const selectEl = document.getElementById("seasonSelect");
  const savantLink = document.getElementById("savantLink");
  const downloadButton = document.getElementById("downloadChart");
  const chartCtx = document.getElementById("teamChart")?.getContext("2d");
  const tableBody = document.getElementById("teamPlayersTableBody");

  if (!chartCtx || !seasons.length) {
    return;
  }

  const getSeasonKey = (season) => String(season);
  const getStatsForSeason = (season) =>
    data.teamStatsBySeason?.[getSeasonKey(season)] || [];
  const getSparklinesForSeason = (season) =>
    data.sparklinesBySeason?.[getSeasonKey(season)] || [];

  const urlParams = new URLSearchParams(window.location.search);
  const paramSeason = urlParams.get("season");

  const resolveSeason = () => {
    if (paramSeason && seasons.includes(Number(paramSeason))) {
      return Number(paramSeason);
    }
    return data.selectedSeason || seasons[0];
  };

  let currentSeason = resolveSeason();
  if (selectEl) {
    selectEl.value = String(currentSeason);
  }

  const updateSavantLink = (season) => {
    if (!savantLink) return;
    const base =
      "https://baseballsavant.mlb.com/leaderboard/outs_above_average";
    const params = new URLSearchParams({
      type: "Fielder",
      startYear: String(season),
      endYear: String(season),
      split: "yes",
      team: data.teamAbbreviation || "",
      range: "year",
      min: "10",
      pos: "",
      roles: "",
      viz: "hide",
    });
    savantLink.href = `${base}?${params.toString()}`;
  };

  const chart = createTeamChart(
    chartCtx,
    data.teamName,
    getStatsForSeason(currentSeason),
  );

  chart.options.onClick = (_event, elements) => {
    if (!elements.length) {
      return;
    }
    const datasetIndex = elements[0].datasetIndex;
    const dataset = chart.data.datasets[datasetIndex];
    if (dataset?.playerId) {
      window.open(
        `/player/${dataset.playerId}?season=${currentSeason}`,
        "_blank",
      );
    }
  };

  if (chart.options.plugins?.legend) {
    chart.options.plugins.legend.onClick = (_event, legendItem) => {
      const dataset = chart.data.datasets[legendItem.datasetIndex];
      if (dataset?.playerId) {
        window.open(
          `/player/${dataset.playerId}?season=${currentSeason}`,
          "_blank",
        );
      }
    };
  }

  renderTeamTable(tableBody, getSparklinesForSeason(currentSeason), currentSeason);
  updateSavantLink(currentSeason);

  const updateSeason = (season, pushState = true) => {
    currentSeason = Number(season);
    const stats = getStatsForSeason(currentSeason);

    chart.data.datasets = buildDatasets(stats);
    chart.update();

    renderTeamTable(
      tableBody,
      getSparklinesForSeason(currentSeason),
      currentSeason,
    );
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
      link.download = `${data.teamName}_OAA_Chart.png`;
      link.click();
    });
  }

  window.addEventListener("resize", () => {
    const isMobile = window.innerWidth < 768;
    chart.options.scales.x.ticks.maxTicksLimit = isMobile ? 5 : 10;
    chart.options.aspectRatio = isMobile ? 1 : 2;
    chart.update();
  });
});
