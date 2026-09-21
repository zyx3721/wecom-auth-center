package handler

// statusPageCSS 监控页全部样式：KMS 风格仪表盘、曲线图、统计卡弹窗与最近登录档案弹窗
const statusPageCSS = `  :root{
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
  .left-col{display:flex;flex-direction:column;gap:14px;min-height:0;min-width:0}
  .left-col > .panel{flex:1 1 0;min-height:220px}
  .logins-panel .logins{flex:1;min-height:0;overflow:auto;padding:2px 16px 10px;display:flex;flex-direction:column}
  .logins .detail-empty{margin:auto;padding:14px 0}
  .login-row{display:grid;grid-template-columns:repeat(5,1fr);gap:10px;align-items:center;justify-items:center;
    padding:9px 40px;border-radius:10px;font-size:12.5px}
  .login-row:nth-child(odd){background:rgba(31,58,110,.025)}
  .login-row .lt{color:var(--muted2);font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}
  .login-row .lu{font-weight:700;color:#232f52;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;
    overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .login-row .ln{color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .login-row .la{color:var(--indigo);font-weight:700}
  .login-row .lr{color:var(--muted);font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;
    overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .login-row.clickable{cursor:pointer;transition:background .14s ease}
  .login-row.clickable:hover{background:rgba(99,102,241,.08)}
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
  .modal-card.narrow{width:min(460px,100%)}
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
  .lm-section{font-size:12px;font-weight:800;color:var(--indigo);margin:14px 2px 4px;letter-spacing:.4px}
  .lm-section:first-child{margin-top:6px}

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
`
