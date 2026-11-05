import './style.css';
import {
    LoadComparison,
    LoadGitComparison,
    ListGitRefs,
    GetDiff,
    GetSideBySide,
    GetInitialPaths,
    SaveMerge,
    CheckTargetOnDisk,
    CopyFiles,
    SelectDirectory,
    SelectFile,
    SuggestPaths,
    GetToolInfo,
    GetSettings,
    GetVersion,
    OpenInEditor,
    OpenInDiffTool,
    OpenDirsInDiffTool,
    ResolveConflict,
    GetConflicts,
    SaveConflicts,
} from '../bindings/katcompare/katcmp-gui/app.js';
import { Events } from '@wailsio/runtime';

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

const LAYOUT_KEY = 'katcompare.layout';

// loadLayout returns the collapse choices saved by an earlier run. Storage can
// be missing or hold anything, so it only trusts real booleans.
function loadLayout() {
    let saved = {};
    try {
        saved = JSON.parse(localStorage.getItem(LAYOUT_KEY)) || {};
    } catch {
        // Fall back to everything showing.
    }
    return {
        filesCollapsed: saved.filesCollapsed === true,
        pathsCollapsed: saved.pathsCollapsed === true,
        focus: saved.focus === true,
    };
}

const state = {
    layout: loadLayout(), // { filesCollapsed, pathsCollapsed, focus }; focus hides both without changing them
    compareFailed: false, // the last Compare failed, so the paths bar stays open to fix it
    mode: 'paths', // 'paths' | 'git'
    comparison: null, // ComparisonResult from the backend
    filterQuery: '',
    selectedRelPath: null,
    diff: null, // DiffResult for the selected file
    diffView: 'unified', // 'unified' | 'sidebyside' | 'merge' | 'conflicts'
    mergeTarget: 'left', // 'left' | 'right'
    defaultView: 'unified', // the view to fall back to: default_view in the config file
    defaultMergeTarget: 'left', // default_merge_target in the config file
    conflicts: [], // ConflictBlockDTOs of the selected file (when conflicted)
    conflictChoices: [], // per block: '' | 'ours' | 'theirs' | 'both' | 'both-theirs-first'
    mergeSelection: { insertions: new Set(), deletions: new Set() },
    targetDisk: 'unchanged', // the merge target on disk vs as loaded: 'unchanged' | 'changed' | 'missing'
    justSavedRelPath: null, // file whose next load keeps its target and starts with nothing checked
    copyTarget: 'to-right', // 'to-right' | 'to-left'
    copySelection: new Set(),
    lastLoad: null, // { git: bool, left, right } — to reload after external changes
    shown: null, // { git, left, right } of the comparison currently displayed
    toolInfo: null, // ToolInfo from the backend
};

// ---------------------------------------------------------------------------
// DOM references
// ---------------------------------------------------------------------------

const el = (id) => document.getElementById(id);

const modeTabs = document.querySelectorAll('.mode-tab');
const pathsForm = el('paths-form');
const gitForm = el('git-form');

const leftPathInput = el('left-path');
const rightPathInput = el('right-path');
const leftRefInput = el('left-ref');
const rightRefInput = el('right-ref');
const leftRefSelect = el('left-ref-select');
const rightRefSelect = el('right-ref-select');

const filterInput = el('filter-input');
const fileListEl = el('file-list');
const copyFilesBtn = el('copy-files-btn');

const contentHeaderEl = el('content-header');
const contentBodyEl = el('content-body');
const mergeStatusEl = el('merge-status');

const statusbarEl = document.querySelector('.statusbar');
const statusMessageEl = el('status-message');

const copyModal = el('copy-modal');
const copyFileListEl = el('copy-file-list');
const copySummaryEl = el('copy-summary');
const copyTargetToggle = el('copy-target-toggle');

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function escapeHtml(str) {
    return String(str ?? '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function debounce(fn, wait) {
    let t;
    return (...args) => {
        clearTimeout(t);
        t = setTimeout(() => fn(...args), wait);
    };
}

function setStatus(message, type) {
    statusMessageEl.textContent = message || '';
    statusbarEl.classList.remove('error', 'success', 'warning');
    if (type) statusbarEl.classList.add(type);
}

function goErrorMessage(err) {
    if (!err) return 'Unknown error';
    if (typeof err === 'string') return err;
    if (err.message) return err.message;
    try {
        return JSON.stringify(err);
    } catch {
        return String(err);
    }
}

function hasUniqueFiles() {
    if (!state.comparison) return false;
    return state.comparison.files.some((f) => f.source !== 'both');
}

// ---------------------------------------------------------------------------
// Collapsing the file list and paths bar
// ---------------------------------------------------------------------------

const appEl = el('app');

// canCollapse reports whether there's a comparison to look at. Without one,
// everything shows, or a remembered collapse would hide the path inputs.
function canCollapse() {
    return (state.comparison?.files.length ?? 0) > 0;
}

function applyLayout() {
    const { filesCollapsed, pathsCollapsed, focus } = state.layout;
    const can = canCollapse();
    appEl.classList.toggle('focus-mode', can && focus);
    appEl.classList.toggle('files-collapsed', can && (focus || filesCollapsed));
    appEl.classList.toggle('paths-collapsed', can && !state.compareFailed && (focus || pathsCollapsed));
    el('collapse-files-btn').disabled = !can;
    el('collapse-paths-btn').disabled = !can;

    const shown = state.shown;
    if (shown) {
        const text = shown.git
            ? `git: ${shown.left || 'HEAD'} ↔ ${shown.right || 'working tree'}`
            : `${shown.left} ↔ ${shown.right}`;
        el('paths-slim-label').textContent = text;
        el('paths-slim-label').title = text;
    }
}

function setLayout(changes) {
    Object.assign(state.layout, changes);
    try {
        localStorage.setItem(LAYOUT_KEY, JSON.stringify(state.layout));
    } catch {
        // The choice just won't be remembered.
    }
    applyLayout();
    if (state.diff) renderContentHeader();
}

// In focus, showing one thing leaves the other as it looks now.
function toggleFiles() {
    if (!canCollapse()) return;
    const { filesCollapsed, focus } = state.layout;
    setLayout(focus ? { focus: false, filesCollapsed: false, pathsCollapsed: true } : { filesCollapsed: !filesCollapsed });
}

function togglePaths() {
    if (!canCollapse()) return;
    const { pathsCollapsed, focus } = state.layout;
    setLayout(focus ? { focus: false, filesCollapsed: true, pathsCollapsed: false } : { pathsCollapsed: !pathsCollapsed });
}

function toggleFocus() {
    if (canCollapse()) setLayout({ focus: !state.layout.focus });
}

el('collapse-files-btn').addEventListener('click', toggleFiles);
el('expand-files-btn').addEventListener('click', toggleFiles);
el('collapse-paths-btn').addEventListener('click', togglePaths);
el('edit-paths-btn').addEventListener('click', editPaths);

// editPaths shows the paths bar, ending focus, ready to type.
function editPaths() {
    setLayout({ focus: false, pathsCollapsed: false });
    (state.mode === 'git' ? leftRefInput : leftPathInput).focus();
}

// Closes the More menu when clicking anywhere else.
document.addEventListener('click', (e) => {
    const menu = document.querySelector('.more-menu[open]');
    if (menu && !menu.contains(e.target)) menu.open = false;
});

// ---------------------------------------------------------------------------
// Mode switching
// ---------------------------------------------------------------------------

modeTabs.forEach((btn) => {
    btn.addEventListener('click', () => {
        state.mode = btn.dataset.mode;
        modeTabs.forEach((b) => b.classList.toggle('active', b === btn));
        pathsForm.classList.toggle('hidden', state.mode !== 'paths');
        gitForm.classList.toggle('hidden', state.mode !== 'git');
        if (state.mode === 'git') refreshGitRefs();
    });
});

// ---------------------------------------------------------------------------
// Compare button: highlight it while the inputs differ from what's displayed
// ---------------------------------------------------------------------------

// pendingInputs returns the inputs the current mode would compare, or null
// when there's nothing to compare yet (empty paths).
function pendingInputs() {
    if (state.mode === 'git') {
        return { git: true, left: leftRefInput.value.trim() || 'HEAD', right: rightRefInput.value.trim() };
    }
    const left = leftPathInput.value.trim();
    const right = rightPathInput.value.trim();
    if (!left && !right) return null;
    return { git: false, left, right };
}

function updateComparePending() {
    const want = pendingInputs();
    const shown = state.shown;
    const pending = !!want && !(shown && shown.git === want.git && shown.left === want.left && shown.right === want.right);
    const btn = el(state.mode === 'git' ? 'git-compare-btn' : 'compare-btn');
    const other = el(state.mode === 'git' ? 'compare-btn' : 'git-compare-btn');
    other.classList.remove('btn-pending');
    btn.classList.toggle('btn-pending', pending);
    btn.title = pending ? 'Not shown yet — press Compare to load these inputs' : '';
}

// Values change from typing, dropdowns, swap buttons, suggestions… so
// re-check after any of those (deferred so handlers have run first).
['input', 'change', 'click', 'keyup'].forEach((type) => {
    document.addEventListener(type, () => setTimeout(updateComparePending, 0), true);
});

// ---------------------------------------------------------------------------
// Git ref dropdowns
// ---------------------------------------------------------------------------

// The placeholder option; the working tree uses the real value "".
const REF_CHOOSE = '__choose__';

// fillRefSelect rebuilds a ref dropdown from the detected refs, one
// <optgroup> per group. The working tree is only offered on the right.
function fillRefSelect(selectEl, refs, includeWorkingTree) {
    let html = `<option value="${REF_CHOOSE}" disabled>Choose…</option>`;
    let group = null;
    for (const r of refs) {
        if (r.ref === '' && !includeWorkingTree) continue;
        if (r.group !== group) {
            if (group !== null) html += '</optgroup>';
            group = r.group;
            html += `<optgroup label="${escapeHtml(group)}">`;
        }
        html += `<option value="${escapeHtml(r.ref)}">${escapeHtml(r.label)}</option>`;
    }
    if (group !== null) html += '</optgroup>';
    selectEl.innerHTML = html;
}

// syncRefSelect shows the input's ref in its dropdown, or the placeholder
// when the typed ref isn't one of the options.
function syncRefSelect(inputEl, selectEl) {
    const value = inputEl.value.trim();
    const match = [...selectEl.options].some((o) => o.value === value && !o.disabled);
    selectEl.value = match ? value : REF_CHOOSE;
}

async function refreshGitRefs() {
    let refs = [];
    try {
        refs = (await ListGitRefs()) || [];
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
    fillRefSelect(leftRefSelect, refs, false);
    fillRefSelect(rightRefSelect, refs, true);
    syncRefSelect(leftRefInput, leftRefSelect);
    syncRefSelect(rightRefInput, rightRefSelect);
}

[[leftRefInput, leftRefSelect], [rightRefInput, rightRefSelect]].forEach(([inputEl, selectEl]) => {
    selectEl.addEventListener('change', () => {
        inputEl.value = selectEl.value;
    });
    inputEl.addEventListener('input', () => syncRefSelect(inputEl, selectEl));
});

el('swap-refs').addEventListener('click', () => {
    [leftRefInput.value, rightRefInput.value] = [rightRefInput.value, leftRefInput.value];
    syncRefSelect(leftRefInput, leftRefSelect);
    syncRefSelect(rightRefInput, rightRefSelect);
});

// ---------------------------------------------------------------------------
// Path autocomplete
// ---------------------------------------------------------------------------

function setupAutocomplete(inputEl, suggestionsEl) {
    let items = [];
    let activeIndex = -1;

    const close = () => {
        items = [];
        activeIndex = -1;
        suggestionsEl.classList.remove('open');
        suggestionsEl.innerHTML = '';
    };

    const render = () => {
        if (items.length === 0) {
            close();
            return;
        }
        suggestionsEl.innerHTML = items
            .map(
                (s, i) =>
                    `<div class="suggestion-item${i === activeIndex ? ' active' : ''}" data-index="${i}">${escapeHtml(s)}</div>`
            )
            .join('');
        suggestionsEl.classList.add('open');
    };

    const accept = (value) => {
        inputEl.value = value;
        close();
        // If the accepted suggestion is a directory, immediately show what's inside it.
        if (value.endsWith('/') || value.endsWith('\\')) {
            fetchSuggestions(value);
        } else {
            inputEl.focus();
        }
    };

    const fetchSuggestions = debounce(async (value) => {
        if (!value) {
            close();
            return;
        }
        try {
            const result = await SuggestPaths(value);
            items = result || [];
            activeIndex = items.length > 0 ? 0 : -1;
            render();
        } catch {
            close();
        }
    }, 120);

    inputEl.addEventListener('input', () => fetchSuggestions(inputEl.value));

    inputEl.addEventListener('keydown', (e) => {
        if (items.length > 0 && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
            e.preventDefault();
            if (e.key === 'ArrowDown') activeIndex = (activeIndex + 1) % items.length;
            else activeIndex = (activeIndex - 1 + items.length) % items.length;
            render();
            return;
        }
        if (e.key === 'Tab' && items.length > 0) {
            e.preventDefault();
            activeIndex = (activeIndex + 1) % items.length;
            render();
            return;
        }
        if (e.key === 'Escape') {
            close();
            return;
        }
        if (e.key === 'Enter') {
            if (items.length > 0 && activeIndex >= 0 && items[activeIndex] !== inputEl.value) {
                e.preventDefault();
                accept(items[activeIndex]);
            }
        }
    });

    suggestionsEl.addEventListener('mousedown', (e) => {
        const row = e.target.closest('.suggestion-item');
        if (!row) return;
        e.preventDefault();
        accept(items[Number(row.dataset.index)]);
    });

    inputEl.addEventListener('blur', () => {
        setTimeout(close, 150);
    });
}

setupAutocomplete(leftPathInput, el('left-suggestions'));
setupAutocomplete(rightPathInput, el('right-suggestions'));

// ---------------------------------------------------------------------------
// Browse buttons
// ---------------------------------------------------------------------------

el('left-browse-file').addEventListener('click', async () => {
    const path = await SelectFile(leftPathInput.value);
    if (path) leftPathInput.value = path;
    updateComparePending();
});
el('left-browse-dir').addEventListener('click', async () => {
    const path = await SelectDirectory(leftPathInput.value);
    if (path) leftPathInput.value = path;
    updateComparePending();
});
el('right-browse-file').addEventListener('click', async () => {
    const path = await SelectFile(rightPathInput.value);
    if (path) rightPathInput.value = path;
    updateComparePending();
});
el('right-browse-dir').addEventListener('click', async () => {
    const path = await SelectDirectory(rightPathInput.value);
    if (path) rightPathInput.value = path;
    updateComparePending();
});

el('swap-paths').addEventListener('click', () => {
    const tmp = leftPathInput.value;
    leftPathInput.value = rightPathInput.value;
    rightPathInput.value = tmp;
});

// ---------------------------------------------------------------------------
// Compare actions
// ---------------------------------------------------------------------------

// applySettings applies the settings that can change while the app runs.
function applySettings(settings) {
    document.documentElement.style.setProperty('--diff-font-size', `${settings.diffFontSize}px`);
    state.defaultView = settings.defaultView;
    state.defaultMergeTarget = settings.defaultMergeTarget;
}

// afterUserCompare applies the settings that act on a Compare the user ran, and
// says so if the config file couldn't be used.
async function afterUserCompare() {
    try {
        const settings = await GetSettings();
        applySettings(settings);
        if (settings.autoCollapsePaths && canCollapse()) setLayout({ pathsCollapsed: true });
        if (settings.configError) {
            setStatus(`Using default settings. Problem in ${settings.configPath}: ${settings.configError}`, 'warning');
        }
    } catch {
        // Leave things as they are.
    }
}

// runComparison shows a comparison's result. byUser is false for a reload, such
// as after saving a merge, which mustn't collapse a bar someone just opened.
async function runComparison(promise, byUser) {
    setStatus('Comparing…');
    try {
        const result = await promise;
        state.shown = { ...state.lastLoad };
        state.compareFailed = false;
        applyComparisonResult(result);
        const hidden = result.hiddenIdentical ? `, ${result.hiddenIdentical} identical hidden` : '';
        setStatus(`Loaded ${result.files.length} file(s)${hidden}.`);
        if (byUser) await afterUserCompare();
    } catch (err) {
        state.compareFailed = true;
        applyLayout();
        setStatus(goErrorMessage(err), 'error');
    }
    updateComparePending();
}

function loadPaths(left, right, byUser = false) {
    state.lastLoad = { git: false, left, right };
    return runComparison(LoadComparison(left, right), byUser);
}

function loadGit(leftRef, rightRef, byUser = false) {
    state.lastLoad = { git: true, left: leftRef, right: rightRef };
    return runComparison(LoadGitComparison(leftRef, rightRef), byUser);
}

// reloadComparison re-runs the last comparison, e.g. after a file was edited
// or a conflict resolved in an external tool.
// statusAfter, if given, replaces the "Loaded N file(s)" status once done.
async function reloadComparison(statusAfter) {
    const last = state.lastLoad;
    if (!last) return;
    if (last.git) await loadGit(last.left, last.right);
    else await loadPaths(last.left, last.right);
    if (statusAfter && !statusbarEl.classList.contains('error')) setStatus(statusAfter, 'success');
}

el('compare-btn').addEventListener('click', () => {
    const left = leftPathInput.value.trim();
    const right = rightPathInput.value.trim();
    if (!left || !right) {
        setStatus('Enter both a left and right path first.', 'error');
        return;
    }
    loadPaths(left, right, true);
});

el('git-compare-btn').addEventListener('click', () => {
    const leftRef = leftRefInput.value.trim() || 'HEAD';
    const rightRef = rightRefInput.value.trim();
    loadGit(leftRef, rightRef, true);
});

[leftPathInput, rightPathInput].forEach((input) => {
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') el('compare-btn').click();
    });
});
[leftRefInput, rightRefInput].forEach((input) => {
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') el('git-compare-btn').click();
    });
});

function applyComparisonResult(result) {
    state.comparison = result;
    state.filterQuery = '';
    filterInput.value = '';

    const sorted = [...result.files].sort((a, b) => a.relPath.localeCompare(b.relPath));
    const stillExists = sorted.some((f) => f.relPath === state.selectedRelPath);
    state.selectedRelPath = stillExists ? state.selectedRelPath : sorted.length > 0 ? sorted[0].relPath : null;

    copyFilesBtn.disabled = result.gitMode || !hasUniqueFiles();

    applyLayout();
    renderFileList();
    if (state.selectedRelPath) {
        selectFile(state.selectedRelPath);
    } else {
        state.diff = null;
        renderContentHeader();
        renderContentBody();
    }
}

// ---------------------------------------------------------------------------
// File list
// ---------------------------------------------------------------------------

filterInput.addEventListener('input', () => {
    state.filterQuery = filterInput.value.trim().toLowerCase();
    renderFileList();
});

function iconFor(entry) {
    if (entry.conflicted) return '<span class="icon icon-conflict">⚠</span>';
    if (entry.mergeStatus?.state === 'safe') return '<span class="icon icon-merged-safe">◆</span>';
    if (entry.mergeStatus?.state === 'check') return '<span class="icon icon-merged-check">▲</span>';
    switch (entry.source) {
        case 'both':
            return entry.identical
                ? '<span class="icon icon-identical">✓</span>'
                : '<span class="icon icon-different">✗</span>';
        case 'left':
            return '<span class="icon icon-left">◄</span>';
        case 'right':
            return '<span class="icon icon-right">►</span>';
        default:
            return '';
    }
}

function badgeFor(entry) {
    if (entry.conflicted) return '<span class="badge badge-conflict">conflict</span>';
    if (entry.mergeStatus?.state === 'safe') return '<span class="badge badge-merged-safe">merged · safe</span>';
    if (entry.mergeStatus?.state === 'check') return '<span class="badge badge-merged-check">merged · check</span>';
    if (entry.source === 'left') return '<span class="badge badge-left">left only</span>';
    if (entry.source === 'right') return '<span class="badge badge-right">right only</span>';
    return '';
}

// visibleFiles returns the files the list shows, in order: those matching the
// filter, if there is one.
function visibleFiles() {
    return [...(state.comparison?.files ?? [])]
        .filter((f) => {
            if (!state.filterQuery) return true;
            return (
                f.displayName.toLowerCase().includes(state.filterQuery) ||
                f.relPath.toLowerCase().includes(state.filterQuery)
            );
        })
        .sort((a, b) => a.relPath.localeCompare(b.relPath));
}

// stepFile selects the next (dir > 0) or previous (dir < 0) file, wrapping
// around at the ends.
function stepFile(dir) {
    const files = visibleFiles();
    if (files.length === 0) return;
    const at = files.findIndex((f) => f.relPath === state.selectedRelPath);
    const next = at < 0 ? 0 : (at + dir + files.length) % files.length;
    selectFile(files[next].relPath);
}

function renderFileList() {
    if (state.comparison?.hiddenIdentical && state.comparison.files.length === 0) {
        fileListEl.innerHTML = `<div class="empty-state">All ${state.comparison.hiddenIdentical} file(s) are identical.</div>`;
        return;
    }
    if (!state.comparison || state.comparison.files.length === 0) {
        fileListEl.innerHTML = '<div class="empty-state">Load a comparison to see files here.</div>';
        return;
    }

    const files = visibleFiles();

    if (files.length === 0) {
        fileListEl.innerHTML = '<div class="empty-state">No files match your filter.</div>';
        return;
    }

    fileListEl.innerHTML = files
        .map(
            (f) => `
        <div class="file-row${f.relPath === state.selectedRelPath ? ' selected' : ''}" data-relpath="${escapeHtml(f.relPath)}">
            ${iconFor(f)}
            <span class="name">${escapeHtml(f.displayName)}</span>
            ${badgeFor(f)}
        </div>`
        )
        .join('');

    fileListEl.querySelectorAll('.file-row').forEach((row) => {
        row.addEventListener('click', () => selectFile(row.dataset.relpath));
    });
}

// ---------------------------------------------------------------------------
// Diff loading & rendering
// ---------------------------------------------------------------------------

// selectFileSeq identifies the latest file picked, so older answers are ignored.
let selectFileSeq = 0;

async function selectFile(relPath) {
    state.selectedRelPath = relPath;
    // Read this before the first await: the save handler clears it once its
    // reload returns, which doesn't wait for this function.
    const justSaved = state.justSavedRelPath === relPath;
    state.justSavedRelPath = null;
    renderFileList();
    const seq = ++selectFileSeq;

    try {
        const diff = await GetDiff(relPath);
        // Another file was picked while this one loaded, so this answer is old.
        if (seq !== selectFileSeq) return;
        const sameFile = state.diff?.relPath === relPath;
        state.diff = diff;
        state.targetDisk = 'unchanged';
        const keepTarget = justSaved && (state.mergeTarget === 'left' ? diff.leftEditable : diff.rightEditable);
        if (!keepTarget) {
            // The default side, unless it's not a file and the other one is.
            const other = state.defaultMergeTarget === 'left' ? 'right' : 'left';
            const editable = { left: diff.leftEditable, right: diff.rightEditable };
            state.mergeTarget = editable[state.defaultMergeTarget] || !editable[other] ? state.defaultMergeTarget : other;
        }
        // After a save nothing is pending, so nothing starts checked.
        if (justSaved) state.mergeSelection = { insertions: new Set(), deletions: new Set() };
        else resetMergeSelection();
        state.conflicts = [];
        state.conflictChoices = [];
        if (diff.conflicted) {
            state.conflicts = (await GetConflicts(relPath)) || [];
            state.conflictChoices = state.conflicts.map(() => '');
            if (!sameFile) state.diffView = 'conflicts';
        } else if (state.diffView === 'conflicts') {
            state.diffView = state.defaultView;
        }
        if (state.diffView === 'merge' && !diff.canMerge) {
            state.diffView = state.defaultView;
        }
        renderContentHeader();
        renderContentBody();
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
}

// reloadAfterSave reloads the comparison after a save, then reports msg as a
// warning if the file's merge status says it needs checking.
async function reloadAfterSave(relPath, msg) {
    state.justSavedRelPath = relPath;
    await reloadComparison();
    state.justSavedRelPath = null;
    if (statusbarEl.classList.contains('error')) return;
    const entry = state.comparison?.files.find((f) => f.relPath === relPath);
    setStatus(msg, entry?.mergeStatus?.state === 'check' ? 'warning' : 'success');
}

// renderMergeStatus fills the banner above the diff with the selected file's
// last save this session, or empties it.
function renderMergeStatus(diff) {
    const s = diff?.mergeStatus;
    if (!s) {
        mergeStatusEl.innerHTML = '';
        return;
    }
    const safe = s.state === 'safe';
    const details = [];
    if (safe) {
        if (s.kind === 'merge') details.push(`${s.applied} applied, ${s.skipped} skipped`);
        else details.push('no conflicts left');
    } else {
        details.push(...(s.issues || []));
    }
    details.push(`${s.backup} kept`);
    if (safe && diff.conflicted) details.push('mark resolved to stage it');
    mergeStatusEl.innerHTML = `
        <div class="merge-status-banner ${safe ? 'safe' : 'check'}">
            <span class="merge-status-title">${safe ? '◆ Merged safely to' : '▲ Check this merge to'} ${escapeHtml(s.target)}</span>
            <span class="merge-status-detail">${details.map(escapeHtml).join(' · ')}</span>
        </div>`;
}

function resetMergeSelection() {
    const insertions = new Set();
    const deletions = new Set();
    if (state.diff) {
        state.diff.lines.forEach((line, i) => {
            if (line.type === 'insert') insertions.add(i);
            if (line.type === 'delete') deletions.add(i);
        });
    }
    state.mergeSelection = { insertions, deletions };
}

function renderContentHeader() {
    const diff = state.diff;
    renderMergeStatus(diff);
    if (!diff) {
        contentHeaderEl.innerHTML = state.comparison?.hiddenIdentical
            ? '<div class="empty-state">Nothing differs. To list identical files, turn off hide_identical in the config file.</div>'
            : '<div class="empty-state">Nothing loaded yet — enter two paths (or a git ref) above and press Compare.</div>';
        return;
    }

    const focus = appEl.classList.contains('focus-mode');
    let fileNav = '';
    if (appEl.classList.contains('files-collapsed')) {
        const files = visibleFiles();
        const at = files.findIndex((f) => f.relPath === diff.relPath);
        fileNav = `
        <div class="segmented file-nav">
            <button class="segmented-btn" id="prev-file-btn" title="Previous file (p)" aria-label="Previous file">‹</button>
            <span class="file-pos">${at < 0 ? '–' : at + 1}/${files.length}</span>
            <button class="segmented-btn" id="next-file-btn" title="Next file (n)" aria-label="Next file">›</button>
        </div>`;
    }
    const externalActions = focus
        ? `<details class="more-menu">
                <summary class="btn btn-ghost btn-small" title="More file actions">More ▾</summary>
                <div class="more-menu-panel">
                    ${externalActionsHtml(diff)}
                    <div class="external-actions">
                        <button class="btn btn-ghost btn-small" id="more-edit-paths-btn" title="Show the paths bar">Edit paths</button>
                        <button class="btn btn-ghost btn-small" id="more-help-btn" title="Help (?)">Help</button>
                    </div>
                </div>
            </details>`
        : externalActionsHtml(diff);

    contentHeaderEl.innerHTML = `
        ${fileNav}
        <div class="content-title" title="${escapeHtml(diff.leftLabel)} ↔ ${escapeHtml(diff.rightLabel)}">
            ${escapeHtml(diff.displayName)}
            <span class="path-sub">${escapeHtml(diff.leftLabel)} ↔ ${escapeHtml(diff.rightLabel)}</span>
        </div>
        <div class="content-stats">
            <span class="stat-equal">${diff.equal} equal</span>
            <span class="stat-insert">+${diff.inserted}</span>
            <span class="stat-delete">-${diff.deleted}</span>
            ${diff.ignoring ? `<span class="stat-ignoring" title="Set in the config file. These differences are hidden, not gone">ignoring ${escapeHtml(diff.ignoring)}</span>` : ''}
        </div>
        <div class="content-actions">
            ${externalActions}
            <div class="segmented" id="view-toggle">
                ${diff.conflicted ? `<button class="segmented-btn${state.diffView === 'conflicts' ? ' active' : ''}" data-view="conflicts">Conflicts</button>` : ''}
                <button class="segmented-btn${state.diffView === 'unified' ? ' active' : ''}" data-view="unified">Unified</button>
                <button class="segmented-btn${state.diffView === 'sidebyside' ? ' active' : ''}" data-view="sidebyside">Side-by-side</button>
                <button class="segmented-btn${state.diffView === 'merge' ? ' active' : ''}" data-view="merge" ${diff.canMerge ? '' : 'disabled title="Only available for files that exist on both sides"'}>Merge</button>
            </div>
            <div class="segmented" id="change-nav">
                <button class="segmented-btn" id="prev-change-btn" title="Previous change ([)">${focus ? '↑' : '↑ Prev'}</button>
                <button class="segmented-btn" id="next-change-btn" title="Next change (])">${focus ? '↓' : '↓ Next'}</button>
            </div>
            <button class="btn btn-ghost btn-small" id="focus-btn" title="${focus ? 'Bring back the file list and paths bar (f)' : 'Hide the file list and paths bar for more room (f)'}">${focus ? 'Exit focus' : 'Focus'}</button>
        </div>
    `;

    el('focus-btn').addEventListener('click', toggleFocus);
    el('prev-file-btn')?.addEventListener('click', () => stepFile(-1));
    el('next-file-btn')?.addEventListener('click', () => stepFile(1));
    el('more-edit-paths-btn')?.addEventListener('click', editPaths);
    el('more-help-btn')?.addEventListener('click', openHelpModal);
    const menu = contentHeaderEl.querySelector('.more-menu');
    menu?.querySelector('.more-menu-panel').addEventListener('click', () => {
        menu.open = false;
    });

    document.querySelectorAll('#view-toggle .segmented-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
            if (btn.disabled) return;
            state.diffView = btn.dataset.view;
            renderContentHeader();
            renderContentBody();
        });
    });

    contentHeaderEl.querySelectorAll('[data-open-side]').forEach((btn) => {
        btn.addEventListener('click', () => openInEditor(btn.dataset.openSide, 0));
    });
    el('prev-change-btn').addEventListener('click', () => jumpToChange(-1));
    el('next-change-btn').addEventListener('click', () => jumpToChange(1));
    el('external-diff-btn')?.addEventListener('click', openInDiffTool);
    el('dir-diff-btn')?.addEventListener('click', openDirsInDiffTool);
    el('resolve-conflict-btn')?.addEventListener('click', resolveConflict);
}

// ---------------------------------------------------------------------------
// External tools (editor, diff tool, merge tool)
// ---------------------------------------------------------------------------

function externalActionsHtml(diff) {
    const info = state.toolInfo || {};
    const noDiffTool = 'No diff tool found — run: git config --global diff.tool meld';
    const buttons = [];
    if (state.comparison?.gitMode) {
        if (diff.rightEditable) {
            buttons.push(`<button class="btn btn-ghost btn-small" data-open-side="right" title="Open the working-tree file in your editor (e)">Open working copy</button>`);
        }
    } else {
        if (diff.leftEditable) {
            buttons.push(`<button class="btn btn-ghost btn-small" data-open-side="left" title="Open the left file in your editor">Open left</button>`);
        }
        if (diff.rightEditable) {
            buttons.push(`<button class="btn btn-ghost btn-small" data-open-side="right" title="Open the right file in your editor (e)">Open right</button>`);
        }
    }
    buttons.push(`<button class="btn btn-ghost btn-small" id="external-diff-btn" title="${info.diffToolConfigured ? 'Open this file pair in your diff tool (d)' : noDiffTool}">External diff</button>`);
    if (state.comparison?.dirs && info.canDirDiff) {
        buttons.push(`<button class="btn btn-ghost btn-small" id="dir-diff-btn" title="Open both folders in your diff tool">Folders in diff tool</button>`);
    }
    if (diff.conflicted) {
        buttons.push(`<button class="btn btn-ghost btn-small" id="resolve-conflict-btn" title="Resolve in your external merge tool (git mergetool)">Merge tool</button>`);
    }
    return `<div class="external-actions">${buttons.join('')}</div>`;
}

// defaultEditSide is the side opened by the `e` shortcut: the right (newer /
// working) side when it's a real file, else the left.
function defaultEditSide(diff) {
    if (diff.rightEditable) return 'right';
    if (diff.leftEditable) return 'left';
    return null;
}

async function openInEditor(side, line) {
    const diff = state.diff;
    if (!diff || !side) return;
    try {
        await OpenInEditor(diff.relPath, side, line || 0);
        setStatus(`Opened ${side} side in editor${line ? ` at line ${line}` : ''}.`, 'success');
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
}

async function openInDiffTool() {
    const diff = state.diff;
    if (!diff) return;
    try {
        await OpenInDiffTool(diff.relPath);
        setStatus('Opened in external diff tool.', 'success');
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
}

async function openDirsInDiffTool() {
    try {
        await OpenDirsInDiffTool();
        setStatus('Opened both folders in external diff tool.', 'success');
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
}

async function resolveConflict() {
    const diff = state.diff;
    if (!diff) return;
    try {
        await ResolveConflict(diff.relPath);
        setStatus(`Resolving ${diff.displayName} in merge tool… the list reloads when it closes.`);
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
}

Events.On('conflict-resolved', (ev) => {
    const [relPath, errMsg] = ev.data;
    if (errMsg) {
        reloadComparison().then(() => setStatus(`Merge tool exited: ${errMsg}`, 'error'));
    } else {
        reloadComparison(`Merge tool finished for ${relPath}.`);
    }
});

// Clicking a line number opens that side in the editor at that line.
contentBodyEl.addEventListener('click', (e) => {
    const ln = e.target.closest('.ln[data-line]');
    if (!ln || !state.diff) return;
    const side = ln.dataset.side;
    if ((side === 'left' && !state.diff.leftEditable) || (side === 'right' && !state.diff.rightEditable)) return;
    openInEditor(side, Number(ln.dataset.line));
});

// lnAttrs returns attributes that make a line number clickable when that
// side can be opened in the editor.
function lnAttrs(side, lineNum) {
    const diff = state.diff;
    if (!diff || lineNum <= 0) return '';
    const editable = side === 'left' ? diff.leftEditable : diff.rightEditable;
    if (!editable) return '';
    return ` data-side="${side}" data-line="${lineNum}" title="Open in editor at line ${lineNum}"`;
}

function renderContentBody() {
    const diff = state.diff;
    if (!diff) {
        contentBodyEl.innerHTML = '';
        return;
    }
    if (state.diffView === 'unified') renderUnified(diff);
    else if (state.diffView === 'sidebyside') renderSideBySide(diff);
    else if (state.diffView === 'merge') renderMerge(diff);
    else if (state.diffView === 'conflicts') renderConflicts(diff);
}

function markerFor(type) {
    if (type === 'insert') return '+';
    if (type === 'delete') return '-';
    return '';
}

// ---------------------------------------------------------------------------
// Change navigation (next / previous block of changed lines)
// ---------------------------------------------------------------------------

// hunkStart returns the class marking rows[i] as the first row of a block of
// changes (rows are diff lines or side-by-side rows; both have a type).
function hunkStart(rows, i) {
    const changed = (r) => r && r.type !== 'equal';
    return changed(rows[i]) && !changed(rows[i - 1]) ? ' hunk-start' : '';
}

// updateStickyToolbar pins the Merge or Conflicts toolbar while the lines scroll
// beneath it, unless the window is so small that it would hide most of them.
function updateStickyToolbar() {
    const bar = contentBodyEl.querySelector('.merge-toolbar');
    bar?.classList.toggle('scrolls-away', bar.offsetHeight > contentBodyEl.clientHeight / 2);
}

new ResizeObserver(updateStickyToolbar).observe(contentBodyEl);

// stickyToolbarHeight is how much of the content area the toolbar covers, so
// that scrolling to something can land below it.
function stickyToolbarHeight() {
    const bar = contentBodyEl.querySelector('.merge-toolbar');
    return bar && !bar.classList.contains('scrolls-away') ? bar.offsetHeight : 0;
}

// jumpToChange scrolls the next (dir > 0) or previous (dir < 0) block of
// changes to a quarter of the way down the diff and highlights it. It steps
// from the highlighted block while that is on screen, otherwise from the
// scroll position, so it also works after scrolling by hand.
function jumpToChange(dir) {
    if (!state.diff) return;
    const starts = [...contentBodyEl.querySelectorAll('.hunk-start')];
    if (starts.length === 0) {
        setStatus('No changes in this file.');
        return;
    }
    const bodyRect = contentBodyEl.getBoundingClientRect();
    const posOf = (row) => row.getBoundingClientRect().top - bodyRect.top + contentBodyEl.scrollTop;
    const anchor = Math.max(Math.round(contentBodyEl.clientHeight / 4), stickyToolbarHeight() + 8);

    let idx;
    const current = contentBodyEl.querySelector('.hunk-start.hunk-current');
    const currentRect = current?.getBoundingClientRect();
    if (currentRect && currentRect.bottom > bodyRect.top && currentRect.top < bodyRect.bottom) {
        idx = starts.indexOf(current) + dir;
    } else {
        const y = contentBodyEl.scrollTop + anchor;
        idx = dir > 0 ? starts.findIndex((r) => posOf(r) > y + 1) : starts.findLastIndex((r) => posOf(r) < y - 1);
    }
    if (idx < 0 || idx >= starts.length) {
        setStatus(dir > 0 ? 'No more changes below.' : 'No more changes above.');
        return;
    }
    const target = starts[idx];
    current?.classList.remove('hunk-current');
    target.classList.add('hunk-current');
    contentBodyEl.scrollTop = posOf(target) - anchor;
    setStatus(`Change ${idx + 1} of ${starts.length}`);
}

function renderUnified(diff) {
    contentBodyEl.innerHTML = diff.lines
        .map((line, i) => {
            const leftNum = line.leftLineNum > 0 ? line.leftLineNum : '';
            const rightNum = line.rightLineNum > 0 ? line.rightLineNum : '';
            return `<div class="diff-line ${line.type}${hunkStart(diff.lines, i)}">
                <span class="ln"${lnAttrs('left', line.leftLineNum)}>${leftNum}</span>
                <span class="ln ln-right"${lnAttrs('right', line.rightLineNum)}>${rightNum}</span>
                <span class="marker">${markerFor(line.type)}</span>
                <span class="content">${escapeHtml(line.content)}</span>
            </div>`;
        })
        .join('');
}

function sbsCell(content, lineNum, cls, side) {
    if (cls === 'empty') {
        return `<div class="sbs-cell empty"></div>`;
    }
    return `<div class="sbs-cell ${cls}">
        <div class="sbs-cell-inner">
            <span class="ln"${lnAttrs(side, lineNum)}>${lineNum > 0 ? lineNum : ''}</span>
            <span class="content">${escapeHtml(content)}</span>
        </div>
    </div>`;
}

function renderSideBySide(diff) {
    // Rows come separately, so that only the view in use is sent to the page.
    if (!diff.sideBySide) {
        contentBodyEl.innerHTML = '<div class="empty-state">Loading…</div>';
        GetSideBySide(diff.relPath)
            .then((rows) => {
                diff.sideBySide = rows || [];
                if (state.diff === diff && state.diffView === 'sidebyside') renderSideBySide(diff);
            })
            .catch((err) => {
                if (state.diff === diff) setStatus(goErrorMessage(err), 'error');
            });
        return;
    }
    contentBodyEl.innerHTML = `<div class="sbs-table">${diff.sideBySide
        .map((row, i) => {
            let leftCls = '';
            let rightCls = '';
            switch (row.type) {
                case 'insert':
                    leftCls = 'empty';
                    rightCls = 'insert';
                    break;
                case 'delete':
                    leftCls = 'delete';
                    rightCls = 'empty';
                    break;
                case 'modified':
                    leftCls = 'modified left';
                    rightCls = 'modified right';
                    break;
                default:
                    leftCls = '';
                    rightCls = '';
            }
            return `<div class="sbs-row${hunkStart(diff.sideBySide, i)}">
                ${sbsCell(row.leftContent, row.leftLineNum, leftCls, 'left')}
                ${sbsCell(row.rightContent, row.rightLineNum, rightCls, 'right')}
            </div>`;
        })
        .join('')}</div>`;
}

// ---------------------------------------------------------------------------
// Conflicts view (resolve git conflict markers in place)
// ---------------------------------------------------------------------------

const CHOICE_LABELS = {
    ours: 'Ours',
    theirs: 'Theirs',
    both: 'Both (ours first)',
    'both-theirs-first': 'Both (theirs first)',
};

function choiceLines(block, choice) {
    switch (choice) {
        case 'ours':
            return block.ours || [];
        case 'theirs':
            return block.theirs || [];
        case 'both':
            return [...(block.ours || []), ...(block.theirs || [])];
        case 'both-theirs-first':
            return [...(block.theirs || []), ...(block.ours || [])];
    }
    return [];
}

function codeLines(lines, cls) {
    if (!lines || lines.length === 0) return `<div class="conflict-code empty ${cls}">(no lines)</div>`;
    return `<div class="conflict-code ${cls}">${lines.map((l) => `<div>${escapeHtml(l) || '&nbsp;'}</div>`).join('')}</div>`;
}

function renderConflicts(diff) {
    const blocks = state.conflicts;
    const choices = state.conflictChoices;
    const resolved = choices.filter((c) => c).length;
    const total = blocks.length;

    if (total === 0) {
        contentBodyEl.innerHTML = `
            <div class="conflict-done">
                <p>No conflict markers left in <strong>${escapeHtml(diff.displayName)}</strong>, but git still lists it as conflicted.</p>
                <button class="btn btn-primary btn-small" id="conflict-mark">Mark resolved (git add)</button>
            </div>`;
        el('conflict-mark').addEventListener('click', () => saveConflicts(true));
        return;
    }

    const cards = blocks
        .map((b, i) => {
            const choice = choices[i];
            const head = `<div class="conflict-card-head">
                <span class="conflict-title">Conflict ${i + 1} of ${total}</span>
                <span class="conflict-line">line ${b.startLine}</span>
                ${choice ? `<span class="badge badge-resolved">${CHOICE_LABELS[choice]}</span>` : '<span class="badge badge-conflict">unresolved</span>'}
                <span class="spacer"></span>
                ${Object.entries(CHOICE_LABELS)
                    .map(
                        ([key, label]) =>
                            `<button class="btn btn-small ${choice === key ? 'btn-primary' : 'btn-ghost'}" data-block="${i}" data-choice="${key}">${label}</button>`
                    )
                    .join('')}
                ${choice ? `<button class="btn btn-ghost btn-small" data-block="${i}" data-choice="">Undo</button>` : ''}
            </div>`;
            const body = choice
                ? `<div class="conflict-side result">
                        <div class="conflict-side-label">Result</div>
                        ${codeLines(choiceLines(b, choice), 'result')}
                   </div>`
                : `<div class="conflict-sides">
                        <div class="conflict-side">
                            <div class="conflict-side-label ours">Ours <span>${escapeHtml(b.oursLabel)}</span></div>
                            ${codeLines(b.ours, 'ours')}
                        </div>
                        <div class="conflict-side">
                            <div class="conflict-side-label theirs">Theirs <span>${escapeHtml(b.theirsLabel)}</span></div>
                            ${codeLines(b.theirs, 'theirs')}
                        </div>
                   </div>
                   ${b.base ? `<details class="conflict-base"><summary>Common ancestor ${escapeHtml(b.baseLabel)}</summary>${codeLines(b.base, 'base')}</details>` : ''}`;
            return `<div class="conflict-card hunk-start${choice ? ' resolved' : ''}" data-index="${i}">
                ${head}
                ${b.before?.length ? codeLines(b.before, 'context') : ''}
                ${body}
                ${b.after?.length ? codeLines(b.after, 'context') : ''}
            </div>`;
        })
        .join('');

    contentBodyEl.innerHTML = `
        <div class="merge-toolbar">
            <div class="stats"><span>${resolved} of ${total} resolved</span></div>
            <button class="btn btn-ghost btn-small" id="conflict-all-ours">All ours</button>
            <button class="btn btn-ghost btn-small" id="conflict-all-theirs">All theirs</button>
            <button class="btn btn-ghost btn-small" id="conflict-reset" ${resolved ? '' : 'disabled'}>Reset</button>
            <div class="spacer"></div>
            <button class="btn btn-ghost btn-small" id="conflict-save" ${resolved ? '' : 'disabled'} title="Write the chosen resolutions; unresolved conflicts keep their markers">Save</button>
            <button class="btn btn-primary btn-small" id="conflict-save-mark" ${resolved === total ? '' : 'disabled title="Resolve every conflict first"'}>Save &amp; mark resolved</button>
        </div>
        <div class="conflict-list">${cards}</div>`;
    updateStickyToolbar();

    contentBodyEl.querySelectorAll('[data-choice]').forEach((btn) => {
        btn.addEventListener('click', () => setConflictChoice(Number(btn.dataset.block), btn.dataset.choice));
    });
    el('conflict-all-ours').addEventListener('click', () => setAllConflicts('ours'));
    el('conflict-all-theirs').addEventListener('click', () => setAllConflicts('theirs'));
    el('conflict-reset').addEventListener('click', () => setAllConflicts(''));
    el('conflict-save').addEventListener('click', () => saveConflicts(false));
    el('conflict-save-mark').addEventListener('click', () => saveConflicts(true));
}

// Re-render keeping the scroll position (cards change height when resolved).
function rerenderConflicts() {
    const top = contentBodyEl.scrollTop;
    renderContentBody();
    contentBodyEl.scrollTop = top;
}

function setConflictChoice(i, choice) {
    state.conflictChoices[i] = choice;
    rerenderConflicts();
    const card = contentBodyEl.querySelector(`.conflict-card[data-index="${i}"]`);
    card?.classList.add('hunk-current');
}

function setAllConflicts(choice) {
    state.conflictChoices = state.conflicts.map(() => choice);
    rerenderConflicts();
}

// conflictKey applies a choice to the highlighted conflict (from ]/[), or to
// the first unresolved one.
function conflictKey(choice) {
    const current = contentBodyEl.querySelector('.conflict-card.hunk-current');
    let i = current ? Number(current.dataset.index) : state.conflictChoices.findIndex((c) => !c);
    if (i < 0) return;
    setConflictChoice(i, choice);
    if (!choice) return;
    // Move on to the next unresolved conflict.
    const next = state.conflictChoices.findIndex((c, j) => j > i && !c);
    if (next < 0) return;
    contentBodyEl.querySelector('.conflict-card.hunk-current')?.classList.remove('hunk-current');
    const card = contentBodyEl.querySelector(`.conflict-card[data-index="${next}"]`);
    card.classList.add('hunk-current');
    card.style.scrollMarginTop = `${stickyToolbarHeight() + 8}px`;
    card.scrollIntoView({ block: 'start' });
}

async function saveConflicts(markResolved) {
    const diff = state.diff;
    if (!diff) return;
    try {
        const msg = await SaveConflicts(diff.relPath, state.conflictChoices, markResolved);
        await reloadAfterSave(diff.relPath, msg);
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
}

// ---------------------------------------------------------------------------
// Merge mode
// ---------------------------------------------------------------------------

// toggleMergeBlock selects every change in the block starting at row start,
// or deselects them all when they already were.
function toggleMergeBlock(diff, start) {
    let end = start;
    while (end + 1 < diff.lines.length && diff.lines[end + 1].type !== 'equal') end++;
    const setFor = (i) => (diff.lines[i].type === 'insert' ? state.mergeSelection.insertions : state.mergeSelection.deletions);
    let allOn = true;
    for (let i = start; i <= end; i++) if (!setFor(i).has(i)) allOn = false;
    for (let i = start; i <= end; i++) {
        if (allOn) setFor(i).delete(i);
        else setFor(i).add(i);
    }
    return end;
}

function mergeStats() {
    const diff = state.diff;
    return {
        selIns: state.mergeSelection.insertions.size,
        totalIns: diff.inserted,
        selDel: state.mergeSelection.deletions.size,
        totalDel: diff.deleted,
    };
}

// saveEffect says what saving the checked changes does to the target. A checked
// change moves the target towards the other side, so a checked insertion adds a
// line to the left file but removes one from the right file, and vice versa.
function saveEffect(stats, target, targetName) {
    const adds = target === 'left' ? stats.selIns : stats.selDel;
    const removes = target === 'left' ? stats.selDel : stats.selIns;
    const lines = (n) => `${n} ${n === 1 ? 'line' : 'lines'}`;
    if (adds && removes) return `saving adds ${lines(adds)} and removes ${lines(removes)} in ${targetName}`;
    if (adds) return `saving adds ${lines(adds)} to ${targetName}`;
    return `saving removes ${lines(removes)} from ${targetName}`;
}

// saveStateHtml says whether the merge target on disk matches what the Merge
// view shows. Nothing checked means the view's result is the target as loaded.
function saveStateHtml(pending, effect, targetName) {
    if (state.targetDisk !== 'unchanged') {
        const what = state.targetDisk === 'missing' ? 'no longer exists' : 'was changed outside this view';
        return `<span class="save-state changed" title="${escapeHtml(targetName)} ${what} after it was loaded, so what's shown here is out of date">▲ Changed on disk</span>
            <button class="btn btn-ghost btn-small" id="merge-reload">Reload</button>`;
    }
    if (pending > 0) {
        return `<span class="save-state unsaved" title="Nothing is written until you save">○ Unsaved · ${escapeHtml(effect)}</span>`;
    }
    return `<span class="save-state saved" title="${escapeHtml(targetName)} on disk is what this view shows">● Saved · matches disk</span>`;
}

// diskCheckSeq identifies the latest disk check, so older replies are ignored.
let diskCheckSeq = 0;

// refreshTargetDisk compares the merge target on disk with what was loaded,
// and redraws the Merge view if the answer changed.
async function refreshTargetDisk() {
    const diff = state.diff;
    if (!diff || state.diffView !== 'merge') return;
    const target = state.mergeTarget;
    if (!(target === 'left' ? diff.leftEditable : diff.rightEditable)) return;
    const seq = ++diskCheckSeq;
    let result;
    try {
        result = await CheckTargetOnDisk(diff.relPath, target);
    } catch {
        return; // e.g. a reload replaced the comparison; the next check will tell
    }
    if (seq !== diskCheckSeq || state.diff !== diff || state.mergeTarget !== target) return;
    if (result === state.targetDisk) return;
    state.targetDisk = result;
    if (result !== 'unchanged' && diff.mergeStatus) {
        // The file no longer matches its last save, so that status is over.
        // If the other side was saved too, its status returns on reload.
        diff.mergeStatus = null;
        const entry = state.comparison?.files.find((f) => f.relPath === diff.relPath);
        if (entry) entry.mergeStatus = null;
        renderMergeStatus(diff);
        renderFileList();
    }
    if (state.diffView !== 'merge') return;
    renderMergeToolbar(diff);
}

window.addEventListener('focus', refreshTargetDisk);

function renderMerge(diff) {
    const lineRows = diff.lines
        .map((line, i) => {
            if (line.type === 'equal') {
                return `<div class="merge-line equal">
                    <span class="checkbox-cell"></span>
                    <span class="ln"></span>
                    <span class="marker"></span>
                    <span class="content">${escapeHtml(line.content)}</span>
                </div>`;
            }
            const set = line.type === 'insert' ? state.mergeSelection.insertions : state.mergeSelection.deletions;
            const selected = set.has(i);
            const lineNum = line.type === 'insert' ? line.rightLineNum : line.leftLineNum;
            const start = hunkStart(diff.lines, i);
            const blockBtn = start
                ? `<button class="block-toggle" data-block="${i}" title="Toggle this whole block of changes">▾</button>`
                : '';
            return `<div class="merge-line ${line.type} ${selected ? 'selected' : 'unselected'}${start}">
                <span class="checkbox-cell">${blockBtn}<input type="checkbox" data-index="${i}" ${selected ? 'checked' : ''}/></span>
                <span class="ln">${lineNum > 0 ? lineNum : ''}</span>
                <span class="marker">${markerFor(line.type)}</span>
                <span class="content">${escapeHtml(line.content)}</span>
            </div>`;
        })
        .join('');

    contentBodyEl.innerHTML = `
        <div class="merge-toolbar" id="merge-toolbar"></div>
        <div id="merge-lines">${lineRows}</div>
    `;
    renderMergeToolbar(diff);

    // One listener for all the rows, which can number in the tens of thousands.
    const linesEl = el('merge-lines');
    linesEl.addEventListener('change', (e) => {
        if (!e.target.matches('input[type="checkbox"]')) return;
        const i = Number(e.target.dataset.index);
        const set = diff.lines[i].type === 'insert' ? state.mergeSelection.insertions : state.mergeSelection.deletions;
        if (e.target.checked) set.add(i);
        else set.delete(i);
        syncMergeRows(diff, i, i);
        mergePicksChanged(diff);
    });
    linesEl.addEventListener('click', (e) => {
        const btn = e.target.closest('.block-toggle');
        if (!btn) return;
        const start = Number(btn.dataset.block);
        const end = toggleMergeBlock(diff, start);
        syncMergeRows(diff, start, end);
        mergePicksChanged(diff);
    });

    refreshTargetDisk();
}

// syncMergeRows makes rows first to last show the current picks, in place.
function syncMergeRows(diff, first, last) {
    const rows = el('merge-lines').children;
    for (let i = first; i <= last; i++) {
        const line = diff.lines[i];
        if (line.type === 'equal') continue;
        const set = line.type === 'insert' ? state.mergeSelection.insertions : state.mergeSelection.deletions;
        const selected = set.has(i);
        rows[i].classList.toggle('selected', selected);
        rows[i].classList.toggle('unselected', !selected);
        rows[i].querySelector('input[type="checkbox"]').checked = selected;
    }
}

function mergePicksChanged(diff) {
    renderMergeToolbar(diff);
    refreshTargetDisk();
}

// renderMergeToolbar draws the toolbar, which changes with every pick.
function renderMergeToolbar(diff) {
    const stats = mergeStats();
    const editable = { left: diff.leftEditable, right: diff.rightEditable };
    const targetLabel = state.mergeTarget === 'left' ? diff.leftLabel : diff.rightLabel;
    const targetName = targetLabel.split(/[\\/]/).pop();
    const isFile = editable[state.mergeTarget];
    const pending = stats.selIns + stats.selDel;
    const effect = saveEffect(stats, state.mergeTarget, targetName);
    const canSave = isFile && pending > 0 && state.targetDisk !== 'missing';
    const notFile = 'This side is a git revision, not a file on disk';
    const overwriting = state.targetDisk === 'changed';
    const saveLabel = overwriting ? 'Overwrite anyway' : `Save to ${escapeHtml(targetName)}`;
    let saveTitle = `${escapeHtml(effect)} (the old version is kept as ${escapeHtml(targetName)}.orig)`;
    if (!isFile) saveTitle = notFile;
    else if (state.targetDisk === 'missing') saveTitle = `${escapeHtml(targetName)} no longer exists`;
    else if (pending === 0) saveTitle = 'No changes are checked, so there is nothing to save';
    else if (overwriting) saveTitle = `${escapeHtml(targetName)} changed on disk after it was loaded. Saving replaces those edits (the current version is kept as ${escapeHtml(targetName)}.orig)`;

    el('merge-toolbar').innerHTML = `
        <div class="segmented" id="merge-target-toggle">
            <button class="segmented-btn${state.mergeTarget === 'left' ? ' active' : ''}" data-target="left" ${editable.left ? 'title="Apply the checked changes to the left file"' : `disabled title="${notFile}"`}>Apply to left</button>
            <button class="segmented-btn${state.mergeTarget === 'right' ? ' active' : ''}" data-target="right" ${editable.right ? 'title="Apply the checked changes to the right file"' : `disabled title="${notFile}"`}>Apply to right</button>
        </div>
        <div class="stats selection-stats">
            <span class="stat-insert">Insertions: ${stats.selIns}/${stats.totalIns} selected</span>
            <span class="stat-delete">Deletions: ${stats.selDel}/${stats.totalDel} selected</span>
        </div>
        ${isFile ? saveStateHtml(pending, effect, targetName) : ''}
        <div class="merge-actions">
            <button class="btn btn-ghost btn-small" id="merge-select-all">Select all</button>
            <button class="btn btn-ghost btn-small" id="merge-select-none">Select none</button>
            <button class="btn btn-primary btn-small" id="merge-save" title="${saveTitle}" ${canSave ? '' : 'disabled'}>${saveLabel}</button>
        </div>
    `;
    updateStickyToolbar();

    document.querySelectorAll('#merge-target-toggle .segmented-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
            if (btn.disabled) return;
            state.mergeTarget = btn.dataset.target;
            state.targetDisk = 'unchanged';
            mergePicksChanged(diff);
        });
    });

    el('merge-reload')?.addEventListener('click', () => reloadComparison('Reloaded from disk.'));

    el('merge-select-all').addEventListener('click', () => {
        resetMergeSelection();
        syncMergeRows(diff, 0, diff.lines.length - 1);
        mergePicksChanged(diff);
    });
    el('merge-select-none').addEventListener('click', () => {
        state.mergeSelection = { insertions: new Set(), deletions: new Set() };
        syncMergeRows(diff, 0, diff.lines.length - 1);
        mergePicksChanged(diff);
    });

    el('merge-save').addEventListener('click', async () => {
        if (!canSave) return;
        try {
            const msg = await SaveMerge({
                relPath: diff.relPath,
                target: state.mergeTarget,
                selectedInsertions: [...state.mergeSelection.insertions],
                selectedDeletions: [...state.mergeSelection.deletions],
                token: diff.token,
            });
            await reloadAfterSave(diff.relPath, msg);
        } catch (err) {
            setStatus(goErrorMessage(err), 'error');
        }
    });
}

// ---------------------------------------------------------------------------
// Copy mode
// ---------------------------------------------------------------------------

function copyEligibleFiles() {
    if (!state.comparison) return [];
    const wantSource = state.copyTarget === 'to-right' ? 'left' : 'right';
    return state.comparison.files
        .filter((f) => f.source === wantSource)
        .sort((a, b) => a.relPath.localeCompare(b.relPath));
}

function openCopyModal() {
    state.copyTarget = 'to-right';
    resetCopySelection();
    copyModal.classList.remove('hidden');
    renderCopyModal();
}

function closeCopyModal() {
    copyModal.classList.add('hidden');
}

function resetCopySelection() {
    state.copySelection = new Set(copyEligibleFiles().map((f) => f.relPath));
}

function renderCopyModal() {
    copyTargetToggle.querySelectorAll('.segmented-btn').forEach((btn) => {
        btn.classList.toggle('active', btn.dataset.target === state.copyTarget);
    });

    const files = copyEligibleFiles();
    if (files.length === 0) {
        copyFileListEl.innerHTML = `<div class="empty-state">No files can be copied in this direction.</div>`;
    } else {
        copyFileListEl.innerHTML = files
            .map(
                (f) => `
            <label class="copy-row">
                <input type="checkbox" data-relpath="${escapeHtml(f.relPath)}" ${state.copySelection.has(f.relPath) ? 'checked' : ''}/>
                <span class="name">${escapeHtml(f.displayName)}</span>
                ${badgeFor(f)}
            </label>`
            )
            .join('');

        copyFileListEl.querySelectorAll('input[type="checkbox"]').forEach((cb) => {
            cb.addEventListener('change', () => {
                if (cb.checked) state.copySelection.add(cb.dataset.relpath);
                else state.copySelection.delete(cb.dataset.relpath);
                renderCopySummary();
            });
        });
    }

    renderCopySummary();
}

function renderCopySummary() {
    const files = copyEligibleFiles();
    const selected = files.filter((f) => state.copySelection.has(f.relPath)).length;
    copySummaryEl.textContent = `${selected} of ${files.length} selected`;
}

copyFilesBtn.addEventListener('click', openCopyModal);
el('copy-modal-close').addEventListener('click', closeCopyModal);
copyModal.addEventListener('mousedown', (e) => {
    if (e.target === copyModal) closeCopyModal();
});

copyTargetToggle.querySelectorAll('.segmented-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
        state.copyTarget = btn.dataset.target;
        resetCopySelection();
        renderCopyModal();
    });
});

el('copy-select-all').addEventListener('click', () => {
    resetCopySelection();
    renderCopyModal();
});
el('copy-select-none').addEventListener('click', () => {
    state.copySelection.clear();
    renderCopyModal();
});

el('copy-execute-btn').addEventListener('click', async () => {
    const relPaths = [...state.copySelection];
    if (relPaths.length === 0) {
        setStatus('No files selected to copy.', 'error');
        return;
    }
    try {
        const msg = await CopyFiles(relPaths, state.copyTarget);
        closeCopyModal();
        // Refresh the comparison so copied files now show as identical/common.
        await reloadComparison();
        // After the reload, or its own status would replace this.
        if (!statusbarEl.classList.contains('error')) setStatus(msg, msg.includes('error(s)') ? 'warning' : 'success');
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
});

// ---------------------------------------------------------------------------
// Help modal
// ---------------------------------------------------------------------------

const helpModal = el('help-modal');

GetVersion().then((v) => {
    el('help-version').textContent = v;
});

function openHelpModal() {
    helpModal.classList.remove('hidden');
    el('help-modal-done').focus();
}

function closeHelpModal() {
    helpModal.classList.add('hidden');
}

el('help-btn').addEventListener('click', openHelpModal);
el('help-btn-slim').addEventListener('click', openHelpModal);
el('help-modal-close').addEventListener('click', closeHelpModal);
el('help-modal-done').addEventListener('click', closeHelpModal);
helpModal.addEventListener('mousedown', (e) => {
    if (e.target === helpModal) closeHelpModal();
});

document.addEventListener('keydown', (e) => {
    const openMenu = document.querySelector('.more-menu[open]');
    if (e.key === 'Escape' && openMenu) {
        openMenu.open = false;
        return;
    }
    if (e.key === 'Escape' && !helpModal.classList.contains('hidden')) {
        closeHelpModal();
        return;
    }
    if (e.key === 'Escape' && !copyModal.classList.contains('hidden')) {
        closeCopyModal();
        return;
    }
    if (e.key === 'F1') {
        e.preventDefault();
        openHelpModal();
        return;
    }
    // Single-key shortcuts, ignored while typing or with a modal open.
    const t = e.target;
    if (t.closest?.('input, textarea, select, [contenteditable]')) return;
    if (!copyModal.classList.contains('hidden') || !helpModal.classList.contains('hidden')) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key === '?') {
        openHelpModal();
        return;
    }
    if (e.key === 'f') toggleFocus();
    else if (e.key === 'l') toggleFiles();
    else if (e.key === 'P') togglePaths();
    else if (e.key === 'n') stepFile(1);
    else if (e.key === 'p') stepFile(-1);
    if (!state.diff) return;
    if (e.key === 'e') openInEditor(defaultEditSide(state.diff), 0);
    else if (e.key === 'd') openInDiffTool();
    else if (e.key === ']') jumpToChange(1);
    else if (e.key === '[') jumpToChange(-1);
    else if (state.diffView === 'conflicts') {
        const keyChoices = { o: 'ours', t: 'theirs', b: 'both', B: 'both-theirs-first', u: '' };
        if (e.key in keyChoices) conflictKey(keyChoices[e.key]);
    }
});

// ---------------------------------------------------------------------------
// Initial render
// ---------------------------------------------------------------------------

applyLayout();
renderContentHeader();

GetToolInfo()
    .then((info) => {
        state.toolInfo = info;
        if (state.diff) renderContentHeader();
    })
    .catch(() => {});

// Settings come first, so the first comparison opens in the default view.
GetSettings()
    .then((settings) => {
        el('config-path').textContent = settings.configPath || 'not available on this system';
        applySettings(settings);
        state.diffView = settings.defaultView;
        if (settings.configError) {
            setStatus(`Using default settings. Problem in ${settings.configPath}: ${settings.configError}`, 'warning');
        }
    })
    .catch(() => {})
    // Pre-fill paths passed on the command line, and compare if both were given.
    .then(() => GetInitialPaths())
    .then((paths) => {
        const [left, right] = paths || [];
        if (left) leftPathInput.value = left;
        if (right) rightPathInput.value = right;
        if (left && right) loadPaths(left, right, true);
        else updateComparePending();
    })
    .catch((err) => setStatus(goErrorMessage(err), 'error'));
