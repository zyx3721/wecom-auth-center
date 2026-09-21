package handler

// statusHTML 监控页单文件前端：全视口 KMS 风格仪表盘，自绘 SVG 平滑曲线 + 悬停提示 + 统计卡弹窗明细 +
// 最近登录行点击弹窗查看成员信息，30 秒自动刷新。样式与脚本分别在 status_page_style.go 与 status_page_script.go。
const statusHTML = statusPageHead + statusPageCSS + statusPageMid + statusPageJS + statusPageTail

// statusPageHead 页面 DOCTYPE、元信息与样式标签开头
const statusPageHead = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="theme-color" content="#4f46e5">
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='8' fill='%234f46e5'/%3E%3Ctext x='16' y='22' font-size='15' font-weight='bold' text-anchor='middle' fill='white'%3EW%3C/text%3E%3C/svg%3E">
<title>企微统一认证 · 运行状态与监控</title>
<style>
`

// statusPageMid 样式收尾与页面主体结构：统计卡、趋势图、最近登录、监控节点、服务信息与两个弹窗骨架
const statusPageMid = `
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
    <div class="left-col">
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

    <section class="panel logins-panel">
      <div class="panel-head"><h2>最近登录<span class="muted">最近 50 条 · 点击行查看成员信息</span></h2><span class="muted" id="loginsCount"></span></div>
      <div class="logins" id="logins"><div class="detail-empty">暂无登录记录</div></div>
    </section>
    </div>

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

<div class="modal" id="loginModal" aria-hidden="true">
  <div class="modal-backdrop" id="loginModalBackdrop"></div>
  <div class="modal-card narrow" role="dialog" aria-modal="true">
    <div class="modal-head">
      <div>
        <h3 id="loginModalTitle">成员信息</h3>
        <p id="loginModalSub">-</p>
      </div>
      <button class="modal-close" id="loginModalClose" type="button" aria-label="关闭">×</button>
    </div>
    <div class="modal-body" id="loginModalBody"></div>
  </div>
</div>

<script>
`

// statusPageTail 脚本与页面收尾
const statusPageTail = `
</script>

</body></html>`
