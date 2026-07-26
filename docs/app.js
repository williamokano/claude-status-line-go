(function(){
  'use strict';

  var reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  /* ── divider rows, sized to the element ──────────────────────── */
  function fillRules(){
    document.querySelectorAll('.rule').forEach(function(el){
      var n = Math.ceil(el.clientWidth / 8) + 4;
      el.textContent = '░'.repeat(n);
    });
  }
  fillRules();
  window.addEventListener('resize', fillRules);

  /* ── the right-edge fade only earns its place when there's more to see ── */
  var scroller = document.querySelector('.sl__scroll'),
      fade = document.querySelector('.sl__fade');
  function syncFade(){
    if (!scroller || !fade) return;
    fade.style.opacity = scroller.scrollWidth > scroller.clientWidth + 2 ? '1' : '0';
  }
  syncFade();
  window.addEventListener('resize', syncFade);
  scroller.addEventListener('scroll', syncFade);

  /* ── copy buttons ────────────────────────────────────────────── */
  document.querySelectorAll('[data-copy],[data-copy-from]').forEach(function(btn){
    btn.addEventListener('click', function(){
      var text = btn.dataset.copy != null
        ? btn.dataset.copy
        : document.querySelector(btn.dataset.copyFrom).textContent;
      navigator.clipboard.writeText(text).then(function(){
        btn.textContent = 'Copied';
        btn.dataset.done = '1';
        setTimeout(function(){ btn.textContent = 'Copy'; delete btn.dataset.done; }, 1600);
      });
    });
  });

  /* ── tabs — each list is independent of the others ───────────── */
  document.querySelectorAll('.tabs__list').forEach(function(list){
    var tabs = Array.prototype.slice.call(list.querySelectorAll('.tab'));
    function select(i){
      tabs.forEach(function(t, j){
        var on = i === j;
        t.setAttribute('aria-selected', on ? 'true' : 'false');
        t.tabIndex = on ? 0 : -1;
        document.getElementById(t.getAttribute('aria-controls')).hidden = !on;
      });
    }
    tabs.forEach(function(t, i){
      t.addEventListener('click', function(){ select(i); });
      t.addEventListener('keydown', function(e){
        var d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
        if (!d) return;
        e.preventDefault();
        var n = (i + d + tabs.length) % tabs.length;
        select(n);
        tabs[n].focus();
      });
    });
  });

  /* Open the Windows tab for Windows visitors. macOS/Linux stays the default,
     so a failed guess still lands on the tab that covers two of three. */
  var winTab = document.getElementById('tab-win');
  if (winTab) {
    var plat = (navigator.userAgentData && navigator.userAgentData.platform) ||
               navigator.platform || navigator.userAgent || '';
    if (/win/i.test(plat)) winTab.click();
  }

  /* ── scroll reveal ───────────────────────────────────────────── */
  var reveals = document.querySelectorAll('.reveal');
  if (reduce || !('IntersectionObserver' in window)) {
    reveals.forEach(function(el){ el.classList.add('is-in'); });
  } else {
    var io = new IntersectionObserver(function(entries){
      entries.forEach(function(en){
        if (en.isIntersecting) { en.target.classList.add('is-in'); io.unobserve(en.target); }
      });
    }, { rootMargin: '0px 0px -12% 0px' });
    reveals.forEach(function(el){ io.observe(el); });
  }

  /* ── the status line itself ──────────────────────────────────── */
  var BAR = 10, LOOP = 26000;
  var five = document.getElementById('five'),
      ctx = document.getElementById('ctx'),
      tokens = document.getElementById('tokens'),
      weekly = document.getElementById('weekly'),
      weeklyInner = document.getElementById('weeklyInner'),
      cost = document.getElementById('cost'),
      scrub = document.getElementById('scrub');

  // Pages without the hero simulation share this file, so bail out rather
  // than throwing on the first missing element.
  if (!five || !ctx || !tokens || !weekly || !weeklyInner || !cost) return;

  function bar(pct){
    var f = Math.round(pct / 100 * BAR);
    return '█'.repeat(f) + '░'.repeat(BAR - f);
  }
  function level(pct, warn, crit){
    return pct >= crit ? 'crit' : pct >= warn ? 'warn' : 'ok';
  }
  function k(n){
    return n >= 1000 ? Math.round(n / 1000) + 'k' : String(Math.round(n));
  }
  function hm(mins){
    var t = Math.round(mins), h = Math.floor(t / 60), m = t % 60;
    return h > 0 ? h + 'h' + m + 'm' : m + 'm';
  }

  function frame(p){
    var e = p * p * (3 - 2 * p);              // smoothstep

    var f = 92 * e;
    five.className = level(f, 60, 85);
    five.textContent = '🟡5h ' + bar(f) + ' ' + Math.round(f) + '% ↺ ' + hm(300 - 278 * e);

    var c = 71 * e;
    ctx.className = level(c, 60, 85);
    ctx.textContent = 'CTX ' + bar(c) + ' ' + Math.round(c) + '%';

    var hit = p < 0.12 ? 0 : Math.min(99, (p - 0.12) / 0.3 * 99);
    tokens.textContent = 'Σ' + k(115000 * e) + ' ↓' + k(277 * e) + ' ⚡' + Math.round(hit) + '%';

    var w = 74 * e;
    weeklyInner.className = level(w, 60, 85);
    weeklyInner.textContent = '📅7d ' + bar(w) + ' ' + Math.round(w) + '% ↺ 2d4h';
    weekly.classList.toggle('is-on', w >= 60);   // weekly_show_at

    cost.textContent = '$' + (7.92 * e).toFixed(2);
    scrub.textContent = '█'.repeat(Math.round(p * 24)) + '░'.repeat(24 - Math.round(p * 24));
  }

  if (reduce) {
    frame(1);
  } else {
    var t0 = null;
    requestAnimationFrame(function step(ts){
      if (t0 === null) t0 = ts;
      var p = ((ts - t0) % LOOP) / LOOP;
      frame(p < 0.88 ? p / 0.88 : 1);          // fill, then hold before looping
      requestAnimationFrame(step);
    });
  }
})();
