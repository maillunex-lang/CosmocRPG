// Лендинг: подставляет сведения о последней версии из подписанного манифеста (/download/latest.json).
// Подпись манифеста проверяет приложение; здесь данные только показываются, ссылка принимается лишь если она https и ведёт на этот же сайт.
(function () {
  var link = document.getElementById("dl");
  var info = document.getElementById("dlinfo");
  function fail() {
    info.textContent = "Не удалось получить сведения о версии. Обновите страницу позже.";
  }
  fetch("/download/latest.json", { cache: "no-store" })
    .then(function (r) { if (!r.ok) throw new Error("http " + r.status); return r.json(); })
    .then(function (m) {
      var url = String(m.url || "");
      var u = new URL(url, location.href);
      var same = u.hostname.replace(/^www\./, "") === location.hostname.replace(/^www\./, "");
      if (u.protocol !== "https:" || !same || u.pathname.indexOf("/download/") !== 0 || !/\.apk$/.test(u.pathname)) throw new Error("bad url");
      link.href = u.href;
      var mb = (Number(m.size) / 1048576).toFixed(1);
      var d = m.published ? new Date(m.published).toLocaleDateString("ru-RU") : "";
      info.textContent = "Версия " + m.version_name + " (" + m.version_code + ") · " + mb + " МБ" + (d ? " · " + d : "") + (m.notes ? " · " + m.notes : "");
    })
    .catch(fail);
})();
