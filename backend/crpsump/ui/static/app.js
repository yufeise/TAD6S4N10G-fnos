// CRPS 电源监控 前端逻辑 (融合版: API 走 ../api/crps/)
(function () {
  "use strict";

  var $ = function (id) { return document.getElementById(id); };

  var METRIC_DEFS = [
    { key: "vin", label: "输入电压", unit: "V", fmt: "%.1f", w: "%.1f" },
    { key: "iin", label: "输入电流", unit: "A", fmt: "%.2f", w: "%.2f" },
    { key: "vout", label: "输出电压", unit: "V", fmt: "%.2f", w: "%.1f" },
    { key: "iout", label: "输出电流", unit: "A", fmt: "%.1f", w: "%.1f" },
    { key: "pin", label: "输入功率", unit: "W", fmt: "%.1f", w: "%.1f" },
    { key: "pout", label: "输出功率", unit: "W", fmt: "%.1f", w: "%.1f" },
    { key: "efficiency", label: "效率", unit: "%", fmt: "%.1f", w: "%.1f" },
    { key: "pf", label: "功率因数", unit: "", fmt: "%.3f", w: "%.3f" },
    { key: "temp1", label: "温度 1", unit: "°C", fmt: "%.1f", w: "%.1f" },
    { key: "temp2", label: "温度 2", unit: "°C", fmt: "%.1f", w: "%.1f" },
  ];

  var fmt = function (v, spec) {
    if (v === null || v === undefined) return "--";
    var num = Number(v);
    if (isNaN(num)) return "--";
    var s = spec || "%.1f";
    return s.replace("%.", ".").indexOf(".") >= 0
      ? num.toFixed(parseInt(s.replace(/%.(\d+)f/, "$1"), 10))
      : num.toFixed(0);
  };

  var grid = $("metrics");

  function buildGrid() {
    grid.innerHTML = "";
    METRIC_DEFS.forEach(function (m) {
      var c = document.createElement("div");
      c.className = "card";
      c.id = "m-" + m.key;
      c.innerHTML =
        '<div class="label">' + m.label + "</div>" +
        '<div class="value"><span class="num">--</span><span class="unit">' + m.unit + "</span></div>" +
        (m.key === "pf" ? '<div class="pf-bar"><div class="pf-fill" id="pf-fill"></div></div>' : "") +
        '<div class="extra"></div>';
      grid.appendChild(c);
    });
  }

  function setMetric(key, val, spec, extra) {
    var c = $("m-" + key);
    if (!c) return;
    c.querySelector(".num").textContent = fmt(val, spec);
    if (extra !== undefined && extra !== null) {
      c.querySelector(".extra").textContent = extra;
    }
  }

  var lastOk = null;

  function render(data) {
    if (!data) {
      setBanner(false, "无法连接后端服务");
      return;
    }
    METRIC_DEFS.forEach(function (m) {
      var v = data[m.key];
      var extra = "";
      if (m.key === "vout" && data.vout_calc && data.vout === null) {
        extra = "由 POUT/IOUT 推算";
      }
      if (m.key === "iin" && data.iin === null && data.iin_calc != null) {
        v = data.iin_calc;
        extra = "由 PIN/VIN 推算";
      }
      if (m.key === "temp1") extra = "内部温度";
      if (m.key === "temp2") extra = "内部温度";
      setMetric(m.key, v, m.fmt, extra);
    });
    // 功率因数动态条: 宽度/颜色随负载实时变化
    var pfFill = $("pf-fill");
    if (pfFill && data.pf != null) {
      pfFill.style.width = Math.min(100, Math.max(0, data.pf * 100)) + "%";
      pfFill.style.background = data.pf >= 0.95 ? "#22c55e" : data.pf >= 0.85 ? "#f59e0b" : "#ef4444";
    }
    // 风扇转速只显示在风扇控制区(避免与上方参数网格重复)
    $("fan-rpm").textContent = fmt(data.fan, "%.0f");
    if (data.fan_mode) setModeUI(data.fan_mode);

    var ok = data.ok === true;
    var bannerText;
    if (data.error === "gpio-not-configured") {
      bannerText = "尚未设置 GPIO 引脚，请在上方配置 PSCL / PSDA";
    } else if (data.error === "no-gpio") {
      bannerText = "未检测到电源(GPIO) 连接";
    } else if (ok) {
      bannerText = "电源状态正常";
    } else {
      bannerText = "电源异常 / 状态字 0x" + (data.status_raw || "????");
    }
    setBanner(ok, bannerText);
    $("status-banner").className = "status-banner " + (ok ? "ok" : "err");

    // 更新头部连接状态文案
    var sub;
    if (data.error === "gpio-not-configured") {
      sub = "GPIO 未配置，等待设置 PSCL/PSDA";
    } else if (data.error === "no-gpio") {
      sub = "PMBus 未检测到电源 (GPIO)";
    } else if (ok) {
      sub = "PMBus 已连接";
    } else {
      sub = "PMBus 连接异常 (0x" + (data.status_raw || "????") + ")";
    }
    $("header-sub").textContent = sub;

    if (data.mfr_id || data.mfr_model) {
      $("mfr-info").innerHTML =
        '<span class="k">厂商</span> ' + esc(data.mfr_id || "--") +
        '<br><span class="k">型号</span> ' + esc(data.mfr_model || "--");
    }
    $("ts").textContent = "更新于 " + (new Date(data.ts * 1000).toLocaleTimeString());
    $("bus-info").textContent = "总线 " + (data.bus === "gpio" ? "GPIO bitbang" : "i2c-" + data.bus) + " · 地址 " + data.addr;
    $("app-ver").textContent = "电源监控 v" + (data.app_version || "");
    lastOk = data;
  }

  function esc(s) {
    return String(s).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function setBanner(ok, text) {
    var b = $("status-banner");
    b.className = "status-banner " + (ok ? "ok" : ok === false ? "err" : "");
    $("status-text").textContent = text;
  }

  function poll() {
    fetch("../api/crps/status")
      .then(function (r) { return r.json(); })
      .then(render)
      .catch(function (e) {
        setBanner(false, "读取失败: " + e.message);
        $("header-sub").textContent = "PMBus 连接异常";
      });
  }

  // ---- 风扇控制 ----
  var slider = $("fan-slider");
  var pctLabel = $("fan-pct");
  var msg = $("fan-msg");
  var modeAuto = $("mode-auto");
  var modeManual = $("mode-manual");
  var manualCtl = $("manual-ctl");
  var lastSent = null;

  function setModeUI(mode) {
    modeAuto.classList.toggle("active", mode === "auto");
    modeManual.classList.toggle("active", mode === "manual");
    manualCtl.style.display = mode === "manual" ? "flex" : "none";
  }

  function postFan(body, doneMsg) {
    msg.className = "";
    msg.textContent = "正在设置…";
    fetch("../api/crps/fan", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        if (d.ok) {
          msg.className = "ok";
          setModeUI(d.mode);
          msg.textContent = doneMsg(d);
        } else {
          msg.className = "err";
          msg.textContent = "设置失败: " + (d.error || "未知错误");
        }
      })
      .catch(function (e) {
        msg.className = "err";
        msg.textContent = "设置失败: " + e.message;
      });
  }

  modeAuto.addEventListener("click", function () {
    // 实测光宝固件: FAN_COMMAND_1=10% 会触发恢复自动温控(约2700转低速档), 可软切回, 无需断电。
    // 注意: 自动转速下限随温度变化(低温约2600-2700, 高温固件会钳制在更高值如≥3100),
    // 提示不写死具体转速, 避免误导。其他值(0/0xFFFF/0x00FF)实测无效。
    postFan({ mode: "auto" }, function () {
      return "已软切回电源自动温控（写 FAN_COMMAND_1=10%），转速交由固件按温度自动调节";
    });
  });
  modeManual.addEventListener("click", function () {
    postFan({ mode: "manual", pct: parseInt(slider.value, 10) }, function (d) {
      return "已切换：手动 " + d.pct + "%";
    });
  });

  slider.addEventListener("input", function () {
    pctLabel.textContent = slider.value + "%";
  });

  slider.addEventListener("change", function () {
    var pct = parseInt(slider.value, 10);
    if (lastSent === pct) return;
    lastSent = pct;
    postFan({ mode: "manual", pct: pct }, function (d) {
      return "已设置风扇占空比 " + d.pct + "%";
    });
  });

  // ---- GPIO 引脚配置 ----
  var sclSel = $("scl-pin");
  var sdaSel = $("sda-pin");
  var gpioSave = $("gpio-save");
  var gpioMsg = $("gpio-msg");
  var gpioPins = [];

  function gpioMsgShow(ok, text) {
    if (!gpioMsg) return;
    gpioMsg.className = "gpio-msg " + (ok ? "ok" : "err");
    gpioMsg.textContent = text;
  }

  function loadGpioConfig() {
    if (!sclSel || !sdaSel) return;
    fetch("../api/crps/config")
      .then(function (r) { return r.json(); })
      .then(function (d) {
        if (!d || !Array.isArray(d.pins)) return;
        gpioPins = d.pins;
        var curScl = d.scl_pin, curSda = d.sda_pin;
        [sclSel, sdaSel].forEach(function (sel) {
          sel.innerHTML = "";
          gpioPins.forEach(function (p) {
            var o = document.createElement("option");
            o.value = p;
            // 动态标注: 当前设置为 PSCL/PSDA 的针脚显示对应角色
            var tag = "";
            if (p === curScl) tag = " (PSCL)";
            if (p === curSda) tag = " (PSDA)";
            o.textContent = p + tag;
            sel.appendChild(o);
          });
        });
        if (d.scl_pin && gpioPins.indexOf(d.scl_pin) >= 0) sclSel.value = d.scl_pin;
        if (d.sda_pin && gpioPins.indexOf(d.sda_pin) >= 0) sdaSel.value = d.sda_pin;
        if (d.configured === false) {
          gpioMsgShow(false, "尚未设置 GPIO 引脚：选择 PSCL/PSDA 后点“保存并应用”开始读取");
        } else {
          gpioMsgShow(d.readable === true, d.readable === true ? "当前接线可读（已读到电源数据）" : "当前接线未检测到电源数据");
        }
      })
      .catch(function () { gpioMsgShow(false, "无法读取 GPIO 配置"); });
  }

  if (gpioSave) gpioSave.addEventListener("click", function () {
    var scl = sclSel.value, sda = sdaSel.value;
    if (!scl || !sda) { gpioMsgShow(false, "请选择针脚"); return; }
    if (scl === sda) { gpioMsgShow(false, "PSCL 与 PSDA 不能是同一根针"); return; }
    gpioMsgShow(true, "正在应用…");
    fetch("../api/crps/config", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ scl_pin: scl, sda_pin: sda })
    })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        if (d && d.ok) {
          gpioMsgShow(true, "已应用 PSCL=" + d.scl_pin + " PSDA=" + d.sda_pin + "（" + (d.msg === "ok" ? "已持久化" : d.msg) + "），连接状态见上方");
          loadGpioConfig();   // 刷新下拉动态标注
          setTimeout(poll, 300);   // 立即刷新一次状态
        } else {
          gpioMsgShow(false, "应用失败: " + ((d && d.error) || (d && d.msg) || "未知错误"));
        }
      })
      .catch(function (e) { gpioMsgShow(false, "应用失败: " + e.message); });
  });

  // ---- 自动探测 PSCL/PSDA ----
  var gpioDetect = $("gpio-detect");
  if (gpioDetect) gpioDetect.addEventListener("click", function () {
    gpioMsgShow(true, "正在自动探测 Pin19~22…（约几秒，请稍候）");
    gpioDetect.disabled = true;
    fetch("../api/crps/detect", { method: "POST" })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        gpioDetect.disabled = false;
        if (d && d.found) {
          gpioMsgShow(true, "已自动探测到接线：PSCL=" + d.scl_pin + " PSDA=" + d.sda_pin + "，开始读取");
          if (sclSel && d.scl_pin) sclSel.value = d.scl_pin;
          if (sdaSel && d.sda_pin) sdaSel.value = d.sda_pin;
          loadGpioConfig();   // 刷新下拉动态标注
          setTimeout(poll, 300);
        } else {
          gpioMsgShow(false, (d && d.msg) || "未探测到电源接线，请检查线缆后重试");
        }
      })
      .catch(function (e) {
        gpioDetect.disabled = false;
        gpioMsgShow(false, "探测失败: " + e.message);
      });
  });

  // ---- 查看日志(弹窗显示 + 复制) ----
  var dlLog = $("dl-log");
  if (dlLog) dlLog.addEventListener("click", function () {
    fetch("../api/crps/log")
      .then(function (r) { return r.text(); })
      .then(function (t) {
        var body = $("log-body");
        var mask = $("log-mask");
        if (body) body.textContent = t || "(日志为空)";
        if (mask) mask.style.display = "flex";
      })
      .catch(function (e) { gpioMsgShow(false, "读取日志失败: " + e.message); });
  });
  var logCopy = $("log-copy");
  if (logCopy) logCopy.addEventListener("click", function () {
    var body = $("log-body");
    if (!body) return;
    var txt = body.textContent;
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(txt).then(function () {
        gpioMsgShow(true, "日志已复制到剪贴板");
      }, function () { fallbackCopy(txt); });
    } else { fallbackCopy(txt); }
  });
  var logClose = $("log-close");
  if (logClose) logClose.addEventListener("click", function () {
    var mask = $("log-mask");
    if (mask) mask.style.display = "none";
  });
  function fallbackCopy(txt) {
    try {
      var ta = document.createElement("textarea");
      ta.value = txt; ta.style.position = "fixed"; ta.style.opacity = "0";
      document.body.appendChild(ta); ta.select();
      document.execCommand("copy"); document.body.removeChild(ta);
      gpioMsgShow(true, "日志已复制到剪贴板");
    } catch (e) { gpioMsgShow(false, "复制失败, 请长按日志文本手动复制"); }
  }

  // ---- 耗电统计 ----
  var enCv = $("energy-chart-canvas");
  var enCtx = enCv ? enCv.getContext("2d") : null;
  var enTip = $("energy-tooltip");
  var enMsg = $("energy-msg");
  var enBars = []; // 柱状图命中区域 {x,y,w,h,date,wh}

  function enVal(wh) {
    var kwh = wh / 1000;
    return kwh < 10 ? kwh.toFixed(3) : kwh.toFixed(2);
  }

  function renderEnergy(d) {
    if (!d || !d.ok) return;
    $("en-total").textContent = enVal(d.total_wh);
    $("en-today").textContent = enVal(d.today_wh);
    $("en-month").textContent = enVal(d.month_wh);
    $("en-power").textContent = d.pin_w != null ? Number(d.pin_w).toFixed(1) : "--";
    var since = "";
    if (d.first_ts) {
      var t = new Date(d.first_ts * 1000);
      var p2 = function (n) { return String(n).padStart(2, "0"); };
      since = "自 " + t.getFullYear() + "-" + p2(t.getMonth() + 1) + "-" + p2(t.getDate()) + " 累计";
    }
    $("en-since").textContent = since;
    $("en-session").textContent = "本次运行 " + enVal(d.session_wh || 0) + " kWh";
    if (enCtx) drawEnergyChart(d.days || []);
  }

  function pollEnergy() {
    fetch("../api/crps/energy")
      .then(function (r) { return r.json(); })
      .then(renderEnergy)
      .catch(function () { /* 静默, 下次再取 */ });
  }

  function drawEnergyChart(days) {
    var dpr = window.devicePixelRatio || 1;
    var w = enCv.clientWidth || 300, h = 140;
    if (enCv.width !== w * dpr || enCv.height !== h * dpr) {
      enCv.width = w * dpr;
      enCv.height = h * dpr;
    }
    enCtx.setTransform(dpr, 0, 0, dpr, 0, 0);
    enCtx.clearRect(0, 0, w, h);
    enBars = [];
    var data = days.slice(-14);
    if (!data.length) {
      enCtx.fillStyle = "#999"; enCtx.font = "13px sans-serif"; enCtx.textAlign = "center";
      enCtx.fillText("暂无数据（每 15 秒自动累计）", w / 2, h / 2);
      return;
    }
    var padL = 40, padR = 8, padT = 16, padB = 22;
    var iw = w - padL - padR, ih = h - padT - padB;
    var maxWh = 10;
    data.forEach(function (d) { if (d.wh > maxWh) maxWh = d.wh; });
    maxWh = maxWh * 1.15;
    var bw = Math.min(44, iw / data.length * 0.6);
    // 网格 + Y 轴 kWh 刻度
    enCtx.strokeStyle = "#eee"; enCtx.lineWidth = 1;
    enCtx.fillStyle = "#94a3b8"; enCtx.font = "10px sans-serif"; enCtx.textAlign = "right";
    for (var g = 0; g <= 4; g++) {
      var gy = padT + ih * g / 4;
      enCtx.beginPath(); enCtx.moveTo(padL, gy); enCtx.lineTo(w - padR, gy); enCtx.stroke();
      var kv = maxWh * (1 - g / 4) / 1000;
      enCtx.fillText(kv < 1 ? kv.toFixed(2) : kv.toFixed(1), padL - 4, gy + 3);
    }
    enCtx.textAlign = "left";
    enCtx.fillText("kWh", 4, padT - 5); // 单位放在顶部刻度线上方, 避开 0.30 重叠
    // 柱体 (今日高亮)
    var step = Math.ceil(data.length / 7);
    data.forEach(function (d, i) {
      var cx = padL + iw * (i + 0.5) / data.length;
      var bh = d.wh > 0 ? Math.max(2, d.wh / maxWh * ih) : 0;
      enCtx.fillStyle = i === data.length - 1 ? "#2563eb" : "#93c5fd";
      if (bh > 0) enCtx.fillRect(cx - bw / 2, padT + ih - bh, bw, bh);
      enBars.push({ x: cx - bw / 2 - 3, y: padT, w: bw + 6, h: ih, date: d.date, wh: d.wh });
      if (i % step === 0) {
        enCtx.fillStyle = "#94a3b8"; enCtx.textAlign = "center";
        enCtx.fillText(d.date.slice(5), cx, padT + ih + 13);
      }
    });
  }

  if (enCv) {
    enCv.addEventListener("mousemove", function (ev) {
      if (!enTip) return;
      var rect = enCv.getBoundingClientRect();
      var mx = ev.clientX - rect.left, my = ev.clientY - rect.top;
      var hit = null;
      for (var i = 0; i < enBars.length; i++) {
        var b = enBars[i];
        if (mx >= b.x && mx <= b.x + b.w && my >= b.y && my <= b.y + b.h) { hit = b; break; }
      }
      if (!hit) { enTip.style.display = "none"; return; }
      enTip.textContent = hit.date + " · " + (hit.wh / 1000).toFixed(3) + " kWh";
      var left = mx + 10;
      if (left + 150 > rect.width) left = mx - 160;
      enTip.style.left = left + "px";
      enTip.style.top = (my - 30) + "px";
      enTip.style.display = "block";
    });
    enCv.addEventListener("mouseleave", function () {
      if (enTip) enTip.style.display = "none";
    });
  }

  // 统计清零
  var enReset = $("energy-reset");
  if (enReset) enReset.addEventListener("click", function () {
    if (!confirm("确定要清零所有耗电统计吗？累计/今日/本月与每日明细将全部归零，不可恢复。")) return;
    enMsg.className = "export-msg"; enMsg.textContent = "正在清零…";
    fetch("../api/crps/energy", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action: "reset" })
    })
      .then(function (r) { return r.json(); })
      .then(function (d) {
        if (d && d.ok) {
          enMsg.className = "export-msg ok"; enMsg.textContent = "耗电统计已清零";
          pollEnergy();
        } else {
          enMsg.className = "export-msg err"; enMsg.textContent = "清零失败: " + ((d && d.error) || "未知错误");
        }
      })
      .catch(function (e) {
        enMsg.className = "export-msg err"; enMsg.textContent = "清零失败: " + e.message;
      });
  });

  // ---- 启动 ----
  buildGrid();
  loadGpioConfig();
  poll();
  setInterval(poll, 3000);
  pollEnergy();
  setInterval(pollEnergy, 10000);
})();
