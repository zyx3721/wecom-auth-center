package handler

// statusHTML 监控页单文件前端：全视口 KMS 风格仪表盘，自绘 SVG 平滑曲线 + 悬停提示 + 统计卡弹窗明细，30 秒自动刷新
const statusHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="#4f46e5">
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='8' fill='%234f46e5'/%3E%3Ctext x='16' y='22' font-size='15' font-weight='bold' text-anchor='middle' fill='white'%3EW%3C/text%3E%3C/svg%3E">
<title>企微统一认证 · 运行状态与监控</title>
<style>
  :root{
    --card:rgba(255,255,255,.82);
    --border:rgba(255,255,255,.75);
    --stroke:rgba(31,58,110,.08);
    --text:#141b2e;
    --muted:#6b7691;
    --muted2:#98a2b8;
    --indigo:#4f46e5;
    --violet:#8b5cf6;
    --emerald:#10b981;
    --rose:#f43f5e;
    --amber:#f59e0b;
    --shadow:0 14px 40px rgba(31,58,110,.10);
    --shadow-sm:0 6px 18px rgba(31,58,110,.07);
  }
  *{box-sizing:border-box}
  html,body{height:100%;margin:0}
  body{
    font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",Roboto,Helvetica,Arial,sans-serif;
    color:var(--text);
    background:
      radial-gradient(1200px 620px at 8% -12%, rgba(99,102,241,.16), transparent 60%),
      radial-gradient(900px 520px at 98% -6%, rgba(139,92,246,.14), transparent 58%),
      radial-gradient(900px 600px at 50% 120%, rgba(16,185,129,.08), transparent 60%),
      linear-gradient(180deg,#f2f4fb 0%, #e9edf8 100%);
    background-attachment:fixed;
    -webkit-font-smoothing:antialiased;
    overflow:hidden;
  }
  .dashboard{height:100vh;height:100dvh;display:grid;grid-template-rows:auto auto minmax(0,1fr) auto;gap:14px;padding:18px 24px 14px}

  .topbar{display:flex;align-items:center;justify-content:space-between;gap:18px;flex-wrap:wrap}
  .brand{display:flex;align-items:center;gap:14px;min-width:0}
  .logo{width:46px;height:46px;border-radius:14px;flex:none;display:flex;align-items:center;justify-content:center;
    background:linear-gradient(135deg,var(--indigo),var(--violet));color:#fff;font-weight:800;font-size:21px;
    box-shadow:0 10px 24px rgba(79,70,229,.35)}
  .brand-text h1{margin:0;font-size:21px;font-weight:800;letter-spacing:-.4px;white-space:nowrap}
  .brand-text p{margin:2px 0 0;font-size:12px;color:var(--muted2);letter-spacing:.4px}
  .topbar-right{display:flex;align-items:center;gap:12px;flex-wrap:wrap;justify-content:flex-end}
  .badge{display:inline-flex;align-items:center;gap:9px;padding:9px 16px;border-radius:999px;
    background:rgba(16,185,129,.12);color:#0f8a5f;font-weight:700;font-size:13px;
    border:1px solid rgba(16,185,129,.22);white-space:nowrap}
  .badge.offline{background:rgba(244,63,94,.10);color:#d92b4b;border-color:rgba(244,63,94,.22)}
  .badge .pulse{width:8px;height:8px;border-radius:50%;background:currentColor;animation:pulse 2s infinite}
  @keyframes pulse{0%{box-shadow:0 0 0 0 rgba(16,185,129,.5)}70%{box-shadow:0 0 0 8px rgba(16,185,129,0)}100%{box-shadow:0 0 0 0 rgba(16,185,129,0)}}
  .badge.offline .pulse{animation:none}
  .chips{display:flex;gap:8px;flex-wrap:wrap}
  .chip{display:inline-flex;align-items:center;gap:6px;font-size:12px;color:#4a5876;
    background:rgba(255,255,255,.7);border:1px solid var(--stroke);border-radius:999px;
    padding:6px 12px;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;backdrop-filter:blur(8px)}
  .chip b{color:var(--indigo);font-weight:700}

  .stats{display:grid;grid-template-columns:repeat(4,1fr);gap:14px}
  .stat{position:relative;overflow:hidden;padding:16px 18px;border-radius:16px;
    background:var(--card);border:1px solid var(--border);box-shadow:var(--shadow-sm);
    backdrop-filter:blur(14px);display:flex;flex-direction:column;gap:6px;min-height:92px;justify-content:center}
  .stat::before{content:"";position:absolute;left:0;top:0;bottom:0;width:4px;border-radius:4px;
    background:linear-gradient(180deg,var(--indigo),var(--violet));opacity:.9}
  .stat.g::before{background:linear-gradient(180deg,#34d399,var(--emerald))}
  .stat.a::before{background:linear-gradient(180deg,#fbbf24,var(--amber))}
  .stat.r::before{background:linear-gradient(180deg,#fb7185,var(--rose))}
  .stat .label{font-size:12.5px;color:var(--muted);font-weight:600}
  .stat .value{font-size:27px;font-weight:800;letter-spacing:-.8px;line-height:1.05;
    background:linear-gradient(120deg,var(--indigo),var(--violet));-webkit-background-clip:text;background-clip:text;color:transparent}
  .stat.g .value{background:linear-gradient(120deg,#10b981,#34d399);-webkit-background-clip:text;background-clip:text}
  .stat.a .value{background:linear-gradient(120deg,#f59e0b,#fbbf24);-webkit-background-clip:text;background-clip:text}
  .stat.r .value{background:linear-gradient(120deg,#f43f5e,#fb7185);-webkit-background-clip:text;background-clip:text}
  .stat .foot{font-size:11.5px;color:var(--muted2)}
  .stat.clickable{cursor:pointer;transition:transform .16s ease,box-shadow .16s ease}
  .stat.clickable:hover{transform:translateY(-2px);box-shadow:0 16px 36px rgba(31,58,110,.14)}
  .stat-hint{position:absolute;right:14px;top:13px;font-size:10.5px;font-weight:700;color:var(--indigo);
    background:rgba(99,102,241,.10);padding:2px 8px;border-radius:999px;opacity:0;transition:opacity .16s}
  .stat.clickable:hover .stat-hint{opacity:1}

  .main{display:grid;grid-template-columns:minmax(0,1fr) 372px;gap:14px;min-height:0}
  .panel{background:var(--card);border:1px solid var(--border);border-radius:18px;
    box-shadow:var(--shadow);backdrop-filter:blur(16px);display:flex;flex-direction:column;min-height:0;overflow:hidden}
  .panel-head{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:14px 18px;border-bottom:1px solid var(--stroke);flex-wrap:wrap}
  .panel-head h2{margin:0;font-size:14.5px;font-weight:700}
  .panel-head .muted{color:var(--muted2);font-weight:500;font-size:12px;margin-left:4px}
  .legend{display:flex;align-items:center;gap:16px;font-size:12px;color:var(--muted)}
  .lg-line{display:inline-block;width:16px;height:3px;border-radius:3px;margin-right:6px;vertical-align:middle}
  .lg-req{background:var(--indigo)}
  .lg-ok{background:var(--emerald)}

  .chart-wrap{position:relative;flex:1;min-height:0;padding:10px 14px 12px}
  #chart{width:100%;height:100%;display:block}
  .empty{position:absolute;inset:0;display:none;align-items:center;justify-content:center;color:var(--muted2);font-size:13px}
  .chart-wrap.is-empty .empty{display:flex}
  .chart-tip{position:absolute;left:0;top:0;pointer-events:none;z-index:6;opacity:0;transition:opacity .12s ease;
    background:rgba(18,25,44,.94);color:#fff;border-radius:11px;padding:9px 12px;
    box-shadow:0 12px 30px rgba(10,18,40,.28);white-space:nowrap;backdrop-filter:blur(4px)}
  .chart-tip.show{opacity:1}
  .chart-tip .t-date{font-weight:700;font-size:12px;margin-bottom:6px;letter-spacing:.2px}
  .chart-tip .t-row{display:flex;align-items:center;gap:7px;font-size:12px;line-height:1.75}
  .chart-tip .t-dot{width:8px;height:8px;border-radius:50%;flex:none}
  .chart-tip .t-k{color:rgba(255,255,255,.72)}
  .chart-tip .t-v{margin-left:14px;font-weight:800;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}

  .side{display:flex;flex-direction:column;gap:14px;min-height:0;overflow:auto;padding-right:2px}
  .side::-webkit-scrollbar{width:6px}
  .side::-webkit-scrollbar-thumb{background:rgba(31,58,110,.15);border-radius:3px}
  .nodes{padding:14px 16px;display:flex;flex-direction:column;gap:12px}
  .node{border:1px solid var(--stroke);border-radius:14px;padding:15px;
    background:linear-gradient(160deg,rgba(255,255,255,.9),rgba(246,248,255,.9))}
  .node-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:11px}
  .node-name{font-weight:700;font-size:13.5px;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;
    overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .dot{width:9px;height:9px;border-radius:50%;background:#cbd5e5;flex:none}
  .dot.on{background:var(--emerald);box-shadow:0 0 0 4px rgba(16,185,129,.16)}
  .dot.off{background:var(--rose);box-shadow:0 0 0 4px rgba(244,63,94,.14)}
  .tags{display:flex;gap:8px;margin-bottom:13px;flex-wrap:wrap}
  .tag{font-size:11.5px;font-weight:700;padding:4px 10px;border-radius:999px;border:1px solid transparent}
  .tag.ok{background:rgba(16,185,129,.12);color:#0f8a5f;border-color:rgba(16,185,129,.2)}
  .tag.no{background:rgba(244,63,94,.10);color:#d92b4b;border-color:rgba(244,63,94,.2)}
  .tag.info{background:rgba(99,102,241,.10);color:#4f46e5;border-color:rgba(99,102,241,.18)}
  .metrics{display:grid;grid-template-columns:repeat(4,1fr);gap:9px}
  .metric{text-align:center;padding:11px 4px;border-radius:11px;background:rgba(99,102,241,.09);cursor:pointer;transition:transform .15s ease,box-shadow .15s ease}
  .metric:hover{transform:translateY(-2px);box-shadow:0 10px 22px rgba(31,58,110,.12)}
  .metric:nth-child(2){background:rgba(16,185,129,.10)}
  .metric:nth-child(3){background:rgba(139,92,246,.10)}
  .metric:nth-child(4){background:rgba(244,63,94,.10)}
  .metric .k{display:block;font-size:11px;color:var(--muted);margin-bottom:5px}
  .metric .v{font-size:16px;font-weight:800;color:#232f52}

  .side .panel:last-child{flex:1;min-height:0}
  .svc{flex:1;min-height:0;overflow:auto;padding:6px 16px 12px;display:flex;flex-direction:column;justify-content:space-evenly}
  .svc-item{display:flex;align-items:center;justify-content:space-between;gap:8px;
    padding:10px 2px;border-bottom:1px dashed var(--stroke);font-size:12.5px}
  .svc-item:last-child{border-bottom:none}
  .svc-item .k{color:var(--muted)}
  .svc-item .v{font-weight:700;color:#232f52;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;
    max-width:70%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}

  .foot{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;color:var(--muted2);font-size:12px;padding:0 4px}
  .foot b{color:var(--muted);font-weight:700}

  .modal{position:fixed;inset:0;z-index:50;display:none;align-items:center;justify-content:center;padding:24px}
  .modal.open{display:flex}
  .modal-backdrop{position:absolute;inset:0;background:rgba(15,22,40,.45);backdrop-filter:blur(3px)}
  .modal-card{position:relative;width:min(680px,100%);max-height:min(78vh,720px);display:flex;flex-direction:column;
    background:#fff;border-radius:18px;box-shadow:0 30px 80px rgba(10,18,40,.35);overflow:hidden;animation:modalIn .18s ease}
  @keyframes modalIn{from{opacity:0;transform:translateY(8px) scale(.985)}to{opacity:1;transform:none}}
  .modal-head{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;padding:18px 20px 14px;border-bottom:1px solid var(--stroke)}
  .modal-head h3{margin:0;font-size:16px;font-weight:800}
  .modal-head p{margin:4px 0 0;font-size:12px;color:var(--muted2)}
  .modal-close{border:none;background:rgba(31,58,110,.06);color:#4a5876;width:32px;height:32px;border-radius:10px;
    font-size:19px;line-height:1;cursor:pointer;flex:none;transition:.15s}
  .modal-close:hover{background:rgba(244,63,94,.12);color:var(--rose)}
  .modal-tabs{display:flex;gap:6px;padding:12px 20px 0}
  .tab{border:none;background:transparent;color:var(--muted);font-size:13px;font-weight:700;padding:8px 14px;border-radius:10px;cursor:pointer;transition:.15s}
  .tab:hover{background:rgba(99,102,241,.08)}
  .tab.active{background:rgba(99,102,241,.12);color:var(--indigo)}
  .modal-body{flex:1;min-height:0;overflow:auto;padding:10px 20px 20px}
  .detail-row{display:flex;align-items:center;gap:12px;padding:11px 12px;border-radius:12px}
  .detail-row:nth-child(odd){background:rgba(31,58,110,.025)}
  .detail-rank{width:26px;height:26px;flex:none;border-radius:8px;display:flex;align-items:center;justify-content:center;
    font-size:12px;font-weight:800;background:rgba(99,102,241,.10);color:var(--indigo)}
  .detail-main{min-width:0;flex:1}
  .detail-name{font-weight:700;font-size:13.5px;white-space:nowrap}
  .detail-sub{font-size:11.5px;color:var(--muted2);margin-top:2px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .detail-val{font-weight:800;font-size:14px;color:var(--indigo);flex:none;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}
  .detail-empty{text-align:center;color:var(--muted2);font-size:13px;padding:36px 0}

  @media (max-width:1360px){
    .main{grid-template-columns:minmax(0,1fr) 320px}
    .stats{grid-template-columns:repeat(2,1fr)}
  }
  @media (max-width:1080px){
    body{overflow:auto}
    .dashboard{height:auto;min-height:100vh}
    .main{grid-template-columns:1fr}
    .chart-wrap{min-height:320px}
  }
  @media (max-width:640px){
    .dashboard{padding:14px}
    .brand-text h1{font-size:18px}
    .stats{grid-template-columns:1fr 1fr}
    .stat .value{font-size:23px}
    .topbar-right{width:100%;justify-content:flex-start}
  }
</style>
</head>
<body>
<div class="dashboard">

  <header class="topbar">
    <div class="brand">
      <div class="logo">W</div>
      <div class="brand-text">
        <h1>企微统一认证 · 运行状态与监控</h1>
        <p>WECOM AUTH CENTER STATUS &amp; MONITORING</p>
      </div>
    </div>
    <div class="topbar-right">
      <div id="badge" class="badge"><span class="pulse"></span><span id="badgeText">检查中…</span></div>
      <div class="chips">
        <span class="chip">服务器 <b id="chipHost">-</b></span>
        <span class="chip">时区 <b id="chipTz">-</b></span>
      </div>
    </div>
  </header>

  <section class="stats">
    <div class="stat clickable" data-tab="days">
      <span class="stat-hint">详情 ›</span>
      <div class="label">近 7 天登录发起</div>
      <div class="value" id="statLogins7" data-val="0">0</div>
      <div class="foot">login 请求总数</div>
    </div>
    <div class="stat g clickable" data-tab="days">
      <span class="stat-hint">详情 ›</span>
      <div class="label">近 7 天登录成功</div>
      <div class="value" id="statTickets7" data-val="0">0</div>
      <div class="foot">ticket 签发总数</div>
    </div>
    <div class="stat a clickable" data-tab="today">
      <span class="stat-hint">详情 ›</span>
      <div class="label">今日登录发起</div>
      <div class="value" id="statLoginsToday" data-val="0">0</div>
      <div class="foot">今日 login 请求</div>
    </div>
    <div class="stat r clickable" data-tab="today">
      <span class="stat-hint">详情 ›</span>
      <div class="label">今日兑换成功</div>
      <div class="value" id="statVerifyOk" data-val="0">0</div>
      <div class="foot">verify 换取身份次数</div>
    </div>
  </section>

  <main class="main">
    <section class="panel">
      <div class="panel-head">
        <h2>近 7 天登录趋势<span class="muted" id="mockNote"></span></h2>
        <div class="legend">
          <span><span class="lg-line lg-req"></span>登录发起</span>
          <span><span class="lg-line lg-ok"></span>登录成功</span>
        </div>
      </div>
      <div class="chart-wrap" id="chartWrap">
        <svg id="chart" role="img" aria-label="近 7 天登录趋势"></svg>
        <div class="chart-tip" id="chartTip"></div>
        <div class="empty">暂无数据</div>
      </div>
    </section>

    <aside class="side">
      <section class="panel">
        <div class="panel-head"><h2>监控节点<span class="muted">今日实时</span></h2></div>
        <div class="nodes">
          <div class="node">
            <div class="node-head"><span class="node-name" id="nodeName">-</span><span class="dot" id="nodeDot"></span></div>
            <div class="tags"><span class="tag" id="tagStorage">存储 -</span><span class="tag" id="tagMode">企微 -</span></div>
            <div class="metrics">
              <div class="metric clickable" data-tab="today"><span class="k">今日登录</span><span class="v" id="mLogins">0</span></div>
              <div class="metric clickable" data-tab="today"><span class="k">今日签发</span><span class="v" id="mTickets">0</span></div>
              <div class="metric clickable" data-tab="today"><span class="k">今日兑换</span><span class="v" id="mVerify">0</span></div>
              <div class="metric clickable" data-tab="today"><span class="k">今日拒绝</span><span class="v" id="mRejects">0</span></div>
            </div>
          </div>
        </div>
      </section>
      <section class="panel">
        <div class="panel-head"><h2>服务信息</h2></div>
        <div class="svc" id="svc"></div>
      </section>
    </aside>
  </main>

  <footer class="foot">
    <span>近 7 天登录统计数据</span>
    <span>最后更新：<b id="updated">-</b> · 每 30 秒自动刷新</span>
  </footer>

</div>

<div class="modal" id="modal" aria-hidden="true">
  <div class="modal-backdrop" id="modalBackdrop"></div>
  <div class="modal-card" role="dialog" aria-modal="true">
    <div class="modal-head">
      <div>
        <h3 id="modalTitle">详细信息</h3>
        <p id="modalSub">-</p>
      </div>
      <button class="modal-close" id="modalClose" type="button" aria-label="关闭">×</button>
    </div>
    <div class="modal-tabs">
      <button class="tab" id="tabDays" type="button">按日明细</button>
      <button class="tab" id="tabToday" type="button">今日事件</button>
    </div>
    <div class="modal-body" id="modalBody"></div>
  </div>
</div>

<script>
(function () {
  "use strict";

  var REFRESH = 30;
  var NS = "http://www.w3.org/2000/svg";
  var qs = new URLSearchParams(location.search);
  var token = qs.get("token") || localStorage.getItem("wac_status_token") || "";
  if (token) { localStorage.setItem("wac_status_token", token); }

  var lastData = null;

  function $(id) { return document.getElementById(id); }
  function num(n) { return (n == null ? 0 : n).toLocaleString("zh-CN"); }
  function fmtDate(s) {
    var p = String(s || "").split("-");
    if (p.length !== 3) return String(s || "");
    return p[0] + "-" + parseInt(p[1], 10) + "-" + parseInt(p[2], 10);
  }
  function niceMax(v) {
    if (v <= 5) return 5;
    var exp = Math.pow(10, Math.floor(Math.log10(v)));
    var f = v / exp;
    var nf = f <= 1 ? 1 : f <= 2 ? 2 : f <= 2.5 ? 2.5 : f <= 5 ? 5 : 10;
    return nf * exp;
  }
  function svgEl(name, attrs) {
    var e = document.createElementNS(NS, name);
    for (var k in attrs) { if (Object.prototype.hasOwnProperty.call(attrs, k)) e.setAttribute(k, attrs[k]); }
    return e;
  }
  function setBadge(online, text) {
    var b = $("badge");
    b.className = "badge" + (online ? "" : " offline");
    $("badgeText").textContent = text;
  }

  var chartDays = [];

  function drawChart() {
    var svg = $("chart");
    var wrap = $("chartWrap");
    svg.innerHTML = "";

    var days = chartDays;
    var total = 0;
    (days || []).forEach(function (d) { total += (d.logins || 0) + (d.tickets || 0); });
    if (!days || !days.length || total === 0) {
      wrap.classList.add("is-empty");
      return;
    }
    wrap.classList.remove("is-empty");

    var W = wrap.clientWidth - 28;
    var H = wrap.clientHeight - 22;
    if (W < 60 || H < 60) return;
    svg.setAttribute("viewBox", "0 0 " + W + " " + H);

    var padL = 62, padR = 18, padT = 16, padB = 38;
    var plotW = W - padL - padR, plotH = H - padT - padB;
    var n = days.length;

    var maxReq = 1, maxOk = 1;
    days.forEach(function (d) {
      if ((d.logins || 0) > maxReq) maxReq = d.logins;
      if ((d.tickets || 0) > maxOk) maxOk = d.tickets;
    });
    var top = niceMax(Math.max(maxReq, maxOk));

    var step = plotW / n;
    function xAt(i) { return padL + (i + 0.5) * step; }
    function yAt(v) { return padT + plotH - (v / top) * plotH; }

    var defs = svgEl("defs");
    function areaGrad(id, rgb) {
      var gr = svgEl("linearGradient", { id: id, x1: "0", y1: "0", x2: "0", y2: "1" });
      gr.appendChild(svgEl("stop", { offset: "0%", "stop-color": "rgba(" + rgb + ",.24)" }));
      gr.appendChild(svgEl("stop", { offset: "100%", "stop-color": "rgba(" + rgb + ",0)" }));
      return gr;
    }
    defs.appendChild(areaGrad("gReq", "79,70,229"));
    defs.appendChild(areaGrad("gOk", "16,185,129"));
    svg.appendChild(defs);

    var lines = 5;
    for (var g = 0; g <= lines; g++) {
      var val = top * g / lines;
      var gy = yAt(val);
      svg.appendChild(svgEl("line", { x1: padL, y1: gy, x2: W - padR, y2: gy, stroke: "rgba(31,58,110,.07)", "stroke-width": 1 }));
      var yt = svgEl("text", { x: padL - 12, y: gy + 4, "text-anchor": "end", "font-size": "11", fill: "#98a2b8" });
      yt.textContent = Math.round(val);
      svg.appendChild(yt);
    }

    function smoothPath(pts) {
      if (!pts.length) return "";
      var d = "M" + pts[0][0] + " " + pts[0][1];
      for (var i = 0; i < pts.length - 1; i++) {
        var p0 = pts[i - 1] || pts[i];
        var p1 = pts[i];
        var p2 = pts[i + 1];
        var p3 = pts[i + 2] || p2;
        var c1x = p1[0] + (p2[0] - p0[0]) / 6, c1y = p1[1] + (p2[1] - p0[1]) / 6;
        var c2x = p2[0] - (p3[0] - p1[0]) / 6, c2y = p2[1] - (p3[1] - p1[1]) / 6;
        d += " C" + c1x + " " + c1y + " " + c2x + " " + c2y + " " + p2[0] + " " + p2[1];
      }
      return d;
    }

    function drawSeries(key, color, gradId) {
      var pts = days.map(function (d, i) { return [xAt(i), yAt(d[key] || 0)]; });
      var line = smoothPath(pts);
      var base = padT + plotH;
      var area = line + " L" + pts[pts.length - 1][0] + " " + base + " L" + pts[0][0] + " " + base + " Z";
      svg.appendChild(svgEl("path", { d: area, fill: "url(#" + gradId + ")" }));
      svg.appendChild(svgEl("path", { d: line, fill: "none", stroke: color, "stroke-width": "2.6", "stroke-linejoin": "round", "stroke-linecap": "round" }));
      var circles = [];
      pts.forEach(function (p) {
        var c = svgEl("circle", { cx: p[0], cy: p[1], r: 3.8, fill: "#ffffff", stroke: color, "stroke-width": "2.4" });
        svg.appendChild(c);
        circles.push(c);
      });
      return circles;
    }

    var reqCircles = drawSeries("logins", "#4f46e5", "gReq");
    var okCircles = drawSeries("tickets", "#10b981", "gOk");

    var guide = svgEl("line", { x1: 0, y1: padT, x2: 0, y2: padT + plotH, stroke: "rgba(79,70,229,.38)", "stroke-width": 1.4, "stroke-dasharray": "4 4", opacity: 0 });
    svg.appendChild(guide);

    days.forEach(function (d, i) {
      var xt = svgEl("text", { x: xAt(i), y: H - 12, "text-anchor": "middle", "font-size": "11.5", fill: "#8a95aa" });
      xt.textContent = fmtDate(d.date);
      svg.appendChild(xt);
    });

    function showTip(i, ev) {
      var d = days[i];
      tip.innerHTML =
        '<div class="t-date">' + fmtDate(d.date) + '</div>' +
        '<div class="t-row"><span class="t-dot" style="background:#4f46e5"></span><span class="t-k">登录发起</span><span class="t-v">' + num(d.logins || 0) + '</span></div>' +
        '<div class="t-row"><span class="t-dot" style="background:#10b981"></span><span class="t-k">登录成功</span><span class="t-v">' + num(d.tickets || 0) + '</span></div>';
      tip.classList.add("show");
      guide.setAttribute("x1", xAt(i));
      guide.setAttribute("x2", xAt(i));
      guide.setAttribute("opacity", 1);
      if (reqCircles[i]) reqCircles[i].setAttribute("r", 5.4);
      if (okCircles[i]) okCircles[i].setAttribute("r", 5.4);
      moveTip(ev);
    }
    function hideTip() {
      tip.classList.remove("show");
      guide.setAttribute("opacity", 0);
      for (var k = 0; k < reqCircles.length; k++) {
        reqCircles[k].setAttribute("r", 3.8);
        okCircles[k].setAttribute("r", 3.8);
      }
    }

    days.forEach(function (d, i) {
      var col = svgEl("rect", { x: xAt(i) - step / 2, y: padT, width: step, height: plotH, fill: "transparent", style: "cursor:crosshair" });
      col.addEventListener("mouseenter", function (ev) { showTip(i, ev); });
      col.addEventListener("mousemove", moveTip);
      col.addEventListener("mouseleave", hideTip);
      svg.appendChild(col);
    });
  }

  var tip = $("chartTip");
  function moveTip(ev) {
    var wrap = $("chartWrap");
    var r = wrap.getBoundingClientRect();
    var x = ev.clientX - r.left, y = ev.clientY - r.top;
    var tw = tip.offsetWidth, th = tip.offsetHeight;
    var left = x - tw / 2;
    if (left < 6) left = 6;
    if (left + tw > r.width - 6) left = r.width - 6 - tw;
    var top = y - th - 14;
    if (top < 6) top = y + 18;
    tip.style.left = left + "px";
    tip.style.top = top + "px";
  }

  if (window.ResizeObserver) {
    var ro = new ResizeObserver(function () { drawChart(); });
    ro.observe($("chartWrap"));
  } else {
    window.addEventListener("resize", drawChart);
  }

  function svcItem(k, v) {
    var d = document.createElement("div"); d.className = "svc-item";
    var ks = document.createElement("span"); ks.className = "k"; ks.textContent = k;
    var vs = document.createElement("span"); vs.className = "v"; vs.textContent = v;
    d.appendChild(ks); d.appendChild(vs); return d;
  }

  function renderService(d) {
    var box = $("svc");
    box.innerHTML = "";
    box.appendChild(svcItem("进程", "运行中"));
    box.appendChild(svcItem("端口", "监听中"));
    box.appendChild(svcItem("版本", d.server.version || "-"));
    box.appendChild(svcItem("平台", d.server.platform || "-"));
    box.appendChild(svcItem("主机", d.server.hostname || "-"));
    box.appendChild(svcItem("运行时长", fmtUptime(d.server.uptimeSeconds)));
    box.appendChild(svcItem("监听地址", d.server.listen || "-"));
    box.appendChild(svcItem("检测时间", new Date(d.generatedAt).toLocaleTimeString("zh-CN", { hour12: false })));
  }

  function fmtUptime(s) {
    s = Math.max(0, Math.floor(s || 0));
    var d = Math.floor(s / 86400), h = Math.floor(s % 86400 / 3600), m = Math.floor(s % 3600 / 60);
    return d + "d " + h + "h " + m + "m";
  }

  function animateNumber(el, to) {
    to = Math.round(to || 0);
    var from = parseInt(el.getAttribute("data-val") || "0", 10);
    if (isNaN(from)) from = 0;
    if (from === to) { el.textContent = num(to); return; }
    el.setAttribute("data-val", String(to));
    var start = null, dur = 900;
    function step(now) {
      if (start === null) start = now;
      var t = Math.min((now - start) / dur, 1);
      var e = 1 - Math.pow(1 - t, 3);
      el.textContent = num(Math.round(from + (to - from) * e));
      if (t < 1) requestAnimationFrame(step);
    }
    requestAnimationFrame(step);
  }

  function renderTags(d) {
    var st = $("tagStorage"), md = $("tagMode");
    if (d.storage.ok) { st.textContent = "存储 " + d.storage.driver + " 在线"; st.className = "tag ok"; }
    else { st.textContent = "存储 " + d.storage.driver + " 离线"; st.className = "tag no"; }
    if (d.server.mock) { md.textContent = "企微 mock 演练"; md.className = "tag info"; }
    else if (d.server.wecomMode === "inside") { md.textContent = "企微内 H5"; md.className = "tag ok"; }
    else { md.textContent = "PC 扫码"; md.className = "tag ok"; }
  }

  function apply(d) {
    lastData = d;
    chartDays = d.series || [];

    animateNumber($("statLogins7"), d.cards.logins7d);
    animateNumber($("statTickets7"), d.cards.tickets7d);
    animateNumber($("statLoginsToday"), d.cards.loginsToday);
    animateNumber($("statVerifyOk"), d.cards.verifyOkToday);
    $("mLogins").textContent = num(d.cards.loginsToday);
    $("mTickets").textContent = num(d.cards.ticketsToday);
    $("mVerify").textContent = num(d.cards.verifyOkToday);
    $("mRejects").textContent = num(d.rejectsToday.total);

    $("chipHost").textContent = d.server.hostname || "-";
    $("nodeName").textContent = d.server.hostname || "-";
    $("nodeDot").className = "dot " + (d.storage.ok ? "on" : "off");
    $("mockNote").textContent = d.server.mock ? "（mock 演练模式）" : "";

    renderTags(d);
    renderService(d);
    drawChart();
    $("updated").textContent = new Date(d.generatedAt).toLocaleTimeString("zh-CN", { hour12: false });
  }

  function refresh() {
    fetch("/api/status?token=" + encodeURIComponent(token), { cache: "no-store" }).then(function (r) {
      if (!r.ok) throw new Error("HTTP " + r.status);
      return r.json();
    }).then(function (d) {
      setBadge(true, "服务运行中");
      apply(d);
    }).catch(function () {
      setBadge(false, "状态获取失败");
    });
  }

  var modal = $("modal"), modalBody = $("modalBody"), modalTitle = $("modalTitle"), modalSub = $("modalSub");
  var currentTab = "days";

  function detailRow(rank, name, sub, val) {
    var row = document.createElement("div"); row.className = "detail-row";
    var rk = document.createElement("div"); rk.className = "detail-rank"; rk.textContent = rank;
    var main = document.createElement("div"); main.className = "detail-main";
    var nm = document.createElement("div"); nm.className = "detail-name"; nm.textContent = name;
    main.appendChild(nm);
    if (sub) {
      var sb = document.createElement("div"); sb.className = "detail-sub"; sb.textContent = sub;
      main.appendChild(sb);
    }
    var vl = document.createElement("div"); vl.className = "detail-val"; vl.textContent = val;
    row.appendChild(rk); row.appendChild(main); row.appendChild(vl);
    return row;
  }

  function renderModal() {
    if (!lastData) return;
    modalBody.innerHTML = "";
    if (currentTab === "days") {
      var s = lastData.series || [];
      if (!s.length) { modalBody.innerHTML = '<div class="detail-empty">暂无数据</div>'; return; }
      modalSub.textContent = "统计区间 " + fmtDate(s[0].date) + " ~ " + fmtDate(s[s.length - 1].date);
      s.slice().reverse().forEach(function (day, i) {
        modalBody.appendChild(detailRow(String(i + 1), fmtDate(day.date),
          "登录成功 " + num(day.tickets || 0) + " · 拒绝 " + num(day.rejects || 0), num(day.logins || 0) + " 次"));
      });
      return;
    }
    modalSub.textContent = "统计日期 " + fmtDate((lastData.generatedAt || "").slice(0, 10));
    var today = lastData.rejectsToday || {};
    var groups = [
      ["登录发起", lastData.cards.loginsToday, "成功侧"],
      ["ticket 签发", lastData.cards.ticketsToday, "成功侧"],
      ["兑换成功", lastData.cards.verifyOkToday, "成功侧"],
      ["state 校验失败", today.state_reject, "拒绝侧"],
      ["白名单外 app", today.verify_app_reject, "拒绝侧"],
      ["签名失败", today.verify_sign_reject, "拒绝侧"],
      ["时间戳超差", today.verify_ts_reject, "拒绝侧"],
      ["ticket 拒绝（重放/过期）", today.ticket_reject, "拒绝侧"],
      ["ticket 归属不匹配", today.ticket_mismatch, "拒绝侧"]
    ];
    var total = 0;
    groups.forEach(function (g) { total += (g[1] || 0); });
    if (total === 0) { modalBody.innerHTML = '<div class="detail-empty">今日暂无事件</div>'; return; }
    groups.forEach(function (g, i) {
      modalBody.appendChild(detailRow(String(i + 1), g[0], g[2] + "事件", num(g[1] || 0) + " 次"));
    });
  }

  function setTab(tab) {
    currentTab = tab;
    $("tabDays").classList.toggle("active", tab === "days");
    $("tabToday").classList.toggle("active", tab === "today");
    renderModal();
  }

  function openModal(tab) {
    modalTitle.textContent = "详细事件明细";
    modal.classList.add("open");
    modal.setAttribute("aria-hidden", "false");
    setTab(tab);
  }
  function closeModal() {
    modal.classList.remove("open");
    modal.setAttribute("aria-hidden", "true");
  }

  $("modalClose").addEventListener("click", closeModal);
  $("modalBackdrop").addEventListener("click", closeModal);
  $("tabDays").addEventListener("click", function () { setTab("days"); });
  $("tabToday").addEventListener("click", function () { setTab("today"); });
  document.addEventListener("keydown", function (e) { if (e.key === "Escape") closeModal(); });
  Array.prototype.forEach.call(document.querySelectorAll("[data-tab]"), function (el) {
    el.addEventListener("click", function () { openModal(el.getAttribute("data-tab")); });
  });

  if (!token) {
    setBadge(false, "缺少访问令牌");
    $("badge").title = "请以 /status?token=你的令牌 访问";
  } else {
    $("chipTz").textContent = Intl.DateTimeFormat().resolvedOptions().timeZone || "-";
    refresh();
    setInterval(refresh, REFRESH * 1000);
  }
})();
</script>

</body></html>`
