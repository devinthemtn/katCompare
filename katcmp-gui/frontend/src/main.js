import './style.css';
import {
    LoadComparison,
    LoadGitComparison,
    ListGitRefs,
    GetDiff,
    GetInitialPaths,
    SaveMerge,
    CheckTargetOnDisk,
    CopyFiles,
    SelectDirectory,
    SelectFile,
    SuggestPaths,
    GetToolInfo,
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

const state = {
    mode: 'paths', // 'paths' | 'git'
    comparison: null, // ComparisonResult from the backend
    filterQuery: '',
    selectedRelPath: null,
    diff: null, // DiffResult for the selected file
    diffView: 'unified', // 'unified' | 'sidebyside' | 'merge' | 'conflicts'
    mergeTarget: 'left', // 'left' | 'right'
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

async function runComparison(promise) {
    setStatus('Comparing…');
    try {
        const result = await promise;
        state.shown = { ...state.lastLoad };
        applyComparisonResult(result);
        setStatus(`Loaded ${result.files.length} file(s).`);
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
    updateComparePending();
}

function loadPaths(left, right) {
    state.lastLoad = { git: false, left, right };
    return runComparison(LoadComparison(left, right));
}

function loadGit(leftRef, rightRef) {
    state.lastLoad = { git: true, left: leftRef, right: rightRef };
    return runComparison(LoadGitComparison(leftRef, rightRef));
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
    loadPaths(left, right);
});

el('git-compare-btn').addEventListener('click', () => {
    const leftRef = leftRefInput.value.trim() || 'HEAD';
    const rightRef = rightRefInput.value.trim();
    loadGit(leftRef, rightRef);
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

function renderFileList() {
    if (!state.comparison || state.comparison.files.length === 0) {
        fileListEl.innerHTML = '<div class="empty-state">Load a comparison to see files here.</div>';
        return;
    }

    const files = [...state.comparison.files]
        .filter((f) => {
            if (!state.filterQuery) return true;
            return (
                f.displayName.toLowerCase().includes(state.filterQuery) ||
                f.relPath.toLowerCase().includes(state.filterQuery)
            );
        })
        .sort((a, b) => a.relPath.localeCompare(b.relPath));

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

async function selectFile(relPath) {
    state.selectedRelPath = relPath;
    // Read this before the first await: the save handler clears it once its
    // reload returns, which doesn't wait for this function.
    const justSaved = state.justSavedRelPath === relPath;
    state.justSavedRelPath = null;
    renderFileList();

    try {
        const diff = await GetDiff(relPath);
        const sameFile = state.diff?.relPath === relPath;
        state.diff = diff;
        state.targetDisk = 'unchanged';
        const keepTarget = justSaved && (state.mergeTarget === 'left' ? diff.leftEditable : diff.rightEditable);
        if (!keepTarget) state.mergeTarget = diff.leftEditable || !diff.rightEditable ? 'left' : 'right';
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
            state.diffView = 'unified';
        }
        if (state.diffView === 'merge' && !diff.canMerge) {
            state.diffView = 'unified';
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
        contentHeaderEl.innerHTML =
            '<div class="empty-state">Nothing loaded yet — enter two paths (or a git ref) above and press Compare.</div>';
        return;
    }

    contentHeaderEl.innerHTML = `
        <div class="content-title">
            ${escapeHtml(diff.displayName)}
            <span class="path-sub">${escapeHtml(diff.leftLabel)} ↔ ${escapeHtml(diff.rightLabel)}</span>
        </div>
        <div class="content-stats">
            <span class="stat-equal">${diff.equal} equal</span>
            <span class="stat-insert">+${diff.inserted}</span>
            <span class="stat-delete">-${diff.deleted}</span>
        </div>
        <div class="content-actions">
            ${externalActionsHtml(diff)}
            <div class="segmented" id="view-toggle">
                ${diff.conflicted ? `<button class="segmented-btn${state.diffView === 'conflicts' ? ' active' : ''}" data-view="conflicts">Conflicts</button>` : ''}
                <button class="segmented-btn${state.diffView === 'unified' ? ' active' : ''}" data-view="unified">Unified</button>
                <button class="segmented-btn${state.diffView === 'sidebyside' ? ' active' : ''}" data-view="sidebyside">Side-by-side</button>
                <button class="segmented-btn${state.diffView === 'merge' ? ' active' : ''}" data-view="merge" ${diff.canMerge ? '' : 'disabled title="Only available for files that exist on both sides"'}>Merge</button>
            </div>
            <div class="segmented" id="change-nav">
                <button class="segmented-btn" id="prev-change-btn" title="Previous change ([)">↑ Prev change</button>
                <button class="segmented-btn" id="next-change-btn" title="Next change (])">↓ Next change</button>
            </div>
        </div>
    `;

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
    const anchor = Math.round(contentBodyEl.clientHeight / 4);

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
}

function mergeStats() {
    const diff = state.diff;
    let totalIns = 0,
        totalDel = 0;
    diff.lines.forEach((l) => {
        if (l.type === 'insert') totalIns++;
        if (l.type === 'delete') totalDel++;
    });
    return {
        selIns: state.mergeSelection.insertions.size,
        totalIns,
        selDel: state.mergeSelection.deletions.size,
        totalDel,
    };
}

// saveStateHtml says whether the merge target on disk matches what the Merge
// view shows. Nothing checked means the view's result is the target as loaded.
function saveStateHtml(pending, targetName) {
    if (state.targetDisk !== 'unchanged') {
        const what = state.targetDisk === 'missing' ? 'no longer exists' : 'was changed outside this view';
        return `<span class="save-state changed" title="${escapeHtml(targetName)} ${what} after it was loaded, so what's shown here is out of date">▲ Changed on disk</span>
            <button class="btn btn-ghost btn-small" id="merge-reload">Reload</button>`;
    }
    if (pending > 0) {
        return `<span class="save-state unsaved" title="Save to write the checked changes to ${escapeHtml(targetName)}">○ Unsaved · ${pending} ${pending === 1 ? 'change' : 'changes'} not written</span>`;
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
    const top = contentBodyEl.scrollTop;
    renderContentBody();
    contentBodyEl.scrollTop = top;
}

window.addEventListener('focus', refreshTargetDisk);

function renderMerge(diff) {
    const stats = mergeStats();
    const editable = { left: diff.leftEditable, right: diff.rightEditable };
    const targetLabel = state.mergeTarget === 'left' ? diff.leftLabel : diff.rightLabel;
    const targetName = targetLabel.split(/[\\/]/).pop();
    const isFile = editable[state.mergeTarget];
    const pending = stats.selIns + stats.selDel;
    const canSave = isFile && pending > 0 && state.targetDisk !== 'missing';
    const notFile = 'This side is a git revision, not a file on disk';
    const overwriting = state.targetDisk === 'changed';
    const saveLabel = overwriting ? 'Overwrite anyway' : `Save to ${escapeHtml(targetName)}`;
    let saveTitle = `Overwrite ${escapeHtml(targetLabel)} (the old version is kept as ${escapeHtml(targetName)}.orig)`;
    if (!isFile) saveTitle = notFile;
    else if (state.targetDisk === 'missing') saveTitle = `${escapeHtml(targetName)} no longer exists`;
    else if (pending === 0) saveTitle = 'No changes are checked, so there is nothing to save';
    else if (overwriting) saveTitle = `${escapeHtml(targetName)} changed on disk after it was loaded. Saving replaces those edits (the current version is kept as ${escapeHtml(targetName)}.orig)`;

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
        <div class="merge-toolbar">
            <div class="segmented" id="merge-target-toggle">
                <button class="segmented-btn${state.mergeTarget === 'left' ? ' active' : ''}" data-target="left" ${editable.left ? 'title="Apply the checked changes to the left file"' : `disabled title="${notFile}"`}>Apply to left</button>
                <button class="segmented-btn${state.mergeTarget === 'right' ? ' active' : ''}" data-target="right" ${editable.right ? 'title="Apply the checked changes to the right file"' : `disabled title="${notFile}"`}>Apply to right</button>
            </div>
            <div class="stats">
                <span class="stat-insert">Insertions: ${stats.selIns}/${stats.totalIns} selected</span>
                <span class="stat-delete">Deletions: ${stats.selDel}/${stats.totalDel} selected</span>
            </div>
            ${isFile ? saveStateHtml(pending, targetName) : ''}
            <div class="merge-actions">
                <button class="btn btn-ghost btn-small" id="merge-select-all">Select all</button>
                <button class="btn btn-ghost btn-small" id="merge-select-none">Select none</button>
                <button class="btn btn-primary btn-small" id="merge-save" title="${saveTitle}" ${canSave ? '' : 'disabled'}>${saveLabel}</button>
            </div>
        </div>
        <div id="merge-lines">${lineRows}</div>
    `;

    document.querySelectorAll('#merge-target-toggle .segmented-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
            if (btn.disabled) return;
            state.mergeTarget = btn.dataset.target;
            state.targetDisk = 'unchanged';
            renderContentBody();
        });
    });

    el('merge-reload')?.addEventListener('click', () => reloadComparison('Reloaded from disk.'));

    el('merge-select-all').addEventListener('click', () => {
        resetMergeSelection();
        renderContentBody();
    });
    el('merge-select-none').addEventListener('click', () => {
        state.mergeSelection = { insertions: new Set(), deletions: new Set() };
        renderContentBody();
    });

    document.querySelectorAll('#merge-lines input[type="checkbox"]').forEach((cb) => {
        cb.addEventListener('change', () => {
            const i = Number(cb.dataset.index);
            const line = diff.lines[i];
            const set = line.type === 'insert' ? state.mergeSelection.insertions : state.mergeSelection.deletions;
            if (cb.checked) set.add(i);
            else set.delete(i);
            renderContentBody();
        });
    });

    document.querySelectorAll('#merge-lines .block-toggle').forEach((btn) => {
        btn.addEventListener('click', () => {
            toggleMergeBlock(diff, Number(btn.dataset.block));
            renderContentBody();
        });
    });

    el('merge-save').addEventListener('click', async () => {
        if (!canSave) return;
        try {
            const msg = await SaveMerge({
                relPath: diff.relPath,
                target: state.mergeTarget,
                selectedInsertions: [...state.mergeSelection.insertions],
                selectedDeletions: [...state.mergeSelection.deletions],
            });
            await reloadAfterSave(diff.relPath, msg);
        } catch (err) {
            setStatus(goErrorMessage(err), 'error');
        }
    });

    refreshTargetDisk();
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
        setStatus(msg, 'success');
        // Refresh the comparison so copied files now show as identical/common.
        reloadComparison();
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
el('help-modal-close').addEventListener('click', closeHelpModal);
el('help-modal-done').addEventListener('click', closeHelpModal);
helpModal.addEventListener('mousedown', (e) => {
    if (e.target === helpModal) closeHelpModal();
});

document.addEventListener('keydown', (e) => {
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

renderContentHeader();

GetToolInfo()
    .then((info) => {
        state.toolInfo = info;
        if (state.diff) renderContentHeader();
    })
    .catch(() => {});

// Pre-fill paths passed on the command line, and compare if both were given.
GetInitialPaths()
    .then((paths) => {
        const [left, right] = paths || [];
        if (left) leftPathInput.value = left;
        if (right) rightPathInput.value = right;
        if (left && right) loadPaths(left, right);
        else updateComparePending();
    })
    .catch((err) => setStatus(goErrorMessage(err), 'error'));
