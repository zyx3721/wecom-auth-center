package handler

// statusHTML 监控页单文件前端：KMS 风格布局，页面内自绘 SVG 折线图，30 秒自动刷新
const statusHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>认证中心 运行状态与监控</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: system-ui, -apple-system, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  background: linear-gradient(135deg, #eef2ff 0%, #f0fdfa 100%); min-height: 100vh; color: #1f2937; }
.wrap { max-width: 1280px; margin: 0 auto; padding: 28px 24px 16px; }
header { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; margin-bottom: 24px; }
.brand { display: flex; align-items: center; gap: 14px; }
.logo { width: 46px; height: 46px; border-radius: 12px; background: linear-gradient(135deg, #6366f1, #8b5cf6);
  color: #fff; font-weight: 700; font-size: 24px; display: flex; align-items: center; justify-content: center; }
.brand h1 { font-size: 22px; }
.brand p { font-size: 11px; letter-spacing: 2px; color: #9ca3af; }
.badges { display: flex; gap: 8px; flex-wrap: wrap; }
.pill { background: #fff; border-radius: 999px; padding: 7px 14px; font-size: 13px; color: #6b7280; box-shadow: 0 1px 3px rgba(0,0,0,.06); }
.pill b { color: #1f2937; }
.pill.ok { color: #047857; background: #d1fae5; }
.pill.bad { color: #b91c1c; background: #fee2e2; }
.pill.mini { padding: 4px 10px; font-size: 12px; }
.cards { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 20px; }
.card { background: #fff; border-radius: 16px; padding: 18px 20px; box-shadow: 0 1px 4px rgba(31,41,55,.06); }
.stat .label { font-size: 13px; color: #6b7280; }
.stat .num { font-size: 34px; font-weight: 700; margin: 6px 0 2px; }
.stat .sub { font-size: 12px; color: #9ca3af; }
.indigo { color: #6366f1; } .green { color: #059669; } .orange { color: #d97706; } .red { color: #dc2626; }
.grid { display: grid; grid-template-columns: 2fr 1fr; gap: 16px; }
.card-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.card-head h2 { font-size: 15px; }
.muted { font-size: 12px; color: #9ca3af; }
.legend { font-size: 12px; color: #6b7280; }
.legend .dot, .dot-green { display: inline-block; width: 9px; height: 9px; border-radius: 50%; margin: 0 4px 0 10px; }
.dot.indigo { background: #6366f1; } .dot.green { background: #10b981; }
.dot-green { background: #10b981; width: 11px; height: 11px; margin: 0 6px 0 0; }
.dot-green.off { background: #ef4444; }
#chart svg { width: 100%; height: auto; display: block; }
.foot-note { font-size: 12px; color: #9ca3af; margin-top: 8px; }
.node-name { font-size: 15px; margin-bottom: 10px; }
.pills { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 14px; }
.mini-stats { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
.mini-stats div { background: #f9fafb; border-radius: 10px; padding: 10px 6px; text-align: center; }
.mini-stats p { font-size: 11px; color: #6b7280; margin-bottom: 4px; }
.mini-stats b { font-size: 17px; }
.info { width: 100%; border-collapse: collapse; font-size: 13px; }
.info td { padding: 7px 4px; }
.info td:nth-child(odd) { color: #9ca3af; width: 64px; }
.info td:nth-child(even) { color: #1f2937; font-weight: 600; word-break: break-all; }
footer { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 8px; font-size: 12px; color: #9ca3af; padding: 16px 4px; }
@media (max-width: 960px) { .cards { grid-template-columns: repeat(2, 1fr); } .grid { grid-template-columns: 1fr; } }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <div class="brand">
      <div class="logo">W</div>
      <div>
        <h1>认证中心 运行状态与监控</h1>
        <p>WECOM AUTH CENTER STATUS &amp; MONITORING</p>
      </div>
    </div>
    <div class="badges">
      <span class="pill ok" id="service-pill">● 服务运行中</span>
      <span class="pill">服务器 <b id="hostname">-</b></span>
      <span class="pill">时区 <b id="tz">-</b></span>
    </div>
  </header>

  <section class="cards">
    <div class="card stat"><p class="label">近 7 天登录发起</p><p class="num indigo" id="c-logins7d">-</p><p class="sub">login 请求总数</p></div>
    <div class="card stat"><p class="label">近 7 天登录成功</p><p class="num green" id="c-tickets7d">-</p><p class="sub">ticket 签发总数</p></div>
    <div class="card stat"><p class="label">今日登录发起</p><p class="num orange" id="c-loginsToday">-</p><p class="sub">今日 login 请求</p></div>
    <div class="card stat"><p class="label">今日兑换成功</p><p class="num red" id="c-verifyOkToday">-</p><p class="sub">verify 换取身份次数</p></div>
  </section>

  <main class="grid">
    <div class="card">
      <div class="card-head">
        <h2>近 7 天登录趋势</h2>
        <div class="legend"><span class="dot indigo"></span>登录发起<span class="dot green"></span>登录成功</div>
      </div>
      <div id="chart"><p class="foot-note">暂无数据</p></div>
      <p class="foot-note">近 7 天按日统计数据</p>
    </div>
    <aside class="side" style="display:flex;flex-direction:column;gap:16px;">
      <div class="card">
        <div class="card-head"><h2>监控节点</h2><span class="muted">今日实时</span></div>
        <p class="node-name"><span class="dot-green" id="storage-dot"></span><b id="hostname2">-</b></p>
        <div class="pills">
          <span class="pill mini" id="storage-pill">存储 -</span>
          <span class="pill mini" id="mode-pill">企微 -</span>
        </div>
        <div class="mini-stats">
          <div><p>今日拒绝</p><b id="r-total">0</b></div>
          <div><p>state 失败</p><b id="r-state">0</b></div>
          <div><p>签名失败</p><b id="r-sign">0</b></div>
          <div><p>ticket 拒绝</p><b id="r-ticket">0</b></div>
        </div>
      </div>
      <div class="card">
        <div class="card-head"><h2>服务信息</h2></div>
        <table class="info">
          <tr><td>版本</td><td id="i-version">-</td><td>平台</td><td id="i-platform">-</td></tr>
          <tr><td>构建</td><td id="i-commit">-</td><td>Go</td><td id="i-go">-</td></tr>
          <tr><td>构建时间</td><td id="i-build">-</td><td>运行时长</td><td id="i-uptime">-</td></tr>
          <tr><td>监听</td><td id="i-listen">-</td><td>检测时间</td><td id="i-time">-</td></tr>
        </table>
      </div>
    </aside>
  </main>

  <footer>
    <span id="mock-note"></span>
    <span>最后更新 <b id="updated">-</b> · 每 30 秒自动刷新</span>
  </footer>
</div>
<script>
(function () {
  var qs = new URLSearchParams(location.search);
  var token = qs.get('token') || localStorage.getItem('wac_status_token') || '';
  if (token) { localStorage.setItem('wac_status_token', token); }
  if (!token) {
    var p = document.getElementById('service-pill');
    p.textContent = '● 缺少访问令牌'; p.className = 'pill bad';
    return;
  }
  document.getElementById('tz').textContent = Intl.DateTimeFormat().resolvedOptions().timeZone || '-';

  function set(id, text) { document.getElementById(id).textContent = text; }
  function fmtUptime(s) {
    var d = Math.floor(s / 86400), h = Math.floor(s % 86400 / 3600), m = Math.floor(s % 3600 / 60);
    return d + ' 天 ' + h + ' 小时 ' + m + ' 分';
  }

  function chart(series) {
    var el = document.getElementById('chart');
    if (!series || !series.length) { el.innerHTML = '<p class="foot-note">暂无数据</p>'; return; }
    var W = 720, H = 260, padL = 40, padR = 16, padT = 14, padB = 30;
    var max = 10;
    for (var i = 0; i < series.length; i++) {
      max = Math.max(max, series[i].logins, series[i].tickets);
    }
    var step = series.length > 1 ? (W - padL - padR) / (series.length - 1) : 0;
    function x(i) { return padL + i * step; }
    function y(v) { return H - padB - v / max * (H - padT - padB); }
    function line(key) {
      var pts = [];
      for (var i = 0; i < series.length; i++) { pts.push(x(i) + ',' + y(series[i][key])); }
      return pts.join(' ');
    }
    var area = padL + ',' + (H - padB) + ' ' + line('logins') + ' ' + x(series.length - 1) + ',' + (H - padB);
    var svg = ['<svg viewBox="0 0 ' + W + ' ' + H + '" xmlns="http://www.w3.org/2000/svg">'];
    var rows = 4;
    for (var g = 0; g <= rows; g++) {
      var gy = padT + g * (H - padT - padB) / rows;
      var gv = Math.round(max * (1 - g / rows));
      svg.push('<line x1="' + padL + '" y1="' + gy + '" x2="' + (W - padR) + '" y2="' + gy + '" stroke="#e5e7eb" stroke-width="1"/>');
      svg.push('<text x="' + (padL - 8) + '" y="' + (gy + 4) + '" font-size="10" fill="#9ca3af" text-anchor="end">' + gv + '</text>');
    }
    svg.push('<polygon points="' + area + '" fill="rgba(99,102,241,.12)"/>');
    svg.push('<polyline points="' + line('logins') + '" fill="none" stroke="#6366f1" stroke-width="2.5"/>');
    svg.push('<polyline points="' + line('tickets') + '" fill="none" stroke="#10b981" stroke-width="2.5"/>');
    for (var j = 0; j < series.length; j++) {
      svg.push('<circle cx="' + x(j) + '" cy="' + y(series[j].logins) + '" r="3.5" fill="#fff" stroke="#6366f1" stroke-width="2"/>');
      svg.push('<circle cx="' + x(j) + '" cy="' + y(series[j].tickets) + '" r="3.5" fill="#fff" stroke="#10b981" stroke-width="2"/>');
      svg.push('<text x="' + x(j) + '" y="' + (H - 8) + '" font-size="10" fill="#9ca3af" text-anchor="middle">' + series[j].date.slice(5) + '</text>');
    }
    svg.push('</svg>');
    el.innerHTML = svg.join('');
  }

  function render(d) {
    set('hostname', d.server.hostname); set('hostname2', d.server.hostname);
    set('c-logins7d', d.cards.logins7d); set('c-tickets7d', d.cards.tickets7d);
    set('c-loginsToday', d.cards.loginsToday); set('c-verifyOkToday', d.cards.verifyOkToday);
    set('r-total', d.rejectsToday.total); set('r-state', d.rejectsToday.state_reject);
    set('r-sign', d.rejectsToday.verify_sign_reject); set('r-ticket', d.rejectsToday.ticket_reject);
    set('i-version', d.server.version); set('i-platform', d.server.platform);
    set('i-commit', d.server.commit); set('i-go', d.server.goVersion);
    set('i-build', d.server.buildDate); set('i-uptime', fmtUptime(d.server.uptimeSeconds));
    set('i-listen', d.server.listen); set('i-time', new Date(d.generatedAt).toLocaleTimeString('zh-CN', { hour12: false }));
    set('updated', new Date(d.generatedAt).toLocaleTimeString('zh-CN', { hour12: false }));
    set('mock-note', d.server.mock ? '当前为 mock 演练模式，数据仅用于本地链路验证' : '');
    var sp = document.getElementById('storage-pill'), sd = document.getElementById('storage-dot');
    if (d.storage.ok) { sp.textContent = '存储 ' + d.storage.driver + ' 在线'; sp.className = 'pill mini ok'; sd.className = 'dot-green'; }
    else { sp.textContent = '存储 ' + d.storage.driver + ' 离线'; sp.className = 'pill mini bad'; sd.className = 'dot-green off'; }
    var mp = document.getElementById('mode-pill');
    if (d.server.mock) { mp.textContent = '企微 mock 演练'; }
    else if (d.server.wecomMode === 'inside') { mp.textContent = '企微内 H5 授权'; }
    else { mp.textContent = 'PC 扫码登录'; }
    chart(d.series);
  }

  function load() {
    fetch('/api/status?token=' + encodeURIComponent(token)).then(function (r) {
      if (r.status === 403) { throw new Error('invalid_token'); }
      return r.json();
    }).then(render).catch(function (e) {
      if (e.message === 'invalid_token') {
        var p = document.getElementById('service-pill');
        p.textContent = '● 令牌无效'; p.className = 'pill bad';
      }
    });
  }
  load();
  setInterval(load, 30000);
})();
</script>
</body>
</html>`
