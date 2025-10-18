(() => {
  'use strict';

  const form = document.getElementById('opts');
  if (!form) {
    return;
  }

  const layoutRoot = document.querySelector('.layout');
  const filtersShell = document.querySelector('.filters');
  const filterPanel = document.getElementById('filter-panel');
  const collapseBtn = document.getElementById('collapseFilters');
  const presetWrap = document.getElementById('preset-chips');
  const errorBanner = document.getElementById('error-banner');
  const errorMessage = document.getElementById('error-message');
  const errorClose = document.getElementById('error-close');
  const progressWrap = document.getElementById('progress');
  const progressStage = document.getElementById('progress-stage');
  const progressStats = document.getElementById('progress-stats');
  const progressETA = document.getElementById('progress-eta');
  const progressRate = document.getElementById('progress-rate');
  const progressElapsed = document.getElementById('progress-elapsed');
  const progressMeter = document.getElementById('progress-meter');
  const progressCancel = document.getElementById('progress-cancel');
  const stScan = document.getElementById('st-scan');
  const stAttr = document.getElementById('st-attr');
  const stPr = document.getElementById('st-pr');
  const resultTable = document.getElementById('result-table');
  const resultCount = document.getElementById('result-count');
  const resultErrors = document.getElementById('result-errors');
  const exportJSONBtn = document.getElementById('export-json');
  const exportTSVBtn = document.getElementById('export-tsv');
  const cliOutput = document.getElementById('cli-command');
  const cliPreview = document.getElementById('cli-preview');
  const cliHelp = document.getElementById('cli-help');
  const shareURLEl = document.getElementById('share-url');
  const cancelBtn = document.getElementById('cancel-scan');
  const resetBtn = document.getElementById('reset-form');
  const fieldAutoNote = document.getElementById('field-auto-note');
  const prOptions = document.getElementById('pr-options');

  const LOCAL_STORAGE_KEY = 'todox:lastParams';

  let es = null;
  let fetchAbort = null;
  let latestResult = null;
  let tableRows = [];
  let sortKey = null;
  let sortDesc = false;
  let lastSnap = null;
  let rafId = 0;

  function showError(message) {
    if (!errorBanner || !errorMessage) {
      return;
    }
    errorMessage.textContent = message;
    errorBanner.classList.remove('is-hidden');
    errorBanner.removeAttribute('hidden');
  }

  function hideError() {
    if (!errorBanner || !errorMessage) {
      return;
    }
    errorBanner.classList.add('is-hidden');
    errorBanner.setAttribute('hidden', '');
    errorMessage.textContent = '';
  }

  if (errorClose) {
    errorClose.addEventListener('click', (ev) => {
      ev.preventDefault();
      hideError();
    });
  }

  function setPanelVisibility(panelEl, expanded) {
    if (collapseBtn) {
      collapseBtn.setAttribute('aria-expanded', String(expanded));
      collapseBtn.setAttribute('aria-label', expanded ? 'フィルターを隠す' : 'フィルターを表示する');
    }
    if (filtersShell instanceof HTMLElement) {
      filtersShell.classList.toggle('is-collapsed', !expanded);
    }
    if (layoutRoot instanceof HTMLElement) {
      layoutRoot.classList.toggle('is-single', !expanded);
    }
    if (!(panelEl instanceof HTMLElement)) {
      return;
    }
    if (expanded) {
      panelEl.classList.remove('is-hidden');
      panelEl.removeAttribute('hidden');
    } else {
      panelEl.classList.add('is-hidden');
      panelEl.setAttribute('hidden', '');
    }
  }

  function togglePanelFromTrigger(trigger) {
    if (!trigger) {
      return;
    }
    const selector = trigger.getAttribute('data-toggle');
    if (!selector) {
      return;
    }
    const target = document.querySelector(selector);
    if (!(target instanceof HTMLElement)) {
      return;
    }
    const expanded = trigger.getAttribute('aria-expanded') === 'true';
    const next = !expanded;
    trigger.setAttribute('aria-expanded', String(next));
    if (trigger === collapseBtn) {
      setPanelVisibility(target, next);
    } else {
      if (next) {
        target.classList.remove('is-hidden');
        target.removeAttribute('hidden');
      } else {
        target.classList.add('is-hidden');
        target.setAttribute('hidden', '');
      }
    }
  }

  document.addEventListener('click', (ev) => {
    const target = ev.target instanceof HTMLElement ? ev.target.closest('[data-toggle]') : null;
    if (!(target instanceof HTMLElement)) {
      return;
    }
    ev.preventDefault();
    togglePanelFromTrigger(target);
  });

  if (collapseBtn && filterPanel) {
    collapseBtn.addEventListener('keydown', (ev) => {
      if (ev.key === 'Enter' || ev.key === ' ') {
        ev.preventDefault();
        togglePanelFromTrigger(collapseBtn);
      }
    });
  }

  document.addEventListener('keydown', (ev) => {
    if (ev.key !== 'Escape') {
      return;
    }
    if (!collapseBtn || !filterPanel) {
      return;
    }
    const expanded = collapseBtn.getAttribute('aria-expanded') === 'true';
    if (!expanded) {
      return;
    }
    const activeElement = document.activeElement;
    if (filterPanel.contains(activeElement)) {
      setPanelVisibility(filterPanel, false);
      collapseBtn.focus();
    }
  });

  const desktopMedia = window.matchMedia('(min-width: 1025px)');
  const handleDesktopChange = (event) => {
    const shouldOpen = !!event.matches;
    setPanelVisibility(filterPanel, shouldOpen);
  };
  if (typeof desktopMedia.addEventListener === 'function') {
    desktopMedia.addEventListener('change', handleDesktopChange);
  } else if (typeof desktopMedia.addListener === 'function') {
    desktopMedia.addListener(handleDesktopChange);
  }
  handleDesktopChange(desktopMedia);

  const helpButtons = form.querySelectorAll('.help-toggle');
  helpButtons.forEach((btn) => {
    btn.addEventListener('click', () => {
      const expanded = btn.getAttribute('aria-expanded') === 'true';
      const helpText = btn.parentElement ? btn.parentElement.nextElementSibling : null;
      if (helpText && helpText.classList.contains('help-text')) {
        if (expanded) {
          helpText.hidden = true;
          btn.setAttribute('aria-expanded', 'false');
        } else {
          helpText.hidden = false;
          btn.setAttribute('aria-expanded', 'true');
        }
      }
    });
  });

  function escText(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  function escAttr(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function normalizePRTooltip(body) {
    const collapsed = String(body ?? '')
      .replace(/\r?\n/g, ' ')
      .replace(/\\[rn]/g, ' ')
      .replace(/\s+/g, ' ')
      .trim();
    if (!collapsed) {
      return '';
    }
    return collapsed.length > 280 ? `${collapsed.slice(0, 279)}…` : collapsed;
  }

  function cssEscape(value) {
    if (typeof CSS !== 'undefined' && typeof CSS.escape === 'function') {
      return CSS.escape(String(value));
    }
    return String(value).replace(/[^a-zA-Z0-9_\-]/g, (ch) => {
      const hex = ch.charCodeAt(0).toString(16);
      return `\\${hex} `;
    });
  }

  function renderBadge(kind) {
    const raw = String(kind ?? '');
    if (!raw) {
      return '';
    }
    const upper = raw.toUpperCase();
    let dataType = 'OTHER';
    if (upper === 'TODO' || upper === 'FIXME') {
      dataType = upper;
    }
    return `<span class="badge" data-type="${dataType}">${escText(upper)}</span>`;
  }

  function buildHeaderMeta(info) {
    const meta = [
      { key: 'kind', label: 'TYPE' },
      { key: 'author', label: 'AUTHOR' },
      { key: 'email', label: 'EMAIL' },
      { key: 'date', label: 'DATE' },
    ];
    if (info && info.has_age) {
      meta.push({ key: 'age_days', label: 'AGE' });
    }
    meta.push({ key: 'commit', label: 'COMMIT' });
    meta.push({ key: 'location', label: 'LOCATION' });
    if (info && info.has_url) {
      meta.push({ key: 'url', label: 'URL' });
    }
    if (info && info.has_prs) {
      meta.push({ key: 'prs', label: 'PRS' });
    }
    if (info && info.has_comment) {
      meta.push({ key: 'comment', label: 'COMMENT' });
    }
    if (info && info.has_message) {
      meta.push({ key: 'message', label: 'MESSAGE' });
    }
    return meta;
  }

  function renderPRCell(value) {
    const list = Array.isArray(value) ? value : [];
    if (!list.length) {
      return '';
    }
    const entries = [];
    for (const pr of list) {
      const info = pr || {};
      const numValue = typeof info.number === 'number' ? info.number : Number(info.number);
      const hasNumber = Number.isFinite(numValue) && numValue > 0;
      const numberText = hasNumber ? String(Math.trunc(numValue)) : '';
      const stateRaw = info && info.state != null ? String(info.state) : '';
      const state = stateRaw.trim().toLowerCase() || 'unknown';
      const titleRaw = info && info.title != null ? String(info.title) : '';
      const titleText = titleRaw.trim();
      const tooltip = normalizePRTooltip(info.body);
      const anchorLabel = hasNumber ? `#${numberText}` : (titleText || state || '#');
      const ariaParts = [];
      if (hasNumber) {
        ariaParts.push(`PR #${numberText}`);
      } else {
        ariaParts.push('Pull request');
        if (titleText) {
          ariaParts.push(titleText);
        }
      }
      if (state) {
        ariaParts.push(state);
      }
      const ariaLabel = ariaParts.join(' ').trim();
      let leading = escText(anchorLabel);
      if (info.url) {
        const attrs = [
          `href="${escAttr(String(info.url))}"`,
          'target="_blank"',
          'rel="noopener noreferrer"',
        ];
        if (ariaLabel) {
          attrs.push(`aria-label="${escAttr(ariaLabel)}"`);
        }
        if (tooltip) {
          attrs.push(`title="${escAttr(tooltip)}"`);
        }
        leading = `<a ${attrs.join(' ')}>${escText(anchorLabel)}</a>`;
      } else if (tooltip) {
        leading = `<span title="${escAttr(tooltip)}">${leading}</span>`;
      }
      let titleSuffix = '';
      if (hasNumber && titleText) {
        titleSuffix = ` ${escText(titleText)}`;
      } else if (!hasNumber && titleText && anchorLabel !== titleText) {
        titleSuffix = ` ${escText(titleText)}`;
      }
      entries.push(`${leading}${titleSuffix} (${escText(state)})`);
    }
    return entries.join('; ');
  }

  function renderTableCell(key, row) {
    const r = row || {};
    switch (key) {
      case 'kind':
        return renderBadge(r.kind);
      case 'author':
        return escText(r.author);
      case 'email':
        return escText(r.email);
      case 'date':
        return escText(r.date);
      case 'age_days': {
        const ageRaw = (typeof r.age_days === 'number' && isFinite(r.age_days)) ? String(r.age_days) : (r.age_days === 0 ? '0' : '');
        return escText(ageRaw);
      }
      case 'commit': {
        const commitRaw = r.commit ? String(r.commit).slice(0, 8) : '';
        return `<code>${escText(commitRaw)}</code>`;
      }
      case 'location': {
        const fileRaw = r.file ? String(r.file) : '';
        const lineRaw = (typeof r.line === 'number' && r.line > 0) ? String(r.line) : '';
        return `<code>${escText(`${fileRaw}:${lineRaw}`)}</code>`;
      }
      case 'url': {
        const urlRaw = r.url ? String(r.url) : '';
        if (!urlRaw) {
          return '';
        }
        return `<a class="link-icon" href="${escAttr(urlRaw)}" target="_blank" rel="noopener noreferrer" aria-label="GitHubで開く"><span aria-hidden="true">🔗</span></a>`;
      }
      case 'prs':
        return renderPRCell(r.prs);
      case 'comment':
        return escText(r.comment);
      case 'message':
        return escText(r.message);
      default:
        return escText(r[key]);
    }
  }

  function renderResultTable(data, options) {
    const info = (data && typeof data === 'object') ? data : {};
    const opts = options || {};
    const rowsSource = opts.rows;
    const rows = Array.isArray(rowsSource) ? rowsSource : (Array.isArray(info.items) ? info.items : []);
    const errs = Array.isArray(info.errors) ? info.errors : [];
    const parts = [];
    if (errs.length > 0) {
      let list = '<ul>';
      for (const e of errs) {
        const fileRaw = e && e.file ? e.file : '(unknown)';
        const lineRaw = e && typeof e.line === 'number' && e.line > 0 ? String(e.line) : '—';
        const loc = `${fileRaw}:${lineRaw}`;
        const stage = e && e.stage ? e.stage : 'git';
        list += `<li><code>${escText(loc)}</code> [${escText(stage)}] ${escText(e && e.message ? e.message : '')}</li>`;
      }
      list += '</ul>';
      parts.push(`<div class="errors"><strong>${errs.length} error(s)</strong>${list}</div>`);
    }
    if (!rows || rows.length === 0) {
      parts.push('<p>該当する結果はありません。</p>');
      return parts.join('');
    }
    const headerMeta = buildHeaderMeta(info);
    const activeKey = opts.sortKey ? String(opts.sortKey) : null;
    const activeDesc = !!opts.sortDesc;
    const head = headerMeta.map((meta) => {
      const classes = ['sort-btn'];
      if (activeKey === meta.key) {
        classes.push(activeDesc ? 'desc' : 'asc');
      }
      const ariaSort = activeKey === meta.key ? (activeDesc ? 'descending' : 'ascending') : 'none';
      return `<th aria-sort="${ariaSort}"><button type="button" class="${classes.join(' ')}" data-key="${escAttr(meta.key)}">${escText(meta.label)}</button></th>`;
    }).join('');
    let tableHTML = `<table><thead><tr>${head}</tr></thead><tbody>`;
    for (const r of rows) {
      const cells = [];
      for (const meta of headerMeta) {
        cells.push(`<td>${renderTableCell(meta.key, r, info)}</td>`);
      }
      tableHTML += `<tr>${cells.join('')}</tr>`;
    }
    tableHTML += '</tbody></table>';
    parts.push(tableHTML);
    return parts.join('');
  }

  function compareStrings(a, b) {
    const sa = a == null ? '' : String(a).trim();
    const sb = b == null ? '' : String(b).trim();
    const emptyA = sa === '';
    const emptyB = sb === '';
    if (emptyA && emptyB) {
      return 0;
    }
    if (emptyA) {
      return 1;
    }
    if (emptyB) {
      return -1;
    }
    if (sa < sb) {
      return -1;
    }
    if (sa > sb) {
      return 1;
    }
    return 0;
  }

  function compareNumbers(a, b) {
    const na = typeof a === 'number' && isFinite(a) ? a : null;
    const nb = typeof b === 'number' && isFinite(b) ? b : null;
    if (na == null && nb == null) {
      return 0;
    }
    if (na == null) {
      return 1;
    }
    if (nb == null) {
      return -1;
    }
    return na - nb;
  }

  function compareLocation(a, b) {
    const fileA = a && a.file ? String(a.file) : '';
    const fileB = b && b.file ? String(b.file) : '';
    const fileCmp = compareStrings(fileA, fileB);
    if (fileCmp !== 0) {
      return fileCmp;
    }
    const lineA = a && typeof a.line === 'number' ? a.line : null;
    const lineB = b && typeof b.line === 'number' ? b.line : null;
    if (lineA == null && lineB == null) {
      return 0;
    }
    if (lineA == null) {
      return 1;
    }
    if (lineB == null) {
      return -1;
    }
    return lineA - lineB;
  }

  function comparePRs(a, b) {
    const listA = Array.isArray(a && a.prs) ? a.prs : [];
    const listB = Array.isArray(b && b.prs) ? b.prs : [];
    if (!listA.length && !listB.length) {
      return 0;
    }
    if (!listA.length) {
      return 1;
    }
    if (!listB.length) {
      return -1;
    }
    const firstA = listA[0] || {};
    const firstB = listB[0] || {};
    const numCmp = compareNumbers(firstA.number, firstB.number);
    if (numCmp !== 0) {
      return numCmp;
    }
    const titleCmp = compareStrings(firstA.title, firstB.title);
    if (titleCmp !== 0) {
      return titleCmp;
    }
    return compareStrings(firstA.state, firstB.state);
  }

  function isValueEmptyForKey(row, key) {
    const r = row || {};
    switch (key) {
      case 'age_days':
        return !(typeof r.age_days === 'number' && isFinite(r.age_days));
      case 'commit':
      case 'date':
      case 'author':
      case 'email':
      case 'kind':
      case 'comment':
      case 'message':
      case 'url': {
        const val = r[key];
        return val == null || String(val).trim() === '';
      }
      case 'location': {
        const file = r.file != null ? String(r.file).trim() : '';
        const hasLine = typeof r.line === 'number' && r.line > 0;
        return !file && !hasLine;
      }
      case 'prs':
        return !(Array.isArray(r.prs) && r.prs.length > 0);
      default: {
        const val = r[key];
        return val == null || String(val).trim() === '';
      }
    }
  }

  function compareRows(a, b, key) {
    switch (key) {
      case 'age_days':
        return compareNumbers(a && a.age_days, b && b.age_days);
      case 'commit':
        return compareStrings(a && a.commit, b && b.commit);
      case 'date':
        return compareStrings(a && a.date, b && b.date);
      case 'author':
        return compareStrings(a && a.author, b && b.author);
      case 'email':
        return compareStrings(a && a.email, b && b.email);
      case 'kind':
        return compareStrings(a && a.kind, b && b.kind);
      case 'comment':
        return compareStrings(a && a.comment, b && b.comment);
      case 'message':
        return compareStrings(a && a.message, b && b.message);
      case 'url':
        return compareStrings(a && a.url, b && b.url);
      case 'location':
        return compareLocation(a, b);
      case 'prs':
        return comparePRs(a, b);
      default:
        return compareStrings(a && a[key], b && b[key]);
    }
  }

  function sortRows(rows, key, desc) {
    const base = Array.isArray(rows) ? rows.slice() : [];
    if (!key) {
      return base;
    }
    base.sort((left, right) => {
      const leftEmpty = isValueEmptyForKey(left, key);
      const rightEmpty = isValueEmptyForKey(right, key);
      if (leftEmpty !== rightEmpty) {
        return leftEmpty ? 1 : -1;
      }
      const cmp = compareRows(left, right, key);
      return desc ? -cmp : cmp;
    });
    return base;
  }

  function attachSortHandlers() {
    if (!resultTable) {
      return;
    }
    const buttons = resultTable.querySelectorAll('th .sort-btn');
    for (const btn of buttons) {
      btn.addEventListener('click', (ev) => {
        ev.preventDefault();
        const key = btn.getAttribute('data-key');
        if (!key) {
          return;
        }
        if (sortKey === key) {
          sortDesc = !sortDesc;
        } else {
          sortKey = key;
          sortDesc = false;
        }
        renderTableWithSort();
      });
    }
  }

  function renderTableWithSort() {
    const base = (latestResult && typeof latestResult === 'object') ? latestResult : {};
    const sorted = sortRows(tableRows, sortKey, sortDesc);
    if (resultTable) {
      resultTable.innerHTML = renderResultTable(base, { rows: sorted, sortKey, sortDesc });
      attachSortHandlers();
    }
  }

  function updateResultData(data) {
    latestResult = (data && typeof data === 'object') ? data : null;
    tableRows = Array.isArray(latestResult && latestResult.items) ? latestResult.items.slice() : [];
    sortKey = null;
    sortDesc = false;
    renderTableWithSort();
    const count = tableRows.length;
    if (resultCount) {
      resultCount.textContent = `結果: ${count} 件`;
    }
    if (exportJSONBtn) {
      exportJSONBtn.disabled = !latestResult || !tableRows.length;
    }
    if (exportTSVBtn) {
      exportTSVBtn.disabled = !latestResult || !tableRows.length;
    }
    if (resultErrors) {
      const errs = Array.isArray(latestResult && latestResult.errors) ? latestResult.errors : [];
      if (errs.length > 0) {
        resultErrors.hidden = false;
        resultErrors.textContent = `エラー: ${errs.length} 件`;
      } else {
        resultErrors.hidden = true;
        resultErrors.textContent = '';
      }
    }
  }

  function resetProgressUI() {
    if (!progressStage) {
      return;
    }
    progressStage.textContent = 'scan';
    if (progressStats) {
      progressStats.textContent = '0/— done';
    }
    if (progressETA) {
      progressETA.textContent = 'ETA: —';
    }
    if (progressRate) {
      progressRate.textContent = 'Rate: —';
    }
    if (progressElapsed) {
      progressElapsed.textContent = 'Elapsed: —';
    }
    updateProgressBar(0, false);
    setStepper('scan');
  }
  function showProgressUI() {
    if (!progressWrap) {
      return;
    }
    progressWrap.hidden = false;
    progressWrap.classList.remove('is-hidden');
    progressWrap.setAttribute('aria-busy', 'true');
    resetProgressUI();
  }

  function hideProgressUI() {
    if (!progressWrap) {
      return;
    }
    progressWrap.hidden = true;
    progressWrap.classList.add('is-hidden');
    progressWrap.removeAttribute('aria-busy');
    lastSnap = null;
    if (rafId && typeof cancelAnimationFrame === 'function') {
      cancelAnimationFrame(rafId);
      rafId = 0;
    }
    if (progressCancel) {
      progressCancel.onclick = null;
    }
  }

  function setStepper(stage) {
    const stages = { scan: stScan, attr: stAttr, pr: stPr };
    for (const key of Object.keys(stages)) {
      const el = stages[key];
      if (!el) {
        continue;
      }
      el.removeAttribute('data-active');
      el.removeAttribute('data-status');
    }
    if (stage === 'scan') {
      if (stScan) {
        stScan.setAttribute('data-active', 'true');
      }
    } else if (stage === 'attr') {
      if (stScan) {
        stScan.setAttribute('data-status', 'done');
      }
      if (stAttr) {
        stAttr.setAttribute('data-active', 'true');
      }
    } else if (stage === 'pr') {
      if (stScan) {
        stScan.setAttribute('data-status', 'done');
      }
      if (stAttr) {
        stAttr.setAttribute('data-status', 'done');
      }
      if (stPr) {
        stPr.setAttribute('data-active', 'true');
      }
    }
  }

  function togglePRStepVisibility(enabled) {
    if (!stPr) {
      return;
    }
    if (enabled) {
      stPr.classList.remove('is-hidden');
    } else {
      stPr.classList.add('is-hidden');
      stPr.removeAttribute('data-active');
      stPr.removeAttribute('data-status');
    }
  }

  function updateProgressBar(percent, determinate) {
    const pct = Math.max(0, Math.min(100, percent || 0));
    if (!progressMeter) {
      return;
    }
    if (determinate) {
      progressMeter.max = 100;
      progressMeter.value = Math.round(pct);
      progressMeter.removeAttribute('data-indeterminate');
    } else {
      progressMeter.removeAttribute('value');
      progressMeter.removeAttribute('max');
      progressMeter.setAttribute('data-indeterminate', 'true');
    }
  }

  function renderProgressOnce() {
    rafId = 0;
    if (!lastSnap) {
      return;
    }
    const s = lastSnap;
    const nf = (typeof Intl !== 'undefined' && Intl.NumberFormat) ? new Intl.NumberFormat() : { format: (v) => String(v) };
    if (progressStage) {
      progressStage.textContent = s.stage || '';
    }
    setStepper(s.stage);
    const determinate = !!(s.total && s.total > 0);
    let percent = 0;
    if (determinate) {
      percent = (s.done / s.total) * 100;
    }
    updateProgressBar(percent, determinate);
    const totalLabel = determinate ? nf.format(s.total) : '—';
    const statText = `${nf.format(s.done || 0)}/${totalLabel} done`;
    const rateLabel = (typeof s.rate_per_sec === 'number' && isFinite(s.rate_per_sec)) ? `${nf.format(Number(s.rate_per_sec.toFixed(1)))}/sec` : '—';
    const fmtEta = (sec) => {
      if (!(typeof sec === 'number' && isFinite(sec))) {
        return '—';
      }
      const mm = Math.floor(sec / 60);
      const ss = Math.round(sec % 60);
      return `${nf.format(Number(sec.toFixed(1)))}s (${mm}:${String(ss).padStart(2, '0')})`;
    };
    const etaLabel = fmtEta(s.eta_sec_p50);
    const elapsedLabel = (typeof s.elapsed_sec === 'number' && isFinite(s.elapsed_sec)) ? `${s.elapsed_sec.toFixed(1)}s` : '—';
    if (progressStats) {
      progressStats.textContent = statText;
    }
    if (progressRate) {
      progressRate.textContent = `Rate: ${rateLabel}`;
    }
    if (progressETA) {
      progressETA.textContent = `ETA: ${etaLabel}`;
    }
    if (progressElapsed) {
      progressElapsed.textContent = `Elapsed: ${elapsedLabel}`;
    }
  }

  function scheduleRender() {
    if (typeof requestAnimationFrame !== 'function') {
      renderProgressOnce();
      return;
    }
    if (rafId) {
      return;
    }
    rafId = requestAnimationFrame(renderProgressOnce);
  }

  function closeStream() {
    if (es) {
      try { es.close(); } catch (_err) {}
      es = null;
    }
  }

  function cancelScan() {
    closeStream();
    if (fetchAbort) {
      try {
        fetchAbort.abort();
      } catch (_err) {}
      fetchAbort = null;
    }
    hideProgressUI();
  }

  if (cancelBtn) {
    cancelBtn.addEventListener('click', () => {
      cancelScan();
    });
  }

  function startScanWithFetch(q) {
    hideProgressUI();
    if (resultTable) {
      resultTable.innerHTML = '';
    }
    latestResult = null;
    tableRows = [];
    sortKey = null;
    sortDesc = false;
    fetchAbort = new AbortController();
    const { signal } = fetchAbort;
    fetch(`/api/scan?${q.toString()}`, { signal })
      .then((res) => res.text().then((raw) => ({ res, raw })))
      .then(({ res, raw }) => {
        if (!res.ok) {
          let msg = `HTTP ${res.status}`;
          if (res.statusText) {
            msg += ` ${res.statusText}`;
          }
          const trimmed = raw.trim();
          if (trimmed) {
            msg += `: ${trimmed}`;
          }
          throw new Error(msg);
        }
        let data = null;
        const trimmed = raw.trim();
        if (trimmed !== '') {
          data = JSON.parse(trimmed);
        }
        updateResultData(data);
      })
      .catch((err) => {
        if (err && err.name === 'AbortError') {
          return;
        }
        console.error(err);
        showError(err instanceof Error ? err.message : String(err));
      })
      .finally(() => {
        fetchAbort = null;
      });
  }

  function startScanWithSSE(q) {
    cancelScan();
    if (resultTable) {
      resultTable.innerHTML = '';
    }
    latestResult = null;
    tableRows = [];
    sortKey = null;
    sortDesc = false;
    showProgressUI();
    try {
      es = new EventSource(`/api/scan/stream?${q.toString()}`);
    } catch (err) {
      hideProgressUI();
      showError(err instanceof Error ? err.message : String(err));
      return;
    }

    es.addEventListener('progress', (ev) => {
      try {
        lastSnap = JSON.parse(ev.data);
        scheduleRender();
      } catch (parseErr) {
        console.warn('progress parse failed', parseErr);
      }
    });

    es.addEventListener('result', (ev) => {
      try {
        const res = JSON.parse(ev.data);
        hideProgressUI();
        updateResultData(res);
      } catch (parseErr) {
        console.error(parseErr);
        showError(parseErr instanceof Error ? parseErr.message : String(parseErr));
      } finally {
        closeStream();
      }
    });

    const handleServerError = (ev) => {
      if (!ev || typeof ev.data === 'undefined') {
        return;
      }
      try {
        const payload = JSON.parse(ev.data || '{}');
        showError(payload && payload.message ? payload.message : 'stream error');
      } catch (parseErr) {
        showError(parseErr instanceof Error ? parseErr.message : String(parseErr));
      }
      hideProgressUI();
      closeStream();
    };

    es.addEventListener('error', handleServerError);
    es.addEventListener('server_error', handleServerError);

    es.onerror = (ev) => {
      if (ev && typeof ev.data !== 'undefined') {
        return;
      }
      if (!es) {
        return;
      }
      if (progressStats) {
        const label = (lastSnap && es.readyState === EventSource.CONNECTING) ? '再接続中…' : '接続中…';
        progressStats.textContent = label;
      }
    };

    if (progressCancel) {
      progressCancel.onclick = () => {
        cancelScan();
      };
    }
  }

  function toCSVList(raw) {
    const values = Array.isArray(raw) ? raw : [raw];
    const out = [];
    for (const val of values) {
      if (val == null) {
        continue;
      }
      const pieces = String(val)
        .split(/[\n,]/)
        .map((s) => s.trim())
        .filter((s) => s !== '');
      out.push(...pieces);
    }
    return out;
  }

  function tokenizeInput(value) {
    if (!value) {
      return [];
    }
    return String(value)
      .split(/[\s,]+/)
      .map((s) => s.trim())
      .filter((s) => s !== '');
  }

  function buildParamsFromForm() {
    const params = new URLSearchParams();
    const elements = Array.from(form.elements);
    for (const el of elements) {
      if (!(el instanceof HTMLInputElement || el instanceof HTMLSelectElement || el instanceof HTMLTextAreaElement)) {
        continue;
      }
      if (!el.name || el.disabled) {
        continue;
      }
      const name = el.name;
      if (el.type === 'checkbox') {
        if (!el.checked) {
          continue;
        }
        params.append(name, el.value || '1');
        continue;
      }
      if (el instanceof HTMLInputElement && el.type === 'radio') {
        if (!el.checked) {
          continue;
        }
        params.append(name, el.value);
        continue;
      }
      if (el instanceof HTMLSelectElement && el.multiple) {
        for (const option of Array.from(el.selectedOptions)) {
          params.append(name, option.value);
        }
        continue;
      }
      if (el.dataset.multi != null) {
        const values = toCSVList(el.value || '');
        for (const value of values) {
          params.append(name, value);
        }
        continue;
      }
      if (el.dataset.tokenize != null) {
        const tokens = tokenizeInput(el.value || '');
        for (const token of tokens) {
          params.append(name, token);
        }
        continue;
      }
      const value = (el.value || '').trim();
      if (value !== '') {
        params.append(name, value);
      }
    }

    params.delete('string_scan');
    const scanValue = form.querySelector('input[name="string_scan"]:checked')?.value;
    if (scanValue === 'comments_only') {
      params.set('comments_only', '1');
      params.delete('include_strings');
      params.delete('no_strings');
    } else if (scanValue === 'include_strings') {
      params.set('include_strings', '1');
      params.delete('comments_only');
      params.delete('no_strings');
    } else if (scanValue === 'no_strings') {
      params.set('no_strings', '1');
      params.delete('include_strings');
      params.delete('comments_only');
    }

    const fields = params.getAll('fields');
    let autoEnabled = false;
    if (fields.some((f) => ['url', 'commit_url'].includes(f))) {
      params.set('with_commit_link', '1');
      const cb = form.elements.namedItem('with_commit_link');
      if (cb instanceof HTMLInputElement && cb.type === 'checkbox') {
        cb.checked = true;
      }
      autoEnabled = true;
    }
    if (fields.some((f) => ['pr', 'prs', 'pr_urls'].includes(f))) {
      params.set('with_pr_links', '1');
      const cb = form.elements.namedItem('with_pr_links');
      if (cb instanceof HTMLInputElement && cb.type === 'checkbox') {
        cb.checked = true;
      }
      autoEnabled = true;
    }
    if (fieldAutoNote) {
      fieldAutoNote.hidden = !autoEnabled;
    }
    togglePRVisibility(params.get('with_pr_links') === '1');

    const jobs = form.elements.namedItem('jobs');
    if (jobs instanceof HTMLInputElement && jobs.value.trim() === '') {
      params.delete('jobs');
    }

    return params;
  }

  function togglePRVisibility(enabled) {
    togglePRStepVisibility(enabled);
    if (prOptions) {
      prOptions.hidden = !enabled;
      prOptions.classList.toggle('is-hidden', !enabled);
    }
  }

  function updateShare(params) {
    const query = params.toString();
    if (window.history && typeof window.history.replaceState === 'function') {
      const url = `${window.location.pathname}?${query}`;
      window.history.replaceState(null, '', url);
    }
    const shareURL = new URL(window.location.href);
    shareURL.search = query;
    shareURL.hash = '';
    if (shareURLEl) {
      shareURLEl.value = shareURL.toString();
    }
  }
  function shellQuote(value) {
    if (value === '') {
      return "''";
    }
    if (/^[A-Za-z0-9_.,@\/-]+$/.test(value)) {
      return value;
    }
    return `'${String(value).replace(/'/g, "'\\''")}'`;
  }

  function paramsToCLI(params) {
    const args = ['todox'];
    const getAll = (key) => params.getAll(key);

    const type = params.get('type');
    if (type && type !== 'both') {
      args.push('--type', type);
    }
    const mode = params.get('mode');
    if (mode && mode !== 'last') {
      args.push('--mode', mode);
    }
    const author = params.get('author');
    if (author) {
      args.push('--author', author);
    }
    const detect = params.get('detect');
    if (detect && detect !== 'auto') {
      args.push('--detect', detect);
    }
    const detectLangs = getAll('detect_langs');
    if (detectLangs.length) {
      args.push('--detect-langs', detectLangs.join(','));
    }
    const tags = getAll('tags');
    if (tags.length) {
      args.push('--tags', tags.join(','));
    }
    if (params.get('comments_only') === '1') {
      args.push('--comments-only');
    } else if (params.get('include_strings') === '1') {
      args.push('--include-strings');
    } else if (params.get('no_strings') === '1') {
      args.push('--no-strings');
    }
    const maxFileBytes = params.get('max_file_bytes');
    if (maxFileBytes) {
      args.push('--max-file-bytes', maxFileBytes);
    }
    if (params.get('no_prefilter') === '1') {
      args.push('--no-prefilter');
    }
    const pathValues = getAll('path');
    for (const value of pathValues) {
      args.push('--path', value);
    }
    const excludeValues = getAll('exclude');
    for (const value of excludeValues) {
      args.push('--exclude', value);
    }
    const regexValues = getAll('path_regex');
    for (const value of regexValues) {
      args.push('--path-regex', value);
    }
    if (params.get('exclude_typical') === '1') {
      args.push('--exclude-typical');
    }
    const fields = getAll('fields');
    if (fields.length) {
      args.push('--fields', fields.join(','));
    }
    if (params.get('with_comment') === '1') {
      args.push('--with-comment');
    }
    if (params.get('with_message') === '1') {
      args.push('--with-message');
    }
    if (params.get('with_age') === '1') {
      args.push('--with-age');
    }
    if (params.get('with_commit_link') === '1') {
      args.push('--with-commit-link');
    }
    if (params.get('with_pr_links') === '1') {
      args.push('--with-pr-links');
      const prState = params.get('pr_state');
      if (prState) {
        args.push('--pr-state', prState);
      }
      const prLimit = params.get('pr_limit');
      if (prLimit) {
        args.push('--pr-limit', prLimit);
      }
      const prPrefer = params.get('pr_prefer');
      if (prPrefer) {
        args.push('--pr-prefer', prPrefer);
      }
    }
    const truncate = params.get('truncate');
    if (truncate) {
      args.push('--truncate', truncate);
    }
    const truncComment = params.get('truncate_comment');
    if (truncComment) {
      args.push('--truncate-comment', truncComment);
    }
    const truncMessage = params.get('truncate_message');
    if (truncMessage) {
      args.push('--truncate-message', truncMessage);
    }
    const sort = params.get('sort');
    if (sort) {
      args.push('--sort', sort);
    }
    const ignoreWS = params.get('ignore_ws');
    if (ignoreWS === '0') {
      args.push('--no-ignore-ws');
    }
    const jobs = params.get('jobs');
    if (jobs) {
      args.push('--jobs', jobs);
    }
    const repo = params.get('repo');
    if (repo) {
      args.push('--repo', repo);
    }

    const cli = args.map((arg, index) => (index === 0 ? arg : shellQuote(arg))).join(' ');
    if (cliOutput) {
      cliOutput.value = cli;
    }
    if (cliPreview) {
      cliPreview.textContent = cli;
    }
    if (cliHelp) {
      const hints = [];
      if (detect && detect !== 'auto') {
        hints.push('`--detect` 検出エンジン');
      }
      if (fields.length) {
        hints.push('`--fields` 表示列');
      }
      if (params.get('with_pr_links') === '1') {
        hints.push('`--with-pr-links` PR 情報');
      }
      if (params.get('with_commit_link') === '1') {
        hints.push('`--with-commit-link` コミットURL');
      }
      if (params.get('with_comment') === '1') {
        hints.push('`--with-comment` コメント出力');
      }
      if (params.get('with_message') === '1') {
        hints.push('`--with-message` コミットメッセージ');
      }
      if (params.get('with_age') === '1') {
        hints.push('`--with-age` 経過日数');
      }
      if (params.get('comments_only') === '1' || params.get('include_strings') === '1' || params.get('no_strings') === '1') {
        hints.push('コメント/文字列の走査モード');
      }
      cliHelp.textContent = hints.length ? hints.join(' / ') : '基本設定のみです。';
    }
  }

  function updateDerivedState() {
    const params = buildParamsFromForm();
    updateShare(params);
    paramsToCLI(params);
    if (window.localStorage) {
      try {
        window.localStorage.setItem(LOCAL_STORAGE_KEY, params.toString());
      } catch (_err) {}
    }
    return params;
  }

  function startScan(ev) {
    ev.preventDefault();
    hideError();
    const params = updateDerivedState();
    if ('EventSource' in window) {
      startScanWithSSE(params);
    } else {
      startScanWithFetch(params);
    }
  }

  form.addEventListener('submit', startScan);
  form.addEventListener('input', () => {
    updateDerivedState();
  });
  form.addEventListener('change', () => {
    updateDerivedState();
  });

  const sortQuick = document.getElementById('sort-quick');
  if (sortQuick) {
    sortQuick.addEventListener('change', () => {
      const value = sortQuick.value;
      const sortField = form.elements.namedItem('sort');
      if (sortField instanceof HTMLInputElement) {
        sortField.value = value || '';
        updateDerivedState();
      }
    });
  }

  const copyButtons = document.querySelectorAll('[data-copy-target]');
  copyButtons.forEach((btn) => {
    const targetSel = btn.getAttribute('data-copy-target');
    if (!targetSel) {
      return;
    }
    btn.addEventListener('click', async () => {
      const target = document.querySelector(targetSel);
      if (!(target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLElement)) {
        return;
      }
      try {
        let text = '';
        if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
          text = target.value;
        } else {
          text = target.textContent || '';
        }
        if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
          await navigator.clipboard.writeText(text);
        } else {
          if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
            target.select();
            target.setSelectionRange(0, text.length);
            document.execCommand('copy');
          } else {
            const range = document.createRange();
            range.selectNodeContents(target);
            const selection = window.getSelection();
            if (selection) {
              selection.removeAllRanges();
              selection.addRange(range);
              document.execCommand('copy');
              selection.removeAllRanges();
            }
          }
        }
        btn.classList.add('copied');
        window.setTimeout(() => btn.classList.remove('copied'), 1200);
      } catch (_err) {}
    });
  });

  if (resetBtn) {
    resetBtn.addEventListener('click', () => {
      form.reset();
      syncRangePairs();
      togglePRVisibility(false);
      updateDerivedState();
    });
  }

  function syncRangePairs() {
    const pairs = form.querySelectorAll('.range-pair');
    pairs.forEach((pair) => {
      const range = pair.querySelector('input[type="range"]');
      const number = pair.querySelector('input[type="number"]');
      if (!(range instanceof HTMLInputElement) || !(number instanceof HTMLInputElement)) {
        return;
      }
      const placeholder = number.getAttribute('placeholder');
      const currentValue = number.value !== '' ? number.value : (placeholder || '0');
      range.value = currentValue;
      if (pair.dataset.initialized === '1') {
        return;
      }
      range.addEventListener('input', () => {
        number.value = range.value;
        updateDerivedState();
      });
      number.addEventListener('input', () => {
        if (number.value === '') {
          range.value = placeholder || '0';
        } else {
          range.value = number.value;
        }
      });
      pair.dataset.initialized = '1';
    });
  }

  syncRangePairs();

  function parseBoolish(value) {
    const normalized = String(value).toLowerCase();
    return normalized === '1' || normalized === 'true' || normalized === 'yes' || normalized === 'on';
  }

  function applyParamsToForm(params) {
    form.reset();
    const entries = Array.from(params.entries());
    const grouped = new Map();
    for (const [key, value] of entries) {
      if (!grouped.has(key)) {
        grouped.set(key, []);
      }
      grouped.get(key).push(value);
    }

    const setTextareaValues = (name, values) => {
      const el = form.elements.namedItem(name);
      if (el instanceof HTMLTextAreaElement) {
        el.value = values.join('\n');
      }
    };

    const setSelectMultiple = (name, values) => {
      const el = form.elements.namedItem(name);
      if (el instanceof HTMLSelectElement && el.multiple) {
        for (const option of Array.from(el.options)) {
          option.selected = values.includes(option.value);
        }
      }
    };

    for (const [key, values] of grouped.entries()) {
      if (['path', 'exclude', 'path_regex'].includes(key)) {
        setTextareaValues(key, values);
      } else if (['detect_langs', 'tags'].includes(key)) {
        const el = form.elements.namedItem(key);
        if (el instanceof HTMLInputElement) {
          el.value = values.join(',');
        }
      } else if (key === 'fields') {
        setSelectMultiple('fields', values);
      } else if (['comments_only', 'include_strings', 'no_strings'].includes(key)) {
        const boolVal = parseBoolish(values[values.length - 1]);
        if (boolVal) {
          const radioValue = key;
          const target = form.querySelector(`input[name="string_scan"][value="${cssEscape(radioValue)}"]`);
          if (target instanceof HTMLInputElement) {
            target.checked = true;
          }
        }
      } else {
        const el = form.elements.namedItem(key);
        const latest = values[values.length - 1];
        if (el instanceof HTMLInputElement) {
          if (el.type === 'checkbox') {
            el.checked = parseBoolish(latest);
          } else if (el.type === 'radio') {
            const radio = form.querySelector(`input[name="${cssEscape(key)}"][value="${cssEscape(latest)}"]`);
            if (radio instanceof HTMLInputElement) {
              radio.checked = true;
            }
          } else {
            el.value = latest;
          }
        } else if (el instanceof HTMLSelectElement) {
          el.value = latest;
        } else if (el instanceof HTMLTextAreaElement) {
          el.value = latest;
        }
      }
    }

    const withPR = parseBoolish(params.get('with_pr_links') || '0');
    togglePRVisibility(withPR);
    syncRangePairs();
  }

  function loadInitialState() {
    const query = window.location.search ? window.location.search.slice(1) : '';
    const stored = window.localStorage ? window.localStorage.getItem(LOCAL_STORAGE_KEY) : '';
    let source = '';
    if (query) {
      source = query;
    } else if (stored) {
      source = stored;
    }
    if (source) {
      try {
        const params = new URLSearchParams(source);
        applyParamsToForm(params);
      } catch (_err) {}
    }
    updateDerivedState();
  }

  loadInitialState();

  function downloadBlob(filename, blob) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  function exportJSON() {
    if (!latestResult) {
      return;
    }
    const blob = new Blob([JSON.stringify(latestResult, null, 2)], { type: 'application/json' });
    downloadBlob('todox-results.json', blob);
  }

  function getHeaderMetaForExport() {
    return buildHeaderMeta(latestResult || {});
  }

  function valueForExport(key, row) {
    const r = row || {};
    switch (key) {
      case 'kind':
        return String(r.kind ?? '');
      case 'author':
        return String(r.author ?? '');
      case 'email':
        return String(r.email ?? '');
      case 'date':
        return String(r.date ?? '');
      case 'age_days':
        return (typeof r.age_days === 'number' && isFinite(r.age_days)) ? String(r.age_days) : '';
      case 'commit':
        return String(r.commit ?? '');
      case 'location':
        return `${r.file ?? ''}:${r.line ?? ''}`;
      case 'url':
        return String(r.url ?? '');
      case 'prs': {
        const list = Array.isArray(r.prs) ? r.prs : [];
        return list.map((pr) => {
          const info = pr || {};
          const num = info.number != null ? `#${info.number}` : '';
          const title = info.title ? ` ${info.title}` : '';
          const state = info.state ? ` (${info.state})` : '';
          return `${num}${title}${state}`.trim();
        }).join('; ');
      }
      case 'comment':
        return String(r.comment ?? '');
      case 'message':
        return String(r.message ?? '');
      default:
        return String(r[key] ?? '');
    }
  }

  function exportTSV() {
    if (!latestResult) {
      return;
    }
    const rows = Array.isArray(latestResult.items) ? latestResult.items : [];
    if (!rows.length) {
      return;
    }
    const headerMeta = getHeaderMetaForExport();
    const lines = [];
    lines.push(headerMeta.map((meta) => meta.label).join('\t'));
    for (const row of rows) {
      const cols = headerMeta.map((meta) => valueForExport(meta.key, row).replace(/\t/g, ' ').replace(/\r?\n/g, ' '));
      lines.push(cols.join('\t'));
    }
    const blob = new Blob([lines.join('\n')], { type: 'text/tab-separated-values' });
    downloadBlob('todox-results.tsv', blob);
  }

  if (exportJSONBtn) {
    exportJSONBtn.addEventListener('click', exportJSON);
  }
  if (exportTSVBtn) {
    exportTSVBtn.addEventListener('click', exportTSV);
  }

  function setField(name, value) {
    const el = form.elements.namedItem(name);
    if (!el) {
      return;
    }
    if (el instanceof HTMLInputElement) {
      if (el.type === 'checkbox') {
        el.checked = parseBoolish(value);
      } else {
        el.value = value;
      }
    } else if (el instanceof HTMLSelectElement) {
      el.value = value;
    } else if (el instanceof HTMLTextAreaElement) {
      el.value = value;
    }
  }

  function applyPreset(name) {
    switch (name) {
      case 'precise':
        setField('detect', 'parse');
        break;
      case 'fast':
        setField('detect', 'regex');
        ['with_comment', 'with_message', 'with_pr_links', 'with_commit_link'].forEach((key) => {
          const el = form.elements.namedItem(key);
          if (el instanceof HTMLInputElement && el.type === 'checkbox') {
            el.checked = false;
          }
        });
        break;
      case 'oldest':
        {
          const el = form.elements.namedItem('with_age');
          if (el instanceof HTMLInputElement && el.type === 'checkbox') {
            el.checked = true;
          }
          setField('sort', '-age');
        }
        break;
      case 'prs':
        {
          const prCheckbox = form.elements.namedItem('with_pr_links');
          if (prCheckbox instanceof HTMLInputElement && prCheckbox.type === 'checkbox') {
            prCheckbox.checked = true;
          }
          setField('pr_state', 'all');
          setField('pr_limit', '5');
          setField('pr_prefer', 'open');
          const fieldsSelect = form.elements.namedItem('fields');
          if (fieldsSelect instanceof HTMLSelectElement) {
            for (const option of Array.from(fieldsSelect.options)) {
              option.selected = false;
            }
            ['type', 'author', 'date', 'prs'].forEach((value) => {
              for (const option of Array.from(fieldsSelect.options)) {
                if (option.value === value) {
                  option.selected = true;
                }
              }
            });
          }
        }
        break;
      case 'concise':
        {
          const comment = form.elements.namedItem('with_comment');
          if (comment instanceof HTMLInputElement && comment.type === 'checkbox') {
            comment.checked = true;
          }
          const message = form.elements.namedItem('with_message');
          if (message instanceof HTMLInputElement && message.type === 'checkbox') {
            message.checked = true;
          }
          const truncate = form.elements.namedItem('truncate');
          if (truncate instanceof HTMLInputElement) {
            truncate.value = '80';
          }
        }
        break;
      case 'whitespace':
        setField('ignore_ws', '0');
        break;
      case 'baseline':
        form.reset();
        break;
      default:
        return;
    }
    syncRangePairs();
    updateDerivedState();
  }

  if (presetWrap) {
    presetWrap.addEventListener('click', (ev) => {
      const target = ev.target;
      if (!(target instanceof HTMLElement)) {
        return;
      }
      const preset = target.getAttribute('data-preset');
      if (!preset) {
        return;
      }
      applyPreset(preset);
    });
  }

})();
