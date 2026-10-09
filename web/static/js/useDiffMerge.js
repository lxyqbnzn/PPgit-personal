(function(root) {
  "use strict";
  const MAX_MERGE_ROWS = 1e4;
  const TEXT = {
    zh: {
      apply: "将所选旧代码覆盖到右侧草稿",
      undo: "撤销上一次覆盖",
      discard: "放弃草稿",
      confirm: "确定替换",
      saving: "正在替换...",
      selected: "已选 {count} 行",
      pending: "待确认：{batches} 次覆盖，{count} 行",
      draft: "未保存草稿",
      select: "选择差异行 {line}",
      applied: "该行已覆盖到草稿",
      discardConfirm: "放弃对该文件的所有修改吗？",
      cancel: "取消",
      discardChanges: "放弃修改",
      readOnly: "当前文件或版本不支持局部替换",
      version_read_only: "目标版本已发布，不允许修改",
      preview_unavailable: "预览不完整，不能进行局部替换",
      invalid_order: "只能从旧版本覆盖到较新的版本",
      binary_file: "二进制文件不支持局部替换",
      invalid_encoding: "该文本编码不支持局部替换",
      no_changed_rows: "文件没有可替换的差异行",
      limit: "单次确认最多替换 10000 行，请分次处理。",
      conflict: "版本文件已发生变化，未保存本次替换。草稿仍保留；请放弃草稿并刷新后重新选择。",
      success: "已替换右侧版本文件，工作区未改动"
    },
    en: {
      apply: "Copy selected old lines into the right-hand draft",
      undo: "Undo last copy",
      discard: "Discard draft",
      confirm: "Confirm replacement",
      saving: "Replacing...",
      selected: "{count} lines selected",
      pending: "Pending: {batches} copies, {count} lines",
      draft: "Unsaved draft",
      select: "Select difference row {line}",
      applied: "Already copied into draft",
      discardConfirm: "Discard all changes to this file?",
      cancel: "Cancel",
      discardChanges: "Discard changes",
      readOnly: "This file or version does not support partial replacement",
      version_read_only: "The target version is published and cannot be modified",
      preview_unavailable: "Partial replacement is unavailable for an incomplete preview",
      invalid_order: "Changes can only be copied from an older version into a newer version",
      binary_file: "Binary files do not support partial replacement",
      invalid_encoding: "This text encoding does not support partial replacement",
      no_changed_rows: "This file has no difference rows to replace",
      limit: "A confirmation can replace at most 10000 lines. Split this change into smaller batches.",
      conflict: "A version file has changed. Nothing was saved and your draft is preserved. Discard the draft and refresh before selecting again.",
      success: "Right-hand version file replaced. Workspace unchanged."
    }
  };
  function projectRows(rows, stagedRows = /* @__PURE__ */ new Set()) {
    let rightLine = 0;
    return rows.map((row, canonicalIndex) => {
      const applied = stagedRows.has(canonicalIndex);
      const exists = applied ? row.left_no != null : row.right_no != null;
      return {
        ...row,
        canonicalIndex,
        draft_applied: applied,
        right_no: exists ? ++rightLine : null,
        right_text: applied ? row.left_text || "" : row.right_text,
        draft_removed: applied && row.left_no == null
      };
    });
  }
  function useDiffMerge(options) {
    const { shallowRef, computed, watch } = Vue;
    const selected = shallowRef(/* @__PURE__ */ new Set());
    const batches = shallowRef([]);
    const saving = shallowRef(false);
    const error = shallowRef("");
    const discardTarget = shallowRef(null);
    let afterDiscard = null;
    let anchor = null;
    function text(key, params = {}) {
      let message = (TEXT[options.language.value] || TEXT.zh)[key] || key;
      for (const [name, value] of Object.entries(params)) message = message.replaceAll(`{${name}}`, String(value));
      return message;
    }
    const staged = computed(() => new Set(batches.value.flat()));
    const dirty = computed(() => staged.value.size > 0);
    const editable = computed(() => options.item.value?.mergeable === true && !options.item.value.too_large && !options.item.value.truncated && !options.item.value.preview_limited && !options.item.value.message && !options.loading.value);
    const disabledReason = computed(() => {
      if (!options.item.value || options.loading.value || editable.value) return "";
      if (options.item.value.merge_disabled_code) return text(options.item.value.merge_disabled_code);
      return options.language.value === "zh" ? options.item.value.merge_disabled_reason || text("readOnly") : text("readOnly");
    });
    const rows = computed(() => projectRows(options.item.value?.diff_rows || [], staged.value));
    function selectable(index) {
      const row = options.item.value?.diff_rows?.[index];
      return editable.value && !saving.value && !discardTarget.value && row && row.status !== "same" && !staged.value.has(index);
    }
    function selectRow(index, event = {}) {
      if (!selectable(index)) return;
      const next = new Set(selected.value);
      const checked = !next.has(index);
      const first = event.shiftKey && anchor !== null ? Math.min(anchor, index) : index;
      const last = event.shiftKey && anchor !== null ? Math.max(anchor, index) : index;
      for (let current = first; current <= last; current += 1) {
        if (!selectable(current)) continue;
        if (checked) next.add(current);
        else next.delete(current);
      }
      if (next.size + staged.value.size > MAX_MERGE_ROWS) {
        error.value = text("limit");
        return;
      }
      selected.value = next;
      anchor = index;
      error.value = "";
    }
    function apply() {
      if (!editable.value || saving.value || discardTarget.value || !selected.value.size) return;
      batches.value = [...batches.value, [...selected.value].sort((a, b) => a - b)];
      selected.value = /* @__PURE__ */ new Set();
      anchor = null;
      error.value = "";
    }
    function reset() {
      cancelDiscard();
      selected.value = /* @__PURE__ */ new Set();
      batches.value = [];
      anchor = null;
      error.value = "";
    }
    function undo() {
      if (saving.value || discardTarget.value) return;
      batches.value = batches.value.slice(0, -1);
      selected.value = /* @__PURE__ */ new Set();
      anchor = null;
      error.value = "";
    }
    function requestDiscard(action = null) {
      if (discardTarget.value) return;
      discardTarget.value = { message: options.item.value?.path || "", shortId: "" };
      afterDiscard = action;
    }
    function cancelDiscard() {
      discardTarget.value = null;
      afterDiscard = null;
    }
    async function confirmDiscard() {
      if (!discardTarget.value || saving.value) return;
      const action = afterDiscard;
      reset();
      await action?.();
    }
    function discard() {
      if (saving.value || discardTarget.value) return;
      if (dirty.value) requestDiscard();
      else reset();
    }
    function guard(action) {
      if (saving.value) {
        options.showToast(text("saving"), true);
        return false;
      }
      if (discardTarget.value) return false;
      if (dirty.value) {
        requestDiscard(action);
        return false;
      }
      reset();
      return true;
    }
    async function confirm() {
      if (!dirty.value || saving.value || discardTarget.value || !editable.value) return;
      const target = options.getTarget();
      const item = options.item.value;
      saving.value = true;
      error.value = "";
      try {
        const data = await options.api(`/api/projects/${encodeURIComponent(target.projectId)}/diff/merge`, {
          method: "POST",
          body: JSON.stringify({
            from_version: target.fromVersion,
            to_version: target.toVersion,
            path: item.path,
            expected_from_object: item.from_object_id || "",
            expected_to_object: item.to_object_id || "",
            rows: [...staged.value].sort((a, b) => a - b)
          })
        });
        reset();
        options.showToast(text("success"));
        await options.onSaved(data, { ...target, path: item.path });
      } catch (failure) {
        error.value = failure.code === "merge_conflict" || failure.status === 409 ? text("conflict") : failure.code === "version_read_only" ? text("version_read_only") : failure.message;
      } finally {
        saving.value = false;
      }
    }
    watch(() => options.item.value?.diff_rows, () => {
      if (!dirty.value && !saving.value) reset();
    });
    return {
      selected,
      staged,
      batches,
      dirty,
      saving,
      error,
      editable,
      disabledReason,
      rows,
      text,
      selectRow,
      apply,
      undo,
      discard,
      guard,
      confirm,
      reset,
      discardTarget,
      cancelDiscard,
      confirmDiscard
    };
  }
  const DiffMergeToolbar = {
    name: "DiffMergeToolbar",
    props: {
      selectedCount: { type: Number, default: 0 },
      stagedCount: { type: Number, default: 0 },
      batchCount: { type: Number, default: 0 },
      saving: Boolean,
      editable: Boolean,
      error: { type: String, default: "" },
      disabledReason: { type: String, default: "" },
      text: { type: Function, required: true }
    },
    emits: ["apply", "undo", "discard", "confirm"],
    template: `
      <div class="diff-merge-toolbar" :data-dirty="stagedCount > 0">
        <div class="diff-merge-actions">
          <button class="diff-merge-icon diff-merge-apply" type="button" :title="text('apply')" :aria-label="text('apply')"
            :disabled="!editable || saving || !selectedCount" @click="$emit('apply')">
            <svg class="lucide-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12h14m-7-7 7 7-7 7"/></svg>
          </button>
          <span class="diff-merge-selection">{{ text('selected', { count: selectedCount }) }}</span>
          <button class="diff-merge-icon diff-merge-undo" type="button" :title="text('undo')" :aria-label="text('undo')"
            :disabled="saving || !batchCount" @click="$emit('undo')">
            <svg class="lucide-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 14 4 9l5-5M4 9h10a6 6 0 0 1 0 12"/></svg>
          </button>
          <button class="diff-merge-icon diff-merge-discard" type="button" :title="text('discard')" :aria-label="text('discard')"
            :disabled="saving || !stagedCount" @click="$emit('discard')">
            <svg class="lucide-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m18 6-12 12M6 6l12 12"/></svg>
          </button>
          <span v-if="stagedCount" class="diff-merge-pending" role="status">{{ text('pending', { batches: batchCount, count: stagedCount }) }}</span>
          <button class="diff-merge-confirm" type="button" :disabled="!editable || saving || !stagedCount" @click="$emit('confirm')">
            <svg class="lucide-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m9 12 2 2 4-4"/><circle cx="12" cy="12" r="10"/></svg>
            {{ text(saving ? 'saving' : 'confirm') }}
          </button>
        </div>
        <div v-if="disabledReason" class="diff-merge-disabled" role="status">{{ disabledReason }}</div>
        <div v-if="error" class="diff-merge-error" role="alert">{{ error }}</div>
      </div>
    `
  };
  root.PPGitDiffMerge = { useDiffMerge, projectRows, DiffMergeToolbar };
})(window);
