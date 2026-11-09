// Лендинг «Звёздные рубежи». Без внешних библиотек и инлайн-стилей (CSP). Три части:
// 1) сведения о последней версии из /download/latest.json (подпись манифеста проверяет приложение; здесь данные только показываются);
// 2) оформление: звёздное небо с параллаксом, блик «стекла» за курсором, наклон окна с игрой, появление блоков;
// 3) просмотр скриншотов в диалоге.
(function () {
  "use strict";
  var reduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  /* ---------- 1. версия и ссылка на скачивание ---------- */
  var link = document.getElementById("dl");
  var info = document.getElementById("dlinfo");
  var chipVer = document.getElementById("chip-ver");
  var chipSize = document.getElementById("chip-size");
  var chipDate = document.getElementById("chip-date");
  function fail() {
    info.textContent = "Не удалось получить сведения о версии. Обновите страницу позже.";
    chipVer.textContent = "Версия: не удалось получить";
  }
  fetch("/download/latest.json", { cache: "no-store" })
    .then(function (r) { if (!r.ok) throw new Error("http " + r.status); return r.json(); })
    .then(function (m) {
      var url = String(m.url || "");
      var u = new URL(url, location.href);
      var same = u.hostname.replace(/^www\./, "") === location.hostname.replace(/^www\./, "");
      // ссылка принимается только по https, на этот же сайт и только на файл .apk из каталога загрузок
      if (u.protocol !== "https:" || !same || u.pathname.indexOf("/download/") !== 0 || !/\.apk$/.test(u.pathname)) throw new Error("bad url");
      link.href = u.href;
      var mb = (Number(m.size) / 1048576).toFixed(1);
      var d = m.published ? new Date(m.published).toLocaleDateString("ru-RU") : "";
      chipVer.textContent = "Версия " + m.version_name + " (" + m.version_code + ")";
      chipSize.textContent = mb + " МБ"; chipSize.hidden = false;
      if (d) { chipDate.textContent = d; chipDate.hidden = false; }
      info.textContent = m.notes ? "Что нового: " + m.notes : "";
    })
    .catch(fail);

  /* ---------- 2. оформление ---------- */
  // звёздное небо: три слоя глубины, мерцание, редкие падающие звёзды; пауза, когда вкладка скрыта
  var canvas = document.getElementById("space");
  if (canvas && canvas.getContext) {
    var ctx = canvas.getContext("2d");
    var stars = [], shoot = null, W = 0, H = 0, dpr = 1, px = 0, py = 0, scrollY = 0, nextShoot = 4000, last = 0, raf = 0;
    var rnd = function (a, b) { return a + Math.random() * (b - a); };
    var resize = function () {
      dpr = Math.min(window.devicePixelRatio || 1, 2);
      W = window.innerWidth; H = window.innerHeight;
      canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      var n = Math.min(260, Math.round((W * H) / 7500));
      stars = [];
      for (var i = 0; i < n; i++) {
        var z = Math.random();
        stars.push({ x: Math.random() * W, y: Math.random() * H, z: z, r: 0.4 + z * 1.4, ph: Math.random() * 6.28, sp: rnd(0.6, 2.2),
          c: Math.random() < 0.18 ? "170,210,255" : (Math.random() < 0.1 ? "255,200,230" : "255,255,255") });
      }
    };
    var draw = function (t) {
      ctx.clearRect(0, 0, W, H);
      for (var i = 0; i < stars.length; i++) {
        var s = stars[i];
        var x = s.x + px * s.z * 26;
        var y = ((s.y - scrollY * (0.04 + s.z * 0.18) + px * 0) % H + H) % H + py * s.z * 18;
        var a = 0.35 + 0.65 * (0.5 + 0.5 * Math.sin(t * 0.001 * s.sp + s.ph));
        ctx.fillStyle = "rgba(" + s.c + "," + (a * (0.45 + s.z * 0.55)).toFixed(3) + ")";
        ctx.beginPath(); ctx.arc(x, y, s.r, 0, 6.2832); ctx.fill();
      }
      if (shoot) {
        var g = ctx.createLinearGradient(shoot.x, shoot.y, shoot.x - shoot.vx * 14, shoot.y - shoot.vy * 14);
        g.addColorStop(0, "rgba(255,255,255," + shoot.a.toFixed(2) + ")"); g.addColorStop(1, "rgba(255,255,255,0)");
        ctx.strokeStyle = g; ctx.lineWidth = 1.6; ctx.beginPath();
        ctx.moveTo(shoot.x, shoot.y); ctx.lineTo(shoot.x - shoot.vx * 14, shoot.y - shoot.vy * 14); ctx.stroke();
      }
    };
    var loop = function (t) {
      var dt = t - last; last = t;
      nextShoot -= dt;
      if (!shoot && nextShoot <= 0) { shoot = { x: rnd(W * 0.2, W), y: rnd(0, H * 0.4), vx: -rnd(9, 14), vy: rnd(4, 7), a: 1 }; nextShoot = rnd(7000, 14000); }
      if (shoot) { shoot.x += shoot.vx; shoot.y += shoot.vy; shoot.a -= 0.012; if (shoot.a <= 0 || shoot.x < -50 || shoot.y > H + 50) shoot = null; }
      draw(t);
      raf = requestAnimationFrame(loop);
    };
    resize();
    window.addEventListener("resize", function () { resize(); if (reduce) draw(0); });
    window.addEventListener("scroll", function () { scrollY = window.scrollY; if (reduce) draw(0); }, { passive: true });
    if (reduce) { draw(0); }
    else {
      raf = requestAnimationFrame(loop);
      document.addEventListener("visibilitychange", function () {
        if (document.hidden) cancelAnimationFrame(raf); else { last = performance.now(); raf = requestAnimationFrame(loop); }
      });
      window.addEventListener("pointermove", function (e) { px = (e.clientX / W - 0.5) * 2; py = (e.clientY / H - 0.5) * 2; }, { passive: true });
    }
  }

  // блик «жидкого стекла» следует за курсором
  var glass = document.querySelectorAll(".glass");
  Array.prototype.forEach.call(glass, function (el) {
    el.addEventListener("pointermove", function (e) {
      var r = el.getBoundingClientRect();
      el.style.setProperty("--mx", (e.clientX - r.left).toFixed(0) + "px");
      el.style.setProperty("--my", (e.clientY - r.top).toFixed(0) + "px");
    }, { passive: true });
  });

  // лёгкий наклон окна с игрой за курсором (не на сенсорных экранах и не при «уменьшенной анимации»)
  var tilt = document.querySelector(".tilt");
  var hero = document.querySelector(".hero-shot");
  if (tilt && hero && !reduce && window.matchMedia("(hover: hover)").matches) {
    hero.addEventListener("pointermove", function (e) {
      var r = hero.getBoundingClientRect();
      var x = (e.clientX - r.left) / r.width - 0.5, y = (e.clientY - r.top) / r.height - 0.5;
      tilt.style.setProperty("--ry", (x * 9).toFixed(2) + "deg");
      tilt.style.setProperty("--rx", (-y * 7).toFixed(2) + "deg");
    });
    hero.addEventListener("pointerleave", function () { tilt.style.setProperty("--ry", "0deg"); tilt.style.setProperty("--rx", "0deg"); });
  }

  // блоки плавно появляются при прокрутке; без JS или без IntersectionObserver всё видно сразу
  var items = document.querySelectorAll(".reveal");
  if ("IntersectionObserver" in window && !reduce) {
    var io = new IntersectionObserver(function (entries) {
      entries.forEach(function (en) { if (en.isIntersecting) { en.target.classList.add("in"); io.unobserve(en.target); } });
    }, { rootMargin: "0px 0px -8% 0px", threshold: 0.08 });
    Array.prototype.forEach.call(items, function (el, i) { el.style.transitionDelay = ((i % 4) * 70) + "ms"; io.observe(el); });
  } else {
    Array.prototype.forEach.call(items, function (el) { el.classList.add("in"); });
  }

  /* ---------- 3. просмотр скриншотов ---------- */
  var dlg = document.getElementById("lightbox");
  var img = document.getElementById("lightbox-img");
  var cap = document.getElementById("lightbox-cap");
  if (dlg && typeof dlg.showModal === "function") {
    Array.prototype.forEach.call(document.querySelectorAll(".zoom"), function (b) {
      b.addEventListener("click", function () {
        var src = b.getAttribute("data-full") || "";
        if (src.indexOf("/img/") !== 0) return; // только свои снимки
        img.src = src; img.alt = b.getAttribute("data-cap") || ""; cap.textContent = b.getAttribute("data-cap") || "";
        dlg.showModal();
      });
    });
    dlg.addEventListener("click", function (e) { if (e.target === dlg) dlg.close(); });
  }
})();
