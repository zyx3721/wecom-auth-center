package handler

// statusPageJS 监控页前端脚本：数据拉取渲染、SVG 曲线、统计卡弹窗与最近登录成员信息弹窗
const statusPageJS = `(function () {
  "use strict";

  var REFRESH = 30;
  var NS = "http://www.w3.org/2000/svg";
  var qs = new URLSearchParams(location.search);
  var token = qs.get("token") || localStorage.getItem("wac_status_token") || "";
  if (token) { localStorage.setItem("wac_status_token", token); }

  var lastData = null;

  function $(id) { return document.getElementById(id); }
  function num(n) { return (n == null ? 0 : n).toLocaleString("zh-CN"); }
  function dash(v) { return (v === null || v === undefined || v === "") ? "-" : v; }
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
        var yLo = Math.min(p1[1], p2[1]), yHi = Math.max(p1[1], p2[1]);
        c1y = Math.min(yHi, Math.max(yLo, c1y));
        c2y = Math.min(yHi, Math.max(yLo, c2y));
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

  function renderLogins(d) {
    var box = $("logins");
    var list = d.recentLogins || [];
    $("loginsCount").textContent = list.length ? ("共 " + list.length + " 条") : "";
    if (!list.length) { box.innerHTML = '<div class="detail-empty">暂无登录记录</div>'; return; }
    box.innerHTML = "";
    list.forEach(function (r) {
      var row = document.createElement("div"); row.className = "login-row clickable";
      row.title = "点击查看成员信息";
      function cell(cls, text) {
        var e = document.createElement("span"); e.className = cls; e.textContent = text || "-"; return e;
      }
      row.appendChild(cell("lt", r.time));
      row.appendChild(cell("lu", r.userid));
      row.appendChild(cell("ln", r.name));
      row.appendChild(cell("la", r.app));
      row.appendChild(cell("lr", r.remote));
      row.addEventListener("click", function () { openLoginModal(r); });
      box.appendChild(row);
    });
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
    renderLogins(d);
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
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") { closeModal(); closeLoginModal(); }
  });
  Array.prototype.forEach.call(document.querySelectorAll("[data-tab]"), function (el) {
    el.addEventListener("click", function () { openModal(el.getAttribute("data-tab")); });
  });

  var loginModal = $("loginModal"), loginModalBody = $("loginModalBody");
  var loginModalTitle = $("loginModalTitle"), loginModalSub = $("loginModalSub");

  function lmSection(text) {
    var s = document.createElement("div"); s.className = "lm-section"; s.textContent = text; return s;
  }

  function lmRow(k, v) {
    return svcItem(k, dash(v));
  }

  function fmtDepts(r) {
    var parts = (r.departments || []).map(function (d) { return d.name || ""; })
      .filter(function (s) { return s; });
    return parts.join("、");
  }

  function openLoginModal(r) {
    loginModalTitle.textContent = dash(r.userid) === "-" ? "成员信息" : r.userid;
    loginModalSub.textContent = "姓名 " + dash(r.name) + " · 来源应用 " + dash(r.app);
    loginModalBody.innerHTML = "";
    loginModalBody.appendChild(lmSection("登录信息"));
    loginModalBody.appendChild(lmRow("登录时间", r.time));
    loginModalBody.appendChild(lmRow("来源应用", r.app));
    loginModalBody.appendChild(lmRow("来源 IP", r.remote));
    loginModalBody.appendChild(lmSection("成员信息"));
    loginModalBody.appendChild(lmRow("企微账号", r.userid));
    loginModalBody.appendChild(lmRow("姓名", r.name));
    loginModalBody.appendChild(lmRow("员工编码", r.jobNumber));
    loginModalBody.appendChild(lmRow("部门", fmtDepts(r)));
    loginModal.classList.add("open");
    loginModal.setAttribute("aria-hidden", "false");
  }

  function closeLoginModal() {
    loginModal.classList.remove("open");
    loginModal.setAttribute("aria-hidden", "true");
  }

  $("loginModalClose").addEventListener("click", closeLoginModal);
  $("loginModalBackdrop").addEventListener("click", closeLoginModal);

  if (!token) {
    setBadge(false, "缺少访问令牌");
    $("badge").title = "请以 /status?token=你的令牌 访问";
  } else {
    $("chipTz").textContent = Intl.DateTimeFormat().resolvedOptions().timeZone || "-";
    refresh();
    setInterval(refresh, REFRESH * 1000);
  }
})();
`
