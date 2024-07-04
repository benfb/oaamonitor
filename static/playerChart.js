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

function createPlayerChart(playerStats, playerName, playerPosition) {
  const ctx = document.getElementById("playerChart").getContext("2d");
  const isDarkMode =
    window.matchMedia &&
    window.matchMedia("(prefers-color-scheme: dark)").matches;

  const labels = playerStats.map((stat) => stat.date);
  const data = playerStats.map((stat) => stat.oaa);

  if (isDarkMode) {
    Chart.defaults.color = "#ffffff";
  }

  const chart = new Chart(ctx, {
    type: "line",
    data: {
      labels: labels,
      datasets: [
        {
          label: "OAA",
          data: data,
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
            color: isDarkMode ? "#323232" : "#E5E5E5",
          },
        },
        y: {
          ticks: {
            stepSize: 1,
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
          text: playerPosition !== 'N/A' ? `${playerName} (${playerPosition}) OAA Over Time` : `${playerName} OAA Over Time`,
          font: {
            size: 20,
          },
        },
      },
    },
    plugins: [plugin],
  });

  return chart;
}

document.addEventListener("DOMContentLoaded", () => {
  const chart = createPlayerChart(playerStats, playerName, playerPosition);

  document
    .getElementById("downloadChart")
    .addEventListener("click", function () {
      const link = document.createElement("a");
      link.href = chart.toBase64Image();
      link.download = `${playerName}_OAA_Chart.png`;
      link.click();
    });
});
