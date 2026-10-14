// assets/charts.js — 部署报告图表
(function () {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();

  // ---- 图 2：验证期累计耗电 实测 vs 理论 ----
  var el = document.getElementById('chart-verify');
  if (el && window.echarts) {
    var chart = echarts.init(el, null, { renderer: 'svg' });
    chart.setOption({
      animation: false,
      color: [accent, accent2],
      tooltip: {
        trigger: 'axis',
        appendToBody: true,
        valueFormatter: function (v) { return Number(v).toFixed(3) + ' Wh'; }
      },
      legend: {
        top: 0,
        textStyle: { color: muted, fontSize: 12 },
        itemWidth: 18, itemHeight: 3
      },
      grid: { left: 56, right: 24, top: 36, bottom: 42 },
      xAxis: {
        type: 'value',
        name: '运行时间（分钟）',
        nameLocation: 'middle', nameGap: 26,
        nameTextStyle: { color: muted, fontSize: 12 },
        min: 0, max: 6,
        axisLabel: { color: muted, fontSize: 12 },
        axisLine: { lineStyle: { color: rule } },
        splitLine: { lineStyle: { color: rule } }
      },
      yAxis: {
        type: 'value',
        name: '累计耗电（Wh）',
        nameLocation: 'middle', nameGap: 42,
        nameTextStyle: { color: muted, fontSize: 12 },
        min: 0, max: 3.2,
        axisLabel: { color: muted, fontSize: 12 },
        axisLine: { lineStyle: { color: rule } },
        splitLine: { lineStyle: { color: rule } }
      },
      series: [
        {
          name: '实测积分累计',
          type: 'line',
          symbol: 'circle', symbolSize: 8,
          label: { show: true, position: 'top', color: ink, fontSize: 11,
                   formatter: function (p) { return p.value[1].toFixed(3); } },
          lineStyle: { width: 3 },
          data: [[0, 0], [0.80, 0.524], [4.55, 2.4798], [5.62, 2.978]]
        },
        {
          name: '理论值（31W 恒功率）',
          type: 'line',
          symbol: 'none',
          lineStyle: { width: 2, type: 'dashed' },
          data: [[0, 0], [5.62, 2.90]]
        }
      ]
    });
    window.addEventListener('resize', function () { chart.resize(); });
  }
})();
