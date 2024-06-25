const plugin = {
    id: 'customCanvasBackgroundColor',
    beforeDraw: (chart, args, options) => {
        const { ctx } = chart;
        ctx.save();
        ctx.globalCompositeOperation = 'destination-over';
        ctx.fillStyle = options.color || '#99ffff';
        ctx.fillRect(0, 0, chart.width, chart.height);
        ctx.restore();
    }
};

function createTeamChart(teamStats, teamName) {
    const ctx = document.getElementById('teamChart').getContext('2d');
    const isDarkMode = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;

    const players = [...new Set(teamStats.map(stat => stat.name))];
    const datasets = players.map(player => {
        const playerStats = teamStats.filter(stat => stat.name === player);
        return {
            label: player,
            data: playerStats.map(stat => ({
                x: stat.date,
                y: stat.oaa,
                id: stat.player_id
            })),
            fill: false,
            borderWidth: 1,
            pointRadius: 5,
            pointHoverRadius: 7,
        };
    });

    if (isDarkMode) {
        Chart.defaults.color = '#ffffff';
    }

    const chart = new Chart(ctx, {
        type: 'line',
        data: {
            datasets: datasets
        },
        options: {
            scales: {
                x: {
                    type: 'time',
                    time: {
                        unit: 'day',
                        tooltipFormat: 'MMM d, yyyy',
                    },
                    grid: {
                        color: isDarkMode ? '#323232' : '#E5E5E5',
                    },
                    ticks: {
                        align: 'start',
                    }
                },
                y: {
                    ticks: {
                        stepSize: 1,
                    },
                    grid: {
                        color: isDarkMode ? '#323232' : '#E5E5E5',
                    },
                }
            },
            plugins: {
                customCanvasBackgroundColor: {
                    color: isDarkMode ? '#181818' : 'white',
                },
                title: {
                    display: true,
                    text: `${teamName} OAA Over Time`,
                    font: {
                        size: 20
                    }
                },
                legend: {
                    onClick: (e, legendItem, legend) => {
                        window.open('/player/' + teamStats.filter(stat => stat.name === legendItem.text)[0].player_id, '_blank')
                    },
                },
                datalabels: {
                    backgroundColor: function (context) {
                        return context.dataset.backgroundColor;
                    },
                    borderRadius: 4,
                    color: 'white',
                    font: {
                        weight: 'bold'
                    },
                    padding: 6,
                    anchor: 'center',
                    align: 'right',
                    offset: 10,
                    border: 10,
                    display: function (context) {
                        return context.dataset.data.length - 1 === context.dataIndex ? 'auto' : false;
                    },
                    color: isDarkMode ? '#ffffff' : '#000000',
                    font: {
                        size: 14
                    },
                    formatter: function (value, context) {
                        return context.dataset.label + ': ' + value.y;
                    }
                }
            },
            onClick: (e, elements) => {
                if (elements[0]) {
                    const i = elements[0].datasetIndex;
                    window.open('/player/' + teamStats.filter(stat => stat.name === chart.data.datasets[i].label)[0].player_id, '_blank')
                }
            },
        },
        plugins: [plugin, ChartDataLabels],
        maintainAspectRatio: false,
    });

    return chart;
}

function createSparklines(sparklinesData) {
    const sparklinesDataMap = new Map(Object.entries(sparklinesData));
    sparklinesDataMap.forEach((stats, player) => {
        new Chart(document.getElementById('playerChart' + player).getContext('2d'), {
            type: 'line',
            data: {
                labels: stats.OAAHistory.map(obj => obj.Date),
                datasets: [
                    {
                        data: stats.OAAHistory.map(obj => obj.OAA),
                        fill: true,
                        borderWidth: 1,
                        pointRadius: 0,
                        pointHoverRadius: 0,
                    }
                ]
            },
            options: {
                scales: {
                    x: { display: false },
                    y: { display: false }
                },
                elements: {
                    line: { tension: 0 }
                },
                plugins: {
                    legend: { display: false },
                    tooltip: { enabled: false }
                }
            }
        });
    });
}

document.addEventListener('DOMContentLoaded', () => {
    const chart = createTeamChart(teamStats, teamName);
    createSparklines(sparklinesData);

    document.getElementById('downloadChart').addEventListener('click', function () {
        const link = document.createElement('a');
        link.href = chart.toBase64Image();
        link.download = `${teamName}_OAA_Chart.png`;
        link.click();
    });
});