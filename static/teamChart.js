const plugin = {
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

function createTeamChart(teamStats, teamName) {
  const ctx = document.getElementById("teamChart").getContext("2d");
  const isDarkMode =
    window.matchMedia &&
    window.matchMedia("(prefers-color-scheme: dark)").matches;
  const isMobile = window.innerWidth < 768;

  const players = [...new Set(teamStats.map((stat) => stat.name))];
  const datasets = players.map((player, index) => {
    const playerStats = teamStats.filter((stat) => stat.name === player);
    return {
      label: player,
      data: playerStats.map((stat) => ({
        x: stat.date,
        y: stat.oaa,
        id: stat.player_id,
      })),
      fill: false,
      borderWidth: 1,
      pointRadius: 3,
      pointHoverRadius: 5,
      borderColor: colorPalette[index % colorPalette.length],
      backgroundColor: colorPalette[index % colorPalette.length],
    };
  });

  if (isDarkMode) {
    Chart.defaults.color = "#ffffff";
  }

  const chart = new Chart(ctx, {
    type: "line",
    data: {
      datasets: datasets,
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
            color: isDarkMode ? "#323232" : "#E5E5E5",
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
            color: isDarkMode ? "#323232" : "#E5E5E5",
          },
        },
      },
      plugins: {
        customCanvasBackgroundColor: {
          color: isDarkMode ? "#181818" : "white",
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
          onClick: (e, legendItem, legend) => {
            window.open(
              "/player/" +
                teamStats.filter((stat) => stat.name === legendItem.text)[0]
                  .player_id,
              "_blank",
            );
          },
        },
        datalabels: {
          display: false,
        },
      },
      onClick: (e, elements) => {
        if (elements[0]) {
          const i = elements[0].datasetIndex;
          window.open(
            "/player/" +
              teamStats.filter(
                (stat) => stat.name === chart.data.datasets[i].label,
              )[0].player_id,
            "_blank",
          );
        }
      },
    },
    plugins: [plugin, ChartDataLabels],
  });

  return chart;
}

function abbreviateName(name) {
  const parts = name.split(" ");
  if (parts.length > 1) {
    return `${parts[0][0]}. ${parts[parts.length - 1]}`;
  }
  return name;
}

function createSparklines(sparklinesData) {
  const sparklinesDataMap = new Map(Object.entries(sparklinesData));
  sparklinesDataMap.forEach((stats, player) => {
    const canvas = document.getElementById("playerChart" + player);
    new Chart(canvas.getContext("2d"), {
      type: "line",
      data: {
        labels: stats.OAAHistory.map((obj) => obj.Date),
        datasets: [
          {
            data: stats.OAAHistory.map((obj) => obj.OAA),
            fill: true,
            borderWidth: 1,
            pointRadius: 0,
            pointHoverRadius: 0,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          x: { display: false },
          y: { display: false },
        },
        plugins: {
          legend: { display: false },
          tooltip: { enabled: false },
        },
      },
    });
  });
}

document.addEventListener("DOMContentLoaded", () => {
  const chart = createTeamChart(teamStats, teamName);
  createSparklines(sparklinesData);

  document
    .getElementById("downloadChart")
    .addEventListener("click", function () {
      const link = document.createElement("a");
      link.href = chart.toBase64Image();
      link.download = `${teamName}_OAA_Chart.png`;
      link.click();
    });

  // Adjust chart size on window resize
  window.addEventListener("resize", () => {
    const isMobile = window.innerWidth < 768;
    chart.options.scales.x.ticks.maxTicksLimit = isMobile ? 5 : 10;
    chart.options.aspectRatio = isMobile ? 1 : 2;
    chart.update();
  });
});
