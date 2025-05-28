google.charts.load('current', { 'packages': ['corechart'] });
google.charts.setOnLoadCallback(drawChart);

const chartID = 'network-chart';
let chart, options;


function drawChart() {
    options = {
        title: 'Network Latency',
        legend: { position: 'bottom' },
        hAxis: {
            slantedText: true,
            slantedTextAngle: 45,
            showTextEvery: 25,
            textStyle: { fontSize: 14, color: '#e5ecf2' },
        },
        vAxis: {
            textStyle: { color: '#e5ecf2' }
        },
        pointsVisible: true,
        pointSize: 2,
        lineWidth: 2,
        titleTextStyle: { color: '#e5ecf2' },
        legendTextStyle: { color: '#e5ecf2' },
        backgroundColor: '#0c0b17',
        interpolateNulls: false,
        vAxes: {
            0: {
                textStyle: { color: '#e5ecf2' },
                format: '# ms',
                gridlines: { color: '#333', minSpacing: 100 },
            },
            1: {
                minValue: 0,
                maxValue: 100,
                viewWindow: { min: 1, max: 100 },
                textStyle: { color: '#e5ecf2' },
                format: '#\'%\'',
                gridlines: { color: 'transparent' },
            }
        },
        crosshair: { trigger: 'both', orientation: 'vertical', color: '#e5ecf2' },
        focusTarget: 'category',
        tooltip: { trigger: 'both' },
        series: {
            0: { color: 'blue', targetAxisIndex: 0, areaOpacity: 0 },
            1: { color: 'green', targetAxisIndex: 0, areaOpacity: 0 },
            2: { color: 'red', targetAxisIndex: 1, areaOpacity: 1 },
        }
    };

    chart = new google.visualization.AreaChart(document.getElementById(chartID));
    setInterval(fetchAndDraw, 1000); // Fetch every 1s
}

function fetchAndDraw() {
    const start = encodeURIComponent(document.getElementById('start').value);
    const end = encodeURIComponent(document.getElementById('end').value);
    const url = `/data?start=${start}&end=${end}`;
    fetch(url)
        .then(response => response.json())
        .then(jsonData => {
            if (jsonData.length > 1) {
                data = google.visualization.arrayToDataTable(jsonData);
                chart.draw(data, options);
            }
        })
        .catch(err => {
            console.error('Failed to fetch chart data:', err);
        });
}