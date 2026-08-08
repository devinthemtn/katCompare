import './style.css';
import {
    LoadComparison,
    LoadGitComparison,
    GetDiff,
    SaveMerge,
    CopyFiles,
    SelectDirectory,
    SelectFile,
    SuggestPaths,
} from '../wailsjs/go/main/App';

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

const state = {
    mode: 'paths', // 'paths' | 'git'
    comparison: null, // ComparisonResult from the backend
    filterQuery: '',
    selectedRelPath: null,
    diff: null, // DiffResult for the selected file
    diffView: 'unified', // 'unified' | 'sidebyside' | 'merge'
    mergeTarget: 'left', // 'left' | 'right'
    mergeSelection: { insertions: new Set(), deletions: new Set() },
    copyTarget: 'to-right', // 'to-right' | 'to-left'
    copySelection: new Set(),
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

const filterInput = el('filter-input');
const fileListEl = el('file-list');
const copyFilesBtn = el('copy-files-btn');

const contentHeaderEl = el('content-header');
const contentBodyEl = el('content-body');

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
        .replace(/>/g, '&gt;');
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
    statusbarEl.classList.remove('error', 'success');
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
    });
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
    const path = await SelectFile();
    if (path) leftPathInput.value = path;
});
el('left-browse-dir').addEventListener('click', async () => {
    const path = await SelectDirectory();
    if (path) leftPathInput.value = path;
});
el('right-browse-file').addEventListener('click', async () => {
    const path = await SelectFile();
    if (path) rightPathInput.value = path;
});
el('right-browse-dir').addEventListener('click', async () => {
    const path = await SelectDirectory();
    if (path) rightPathInput.value = path;
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
        applyComparisonResult(result);
        setStatus(`Loaded ${result.files.length} file(s).`);
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
}

el('compare-btn').addEventListener('click', () => {
    const left = leftPathInput.value.trim();
    const right = rightPathInput.value.trim();
    if (!left || !right) {
        setStatus('Enter both a left and right path first.', 'error');
        return;
    }
    runComparison(LoadComparison(left, right));
});

el('git-compare-btn').addEventListener('click', () => {
    const leftRef = leftRefInput.value.trim() || 'HEAD';
    const rightRef = rightRefInput.value.trim();
    runComparison(LoadGitComparison(leftRef, rightRef));
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
    renderFileList();

    try {
        const diff = await GetDiff(relPath);
        state.diff = diff;
        state.mergeTarget = 'left';
        resetMergeSelection();
        if (state.diffView === 'merge' && !diff.canMerge) {
            state.diffView = 'unified';
        }
        renderContentHeader();
        renderContentBody();
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
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
            <div class="segmented" id="view-toggle">
                <button class="segmented-btn${state.diffView === 'unified' ? ' active' : ''}" data-view="unified">Unified</button>
                <button class="segmented-btn${state.diffView === 'sidebyside' ? ' active' : ''}" data-view="sidebyside">Side-by-side</button>
                <button class="segmented-btn${state.diffView === 'merge' ? ' active' : ''}" data-view="merge" ${diff.canMerge ? '' : 'disabled title="Only available for files that exist on both sides"'}>Merge</button>
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
}

function markerFor(type) {
    if (type === 'insert') return '+';
    if (type === 'delete') return '-';
    return '';
}

function renderUnified(diff) {
    contentBodyEl.innerHTML = diff.lines
        .map((line) => {
            const leftNum = line.leftLineNum > 0 ? line.leftLineNum : '';
            const rightNum = line.rightLineNum > 0 ? line.rightLineNum : '';
            return `<div class="diff-line ${line.type}">
                <span class="ln">${leftNum}</span>
                <span class="ln ln-right">${rightNum}</span>
                <span class="marker">${markerFor(line.type)}</span>
                <span class="content">${escapeHtml(line.content)}</span>
            </div>`;
        })
        .join('');
}

function sbsCell(content, lineNum, cls) {
    if (cls === 'empty') {
        return `<div class="sbs-cell empty"></div>`;
    }
    return `<div class="sbs-cell ${cls}">
        <div class="sbs-cell-inner">
            <span class="ln">${lineNum > 0 ? lineNum : ''}</span>
            <span class="content">${escapeHtml(content)}</span>
        </div>
    </div>`;
}

function renderSideBySide(diff) {
    contentBodyEl.innerHTML = `<div class="sbs-table">${diff.sideBySide
        .map((row) => {
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
            return `<div class="sbs-row">
                ${sbsCell(row.leftContent, row.leftLineNum, leftCls)}
                ${sbsCell(row.rightContent, row.rightLineNum, rightCls)}
            </div>`;
        })
        .join('')}</div>`;
}

// ---------------------------------------------------------------------------
// Merge mode
// ---------------------------------------------------------------------------

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

function renderMerge(diff) {
    const stats = mergeStats();
    const targetPath = state.mergeTarget === 'left' ? `${diff.leftLabel}.merged` : `${diff.rightLabel}.merged`;

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
            return `<div class="merge-line ${line.type} ${selected ? 'selected' : 'unselected'}">
                <span class="checkbox-cell"><input type="checkbox" data-index="${i}" ${selected ? 'checked' : ''}/></span>
                <span class="ln">${lineNum > 0 ? lineNum : ''}</span>
                <span class="marker">${markerFor(line.type)}</span>
                <span class="content">${escapeHtml(line.content)}</span>
            </div>`;
        })
        .join('');

    contentBodyEl.innerHTML = `
        <div class="merge-toolbar">
            <div class="segmented" id="merge-target-toggle">
                <button class="segmented-btn${state.mergeTarget === 'left' ? ' active' : ''}" data-target="left">Apply to left</button>
                <button class="segmented-btn${state.mergeTarget === 'right' ? ' active' : ''}" data-target="right">Apply to right</button>
            </div>
            <div class="stats">
                <span class="stat-insert">Insertions: ${stats.selIns}/${stats.totalIns} selected</span>
                <span class="stat-delete">Deletions: ${stats.selDel}/${stats.totalDel} selected</span>
            </div>
            <div class="spacer"></div>
            <button class="btn btn-ghost btn-small" id="merge-select-all">Select all</button>
            <button class="btn btn-ghost btn-small" id="merge-select-none">Select none</button>
            <button class="btn btn-primary btn-small" id="merge-save">Save to ${escapeHtml(targetPath)}</button>
        </div>
        <div id="merge-lines">${lineRows}</div>
    `;

    document.querySelectorAll('#merge-target-toggle .segmented-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
            state.mergeTarget = btn.dataset.target;
            renderContentBody();
        });
    });

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

    el('merge-save').addEventListener('click', async () => {
        try {
            const msg = await SaveMerge({
                relPath: diff.relPath,
                target: state.mergeTarget,
                selectedInsertions: [...state.mergeSelection.insertions],
                selectedDeletions: [...state.mergeSelection.deletions],
            });
            setStatus(msg, 'success');
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
        setStatus(msg, 'success');
        // Refresh the comparison so copied files now show as identical/common.
        const result = await LoadComparison(state.comparison.leftLabel, state.comparison.rightLabel);
        applyComparisonResult(result);
    } catch (err) {
        setStatus(goErrorMessage(err), 'error');
    }
});

document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && !copyModal.classList.contains('hidden')) {
        closeCopyModal();
    }
});

// ---------------------------------------------------------------------------
// Initial render
// ---------------------------------------------------------------------------

renderContentHeader();
