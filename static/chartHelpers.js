(function () {
  const prefersDarkMode =
    window.matchMedia &&
    window.matchMedia("(prefers-color-scheme: dark)").matches;

  if (prefersDarkMode) {
    Chart.defaults.color = "#f0f4f8";
  }

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

  window.OAAMonitorCharts = {
    backgroundPlugin,
    chartBackgroundColor: prefersDarkMode ? "#111318" : "#ffffff",
    gridColor: prefersDarkMode ? "#1f2229" : "#e5e7eb",
    prefersDarkMode,
  };
})();
