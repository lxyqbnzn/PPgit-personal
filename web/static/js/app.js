(function() {
  "use strict";
  const {
    createApp,
    ref,
    shallowRef,
    computed,
    watch,
    onMounted,
    onBeforeUnmount
  } = Vue;
  const appRoot = document.getElementById("app");
  const assetVersion = appRoot?.dataset.assetVersion || "";
  const MAX_HIGHLIGHT_CHARS = 2e5;
  const I18N = {
    zh: {
      brandSubtitle: "本地版本管理平台",
      addProject: "添加工作区",
      managedProjects: "已管理工作区",
      noProjects: "还没有添加项目",
      removeProjectAction: "移出",
      removeProjectTitle: "移出管理列表",
      projectConsole: "Project Console",
      selectProject: "请选择一个项目",
      addProjectHint: "添加项目后即可开始管理版本",
      refresh: "刷新",
      createVersion: "创建版本",
      directCreate: "直接创建",
      language: "语言",
      currentVersion: "当前工作区版本",
      workspaceVersionMarker: "最近工作区版本",
      unnamedVersion: "未命名版本",
      versionId: "版本编号",
      uncommittedFiles: "未提交文件",
      statusLoading: "检查中",
      statusUnknown: "未知",
      statusFailed: "无法读取工作区状态，请刷新重试",
      loadMoreVersions: "加载更多版本",
      loadingVersions: "正在加载版本...",
      historyProgress: "已加载 {loaded} / {total} 个版本",
      history: "History",
      versionTimeline: "版本时间线",
      compareHint: "可选 2 个版本进行对比",
      selectProjectForHistory: "请选择一个项目查看版本历史",
      noVersionHistory: "当前项目还没有版本历史",
      deleteVersion: "删除版本",
      deleteAction: "删除",
      fileCount: "{count} 个文件",
      inspector: "Inspector",
      versionDetail: "版本详情",
      noVersionSelected: "尚未选择版本",
      filesTab: "文件树",
      diffTab: "版本对比",
      diffMode: "对比模式",
      diffChanged: "Changed",
      diffAll: "All",
      diffRows: "{count} 行",
      fromVersion: "From",
      toVersion: "To",
      checkoutVersion: "切换工作区到该版本",
      checkoutTitle: "切换工作区",
      checkoutDirtyWarning: "工作区有未提交版本文件，切换后会消失且无法恢复，确认切换吗？",
      checkoutChanged: "工作区在检查或确认期间又发生了变化，请重新确认切换。",
      checkingOut: "正在切换...",
      confirmYes: "是",
      confirmNo: "否",
      selectVersionForTree: "选择版本以显示文件树",
      selectTwoVersionsForDiff: "请选择两个版本进行对比后查看差异文件",
      waitingAction: "等待操作",
      waitingActionHint: "点击版本快照可查看文件内容和版本对比，或选择两个版本以完成比较。",
      selectDiffFile: "请选择一个差异文件查看内容",
      loadingDiff: "正在加载差异...",
      noDiffPreview: "[没有可用差异预览]",
      diffPreviewBudget: "[差异内容超过安全预览预算，请按文件对比或使用外部对比工具]",
      fileTooLargeDiff: "[文件过大 (>10MB)，跳过差异预览]",
      nonTextDiff: "[非文本文件，无法预览差异]",
      fileTooLargeContent: "[文件过大 (>10MB)，跳过内容预览]",
      nonTextContent: "[非文本文件，无法直接预览]",
      projectPath: "工作区文件夹绝对路径",
      projectPathPlaceholder: "例如：E:\\my-code-project",
      chooseFolder: "选择文件夹",
      cancel: "取消",
      addingProject: "正在添加...",
      addAndInitRepo: "添加并建立版本库",
      createNewVersion: "创建新版本",
      loadingChangedFiles: "正在加载变更文件...",
      readingChangedFiles: "正在读取文件 {processed}/{total}",
      preparingChangedFiles: "正在准备文件扫描...",
      changedFilesSummary: "共 {count} 个变更文件",
      showChangedFileNames: "显示变更文件名",
      hideChangedFileNames: "收起变更文件名",
      moreChangedFiles: "再显示 {count} 个文件",
      noCommitChanges: "当前没有可创建版本的变更",
      versionMessage: "版本说明",
      versionMessagePlaceholder: "例如：修复用户列表页缺陷，优化查询边界处理",
      creating: "正在创建...",
      confirmCreateVersion: "确认创建版本",
      added: "新增",
      modified: "修改",
      deleted: "删除",
      requestFailed: "请求失败",
      permissionDenied: "无法写入“{path}”。请确认运行 PPGit 本地服务的账户对该目录具有写入权限，并避免从受限终端启动服务。",
      removeProjectConfirm: "确认删除已管理项目“{name}”吗？\n这只会移除 PPGit 管理记录，不会删除磁盘上的项目文件。",
      projectRemoved: "项目已从管理列表中删除",
      maxCompareVersions: "最多只能选择两个版本进行对比",
      checkoutDone: "工作区已切换到所选版本",
      deleteVersionConfirm: "确认删除该版本吗？",
      deletingVersion: "正在删除...",
      versionDeleted: "版本已删除",
      versionDeletedFromDisk: "版本已删除，清理 {trees} 个树文件、{objects} 个对象文件，释放 {size}",
      enterVersionMessage: "请输入版本描述",
      versionCreated: "新版本创建成功",
      enterProjectPath: "请输入项目路径",
      projectAdded: "项目添加成功",
      fileDeletedSuffix: "（已删除）"
    },
    en: {
      brandSubtitle: "Local code version management platform",
      addProject: "Add Workspace",
      managedProjects: "Managed Workspaces",
      noProjects: "No projects added yet",
      removeProjectAction: "Remove",
      removeProjectTitle: "Remove from managed projects",
      projectConsole: "Project Console",
      selectProject: "Select a project",
      addProjectHint: "Add a project to start managing versions",
      refresh: "Refresh",
      createVersion: "Create Version",
      directCreate: "Direct Create",
      language: "Language",
      currentVersion: "Current Workspace Version",
      workspaceVersionMarker: "Current Workspace Version",
      unnamedVersion: "Unnamed version",
      versionId: "Version ID",
      uncommittedFiles: "Uncommitted Files",
      statusLoading: "Checking",
      statusUnknown: "Unknown",
      statusFailed: "Unable to read workspace status. Refresh to retry.",
      loadMoreVersions: "Load More Versions",
      loadingVersions: "Loading versions...",
      historyProgress: "{loaded} / {total} versions loaded",
      history: "History",
      versionTimeline: "Version Timeline",
      compareHint: "Select 2 versions to compare",
      selectProjectForHistory: "Select a project to view version history",
      noVersionHistory: "This project has no version history yet",
      deleteVersion: "Delete Version",
      deleteAction: "Delete",
      fileCount: "{count} files",
      inspector: "Inspector",
      versionDetail: "Version Details",
      noVersionSelected: "No version selected",
      filesTab: "File Tree",
      diffTab: "Version Diff",
      diffMode: "Diff mode",
      diffChanged: "Changed",
      diffAll: "All",
      diffRows: "{count} rows",
      fromVersion: "From",
      toVersion: "To",
      checkoutVersion: "Switch workspace to this version",
      checkoutTitle: "Switch Workspace",
      checkoutDirtyWarning: "The workspace has uncommitted file changes. Switching will discard them and they cannot be recovered. Continue?",
      checkoutChanged: "The workspace changed during the check or confirmation. Please confirm the switch again.",
      checkingOut: "Switching...",
      confirmYes: "Yes",
      confirmNo: "No",
      selectVersionForTree: "Select a version to show the file tree",
      selectTwoVersionsForDiff: "Select two versions to view changed files",
      waitingAction: "Waiting for Action",
      waitingActionHint: "Click a version snapshot to view file content and diffs, or select two versions to compare.",
      selectDiffFile: "Select a changed file to view its content",
      loadingDiff: "Loading diff...",
      noDiffPreview: "[No diff preview available]",
      diffPreviewBudget: "[Diff exceeds the safe preview budget. Compare individual files or use an external diff tool.]",
      fileTooLargeDiff: "[File too large (>10MB), diff preview skipped]",
      nonTextDiff: "[Non-text file, diff preview unavailable]",
      fileTooLargeContent: "[File too large (>10MB), content preview skipped]",
      nonTextContent: "[Non-text file, direct preview unavailable]",
      projectPath: "Absolute workspace folder path",
      projectPathPlaceholder: "Example: E:\\my-code-project",
      chooseFolder: "Choose folder",
      cancel: "Cancel",
      addingProject: "Adding...",
      addAndInitRepo: "Add and Initialize Repository",
      createNewVersion: "Create New Version",
      loadingChangedFiles: "Loading changed files...",
      readingChangedFiles: "Reading files {processed}/{total}",
      preparingChangedFiles: "Preparing file scan...",
      changedFilesSummary: "{count} changed files",
      showChangedFileNames: "Show changed file names",
      hideChangedFileNames: "Hide changed file names",
      moreChangedFiles: "Show {count} more files",
      noCommitChanges: "There are no changes to create a version from",
      versionMessage: "Version Message",
      versionMessagePlaceholder: "Example: fix user list page defect and improve query boundary handling",
      creating: "Creating...",
      confirmCreateVersion: "Confirm Create Version",
      added: "Added",
      modified: "Modified",
      deleted: "Deleted",
      requestFailed: "Request failed",
      permissionDenied: 'Cannot write to "{path}". Check that the account running the local PPGit service has write permission for this folder, and avoid starting the service from a restricted terminal.',
      removeProjectConfirm: 'Delete managed project "{name}"?\nThis only removes the PPGit management record. It will not delete project files on disk.',
      projectRemoved: "Project removed from the managed list",
      maxCompareVersions: "You can select at most two versions to compare",
      checkoutDone: "Workspace switched to the selected version",
      deleteVersionConfirm: "Delete this version?",
      deletingVersion: "Deleting...",
      versionDeleted: "Version deleted",
      versionDeletedFromDisk: "Version deleted. Removed {trees} tree files and {objects} object files, freed {size}",
      enterVersionMessage: "Enter a version description",
      versionCreated: "New version created",
      enterProjectPath: "Enter a project path",
      projectAdded: "Project added successfully",
      fileDeletedSuffix: "(deleted)"
    }
  };
  const BACKEND_MESSAGE_KEYS = {
    "[文件过大 (>10MB)，跳过差异预览]": "fileTooLargeDiff",
    "[非文本文件，无法预览差异]": "nonTextDiff",
    "[没有可用差异预览]": "noDiffPreview",
    "[差异内容超过安全预览预算，请按文件对比或使用外部对比工具]": "diffPreviewBudget",
    "[文件过大 (>10MB)，跳过内容预览]": "fileTooLargeContent",
    "[非文本文件，无法直接预览]": "nonTextContent"
  };
  let currentLanguage = "zh";
  const COMMIT_FILES_PAGE_SIZE = 200;
  function translate(language, key, params = {}) {
    const dictionary = I18N[language] || I18N.zh;
    const template = dictionary[key] || I18N.zh[key] || key;
    return Object.entries(params).reduce(
      (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)),
      template
    );
  }
  function formatDateValue(value, language) {
    const locale = language === "en" ? "en-US" : "zh-CN";
    return new Date(value).toLocaleString(locale, { hour12: false });
  }
  function formatBytes(bytes) {
    if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
    if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${bytes} B`;
  }
  function statusLabelValue(status, language) {
    const labels = {
      added: translate(language, "added"),
      modified: translate(language, "modified"),
      deleted: translate(language, "deleted")
    };
    return labels[status] || status;
  }
  function escapeHtml(value) {
    return String(value ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;");
  }
  function versionedStaticUrl(path) {
    const suffix = assetVersion ? `?v=${encodeURIComponent(assetVersion)}` : "";
    return `/static/${path}${suffix}`;
  }
  let highlightAssetsPromise = null;
  function ensureHighlightAssets() {
    if (highlightAssetsPromise) return highlightAssetsPromise;
    const stylesheetPromise = new Promise((resolve) => {
      const existing = document.getElementById("ppgit-highlight-theme");
      if (existing) {
        resolve();
        return;
      }
      const link = document.createElement("link");
      link.id = "ppgit-highlight-theme";
      link.rel = "stylesheet";
      link.href = versionedStaticUrl("vendor/github-dark.min.css");
      link.onload = resolve;
      link.onerror = resolve;
      document.head.appendChild(link);
    });
    const scriptPromise = new Promise((resolve) => {
      if (window.hljs) {
        resolve(true);
        return;
      }
      const script = document.createElement("script");
      script.src = versionedStaticUrl("vendor/highlight.min.js");
      script.async = true;
      script.onload = () => resolve(Boolean(window.hljs));
      script.onerror = () => resolve(false);
      document.head.appendChild(script);
    });
    highlightAssetsPromise = Promise.all([stylesheetPromise, scriptPromise]).then(([, loaded]) => loaded);
    return highlightAssetsPromise;
  }
  function isAbortError(error) {
    return error?.name === "AbortError";
  }
  function diffCellClassValue(row, side) {
    const status = row?.status || "same";
    if (status === "deleted" && side === "right") return "diff-cell-empty diff-cell-deleted";
    if (status === "added" && side === "left") return "diff-cell-empty diff-cell-added";
    return `diff-cell-${status}`;
  }
  const pendingWriteRequests = shallowRef(0);
  let writeRequestEpoch = 0;
  async function api(url, options = {}) {
    const headers = new Headers(options.headers);
    headers.set("Content-Type", "application/json");
    const isWrite = !["GET", "HEAD", "OPTIONS"].includes((options.method || "GET").toUpperCase());
    if (isWrite) {
      const token = document.querySelector('meta[name="ppgit-csrf-token"]')?.content;
      if (token) headers.set("X-PPGit-Token", token);
    }
    if (isWrite) {
      pendingWriteRequests.value += 1;
      writeRequestEpoch += 1;
    }
    try {
      const response = await fetch(url, { ...options, headers });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        const message = data.code === "permission_denied" ? translate(currentLanguage, "permissionDenied", { path: data.path || "" }) : data.error || translate(currentLanguage, "requestFailed");
        const error = new Error(message);
        error.status = response.status;
        error.code = data.code;
        error.confirmationToken = data.confirmation_token;
        error.uncommittedCount = data.uncommitted_count;
        throw error;
      }
      return data;
    } finally {
      if (isWrite) pendingWriteRequests.value -= 1;
    }
  }
  const FileTreeNode = {
    name: "FileTreeNode",
    props: {
      node: { type: Object, required: true },
      changedPaths: { type: Map, default: () => /* @__PURE__ */ new Map() },
      changedDirPaths: { type: Set, default: () => /* @__PURE__ */ new Set() },
      language: { type: String, default: "zh" }
    },
    emits: ["select-file"],
    template: `
      <div v-if="node.type === 'directory'">
        <div class="file-node directory" :class="{ 'dir-changed': changedDirPaths.has(node.path) }" @click="expanded = !expanded">
          <span class="tree-arrow" :class="{ open: expanded }">&#9654;</span>
          {{ node.name }}
        </div>
        <div v-if="expanded" class="tree-children">
          <file-tree-node
            v-for="child in (node.children || [])"
            :key="child.path || child.name"
            :node="child"
            :changed-paths="changedPaths"
            :changed-dir-paths="changedDirPaths"
            :language="language"
            @select-file="$emit('select-file', $event)"
          />
        </div>
      </div>
      <button
        v-else
        class="file-node"
        :class="[fileChangeClass, { 'file-deleted': node.deleted }]"
        @click="!node.deleted && $emit('select-file', node.path)"
        :title="fileTitle"
      >
        <span class="tree-file-icon">&#128196;</span> {{ node.name }}
      </button>
    `,
    setup(props) {
      const expanded = ref(false);
      const fileChangeClass = computed(() => {
        const status = props.changedPaths.get(props.node.path);
        return status ? `changed-${status}` : "";
      });
      const fileTitle = computed(() => {
        if (props.node.deleted) {
          return `${props.node.path} ${translate(props.language, "fileDeletedSuffix")}`;
        }
        return `${props.node.path} · ${formatBytes(props.node.size || 0)}`;
      });
      return { expanded, fileChangeClass, fileTitle, formatBytes };
    }
  };
  const VIRTUAL_DIFF_HEADER_HEIGHT = 38;
  const VirtualDiffRows = {
    name: "VirtualDiffRows",
    props: {
      rows: { type: Array, default: () => [] },
      leftLabel: { type: String, default: "" },
      rightLabel: { type: String, default: "" },
      selectedRows: { type: Set, default: () => /* @__PURE__ */ new Set() },
      editable: Boolean,
      saving: Boolean,
      mergeText: { type: Function, default: () => "" },
      viewportKey: { type: String, default: "" },
      rowHeight: { type: Number, default: 30 },
      overscan: { type: Number, default: 12 }
    },
    emits: ["select-row"],
    template: `
      <div ref="scroller" class="virtual-diff-scroller" @scroll="handleScroll">
        <div class="virtual-diff-content">
          <div class="virtual-diff-head-row">
            <div class="side-diff-head">{{ leftLabel }}</div>
            <div class="side-diff-head">{{ rightLabel }}</div>
          </div>
          <div class="virtual-diff-spacer" :style="{ height: totalHeight + 'px' }">
            <div class="virtual-diff-window" :style="{ transform: 'translateY(' + offsetY + 'px)' }">
              <div
                v-for="entry in visibleRows"
                :key="entry.row.canonicalIndex ?? entry.index"
                class="virtual-diff-row"
                :data-row-index="entry.row.canonicalIndex"
                :style="{ height: rowHeight + 'px' }"
              >
                <div class="side-diff-cell diff-source-cell" :class="[diffCellClass(entry.row, 'left'), { 'diff-cell-selected': selectedRows.has(entry.row.canonicalIndex) }]" @click="selectRow(entry.row, $event)">
                  <span class="diff-row-select-slot">
                    <input v-if="editable && entry.row.status !== 'same'" type="checkbox" class="diff-row-select"
                      :data-row-index="entry.row.canonicalIndex" :checked="selectedRows.has(entry.row.canonicalIndex)"
                      :disabled="saving || entry.row.draft_applied"
                      :aria-label="mergeText('select', { line: entry.row.left_no ?? entry.row.right_no })"
                      :title="entry.row.draft_applied ? mergeText('applied') : mergeText('select', { line: entry.row.left_no ?? entry.row.right_no })"
                      @click.stop="selectRow(entry.row, $event)" />
                  </span>
                  <span class="diff-line-no">{{ entry.row.left_no ?? '' }}</span>
                  <code :title="entry.row.left_text">{{ entry.row.left_text }}</code>
                </div>
                <div class="side-diff-cell" :class="[diffCellClass(entry.row, 'right'), { 'diff-cell-draft': entry.row.draft_applied, 'diff-cell-draft-removed': entry.row.draft_removed }]">
                  <span class="diff-line-no">{{ entry.row.right_no ?? '' }}</span>
                  <code :title="entry.row.right_text">{{ entry.row.right_text }}</code>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    `,
    setup(props, { emit }) {
      const scroller = ref(null);
      const scrollTop = shallowRef(0);
      const viewportHeight = shallowRef(480);
      const totalHeight = computed(() => props.rows.length * props.rowHeight);
      const startIndex = computed(() => Math.max(
        Math.floor(Math.max(scrollTop.value - VIRTUAL_DIFF_HEADER_HEIGHT, 0) / props.rowHeight) - props.overscan,
        0
      ));
      const endIndex = computed(() => Math.min(
        startIndex.value + Math.ceil(viewportHeight.value / props.rowHeight) + props.overscan * 2,
        props.rows.length
      ));
      const offsetY = computed(() => startIndex.value * props.rowHeight);
      const visibleRows = computed(() => props.rows.slice(startIndex.value, endIndex.value).map((row, offset) => ({ row, index: startIndex.value + offset })));
      function updateViewport() {
        if (scroller.value) viewportHeight.value = scroller.value.clientHeight;
      }
      function handleScroll(event) {
        scrollTop.value = event.currentTarget.scrollTop;
      }
      function selectRow(row, event) {
        if (event.target.type === "checkbox") event.target.checked = props.selectedRows.has(row.canonicalIndex);
        if (!props.editable || props.saving || row.draft_applied || row.status === "same") return;
        if (event.target.type !== "checkbox" && window.getSelection()?.toString()) return;
        emit("select-row", row.canonicalIndex, event);
      }
      let resizeObserver = null;
      onMounted(() => {
        updateViewport();
        if (window.ResizeObserver) {
          resizeObserver = new window.ResizeObserver(updateViewport);
          resizeObserver.observe(scroller.value);
        } else {
          window.addEventListener("resize", updateViewport);
        }
      });
      onBeforeUnmount(() => {
        resizeObserver?.disconnect();
        window.removeEventListener("resize", updateViewport);
      });
      watch(() => props.viewportKey, () => {
        scrollTop.value = 0;
        if (scroller.value) scroller.value.scrollTop = 0;
      });
      return {
        scroller,
        totalHeight,
        offsetY,
        visibleRows,
        handleScroll,
        selectRow,
        diffCellClass: diffCellClassValue
      };
    }
  };
  const BorderGlowButton = {
    name: "BorderGlowButton",
    template: `
      <button
        ref="buttonRef"
        type="button"
        class="border-glow-button"
        @pointermove="handlePointerMove"
        @pointerleave="resetGlow"
      >
        <span class="border-glow-edge" aria-hidden="true"></span>
        <span class="border-glow-content"><slot /></span>
      </button>
    `,
    setup() {
      const buttonRef = ref(null);
      function handlePointerMove(event) {
        const button = buttonRef.value;
        if (!button) return;
        const rect = button.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        const centerX = rect.width / 2;
        const centerY = rect.height / 2;
        const deltaX = x - centerX;
        const deltaY = y - centerY;
        const edgeX = deltaX === 0 ? 0 : Math.abs(deltaX) / centerX;
        const edgeY = deltaY === 0 ? 0 : Math.abs(deltaY) / centerY;
        const proximity = Math.min(Math.max(edgeX, edgeY), 1) * 100;
        let angle = Math.atan2(deltaY, deltaX) * (180 / Math.PI) + 90;
        if (angle < 0) angle += 360;
        button.style.setProperty("--edge-proximity", proximity.toFixed(3));
        button.style.setProperty("--cursor-angle", `${angle.toFixed(3)}deg`);
      }
      function resetGlow() {
        buttonRef.value?.style.setProperty("--edge-proximity", "0");
      }
      return { buttonRef, handlePointerMove, resetGlow };
    }
  };
  const app = createApp({
    setup() {
      const language = shallowRef("zh");
      const projects = shallowRef([]);
      const activeProjectId = shallowRef(null);
      const activeProject = shallowRef(null);
      const versions = shallowRef([]);
      const historyPage = shallowRef(0);
      const historyPages = shallowRef(0);
      const historyTotal = shallowRef(0);
      const historyLoading = shallowRef(false);
      const selectedVersionId = shallowRef(null);
      const selectedVersion = shallowRef(null);
      const versionLoading = shallowRef(false);
      const checkingOut = shallowRef(false);
      const detailTree = shallowRef([]);
      const compareIds = shallowRef([]);
      const diffData = shallowRef(null);
      const activeDiffDetail = shallowRef(null);
      const activeDiffPath = shallowRef(null);
      const diffPairKey = shallowRef(null);
      const diffFileLoading = shallowRef(false);
      const diffViewMode = shallowRef("changed");
      const activeTab = shallowRef("files");
      const statusCount = shallowRef(null);
      const statusLoading = shallowRef(false);
      const statusFailed = shallowRef(false);
      const fileContentRaw = shallowRef("");
      const fileContentLoaded = shallowRef(false);
      const fileContentMessageKey = shallowRef(null);
      const highlighterReady = shallowRef(Boolean(window.hljs));
      const toastMessage = shallowRef("");
      const toastVisible = shallowRef(false);
      const toastIsError = shallowRef(false);
      const activeModal = shallowRef(null);
      const projectPathInput = shallowRef("");
      const projectError = shallowRef("");
      const browsingFolder = shallowRef(false);
      const commitMessage = shallowRef("");
      const directCreate = shallowRef(false);
      const commitFiles = shallowRef([]);
      const commitFileCount = shallowRef(0);
      const commitFilesLoading = shallowRef(false);
      const commitFilesEmpty = shallowRef(false);
      const commitScanFailed = shallowRef(false);
      const commitFileDetailsVisible = shallowRef(false);
      const commitFileDetailsLoading = shallowRef(false);
      const commitScanId = shallowRef(null);
      const commitScanProcessed = shallowRef(0);
      const commitScanTotal = shallowRef(0);
      const commitFileDisplayLimit = shallowRef(COMMIT_FILES_PAGE_SIZE);
      const submittingProject = shallowRef(false);
      const submittingCommit = shallowRef(false);
      const pendingDeleteVersion = shallowRef(null);
      const submittingDelete = shallowRef(false);
      const deleteVersionError = shallowRef("");
      const pendingPageWork = computed(() => Boolean(
        pendingWriteRequests.value || submittingProject.value || submittingCommit.value || submittingDelete.value || checkingOut.value || browsingFolder.value || activeModal.value === "commit" && commitFilesLoading.value || activeModal.value === "project" && projectPathInput.value.trim() || activeModal.value === "commit" && commitMessage.value.trim() || diffMerge.dirty.value || diffMerge.saving.value
      ));
      window.usePageLeaveGuard(pendingPageWork);
      const detailTitle = shallowRef(translate(language.value, "versionDetail"));
      const detailMeta = shallowRef(translate(language.value, "noVersionSelected"));
      let projectRequestController = null;
      let historyRequestController = null;
      let statusRequestController = null;
      let versionRequestController = null;
      let fileRequestController = null;
      let diffSummaryRequestController = null;
      let diffFileRequestController = null;
      let commitScanRequestController = null;
      let fileRequestEpoch = 0;
      let diffSummaryRequestEpoch = 0;
      let diffFileRequestEpoch = 0;
      const diffFileCache = /* @__PURE__ */ new Map();
      const DIFF_FILE_CACHE_LIMIT = 100;
      const DIFF_FILE_CACHE_BYTES = 32 * 1024 * 1024;
      const DIFF_FILE_CACHE_ROWS = 1e5;
      const HISTORY_PAGE_SIZE = 50;
      let diffFileCacheBytes = 0;
      let diffFileCacheRows = 0;
      function t(key, params = {}) {
        return translate(language.value, key, params);
      }
      function setLanguage(nextLanguage) {
        if (!I18N[nextLanguage]) return;
        currentLanguage = nextLanguage;
        document.documentElement.lang = nextLanguage === "en" ? "en" : "zh-CN";
        language.value = nextLanguage;
      }
      const checkout = window.PPGitVersionCheckout.useVersionCheckout({
        api,
        busy: checkingOut,
        activeModal,
        t,
        canStart: () => activeTab.value === "files" && activeProjectId.value && !activeModal.value && !versionLoading.value && selectedVersion.value && selectedVersion.value.version_id === selectedVersionId.value,
        canConfirm: (target) => activeTab.value === "files" && activeProjectId.value === target.projectId,
        getTarget: () => ({
          projectId: activeProjectId.value,
          versionId: selectedVersionId.value,
          message: selectedVersion.value.message || t("unnamedVersion"),
          shortId: selectedVersion.value.short_version_id || selectedVersionId.value.slice(0, 8)
        }),
        onDirty: (target, count) => {
          if (activeProjectId.value !== target.projectId || !Number.isSafeInteger(count) || count <= 0) return;
          statusRequestController?.abort();
          statusRequestController = null;
          statusCount.value = count;
          statusLoading.value = false;
          statusFailed.value = false;
        },
        onComplete: async (target) => {
          showToast(t("checkoutDone"));
          if (activeProjectId.value === target.projectId) await refreshActiveProject();
        },
        showError: (message) => showToast(message, true)
      });
      const projectCount = computed(() => projects.value.length);
      const hasMoreVersions = computed(() => historyPage.value < historyPages.value);
      const statusDisplay = computed(() => statusCount.value === null ? t(statusLoading.value ? "statusLoading" : "statusUnknown") : String(statusCount.value));
      const currentVersionMessage = computed(
        () => activeProject.value?.current_version_message || t("unnamedVersion")
      );
      const workspaceVersionId = computed(
        () => activeProject.value?.current_version_id || activeProject.value?.head_version_id || null
      );
      const shortVersionId = computed(
        () => activeProject.value?.short_version_id || "-"
      );
      const detailTabs = computed(() => [
        { key: "files", label: t("filesTab") },
        { key: "diff", label: t("diffTab") }
      ]);
      const highlightedCode = computed(() => {
        const raw = fileContentRaw.value;
        if (!raw) return "";
        if (raw.length > MAX_HIGHLIGHT_CHARS) return escapeHtml(raw);
        if (highlighterReady.value && window.hljs) {
          try {
            return window.hljs.highlightAuto(raw).value;
          } catch {
          }
        }
        return escapeHtml(raw);
      });
      const activeDiffItem = computed(() => {
        if (!diffData.value || !activeDiffPath.value) return null;
        const summary = (diffData.value.files || []).find((e) => e.path === activeDiffPath.value) || null;
        if (!summary || activeDiffDetail.value?.path !== activeDiffPath.value) return summary;
        return { ...summary, ...activeDiffDetail.value };
      });
      const diffMerge = window.PPGitDiffMerge.useDiffMerge({
        item: activeDiffItem,
        loading: diffFileLoading,
        language,
        api,
        showToast,
        getTarget: () => ({
          projectId: activeProjectId.value,
          fromVersion: orderedCompareIdsByAge()[0],
          toVersion: orderedCompareIdsByAge()[1]
        }),
        onSaved: async (data, target) => {
          clearProjectDiffCache(target.projectId);
          if (activeProjectId.value !== target.projectId) return;
          await refreshActiveProject({ skipMergeGuard: true });
          await loadDiff({ preservePath: target.path });
        }
      });
      const displayedDiffRows = computed(() => {
        const rows = diffMerge.rows.value;
        if (diffViewMode.value === "all") return rows;
        return rows.filter((row) => row.status !== "same");
      });
      const changedPathsMap = computed(() => {
        const map = /* @__PURE__ */ new Map();
        const files = selectedVersion.value?.files;
        if (!files) return map;
        for (const f of files) map.set(f.path, f.status);
        return map;
      });
      const changedDirPaths = computed(() => {
        const dirs = /* @__PURE__ */ new Set();
        const files = selectedVersion.value?.files;
        if (!files) return dirs;
        for (const f of files) {
          const parts = f.path.split("/");
          let cur = "";
          for (let i = 0; i < parts.length - 1; i++) {
            cur = cur ? cur + "/" + parts[i] : parts[i];
            dirs.add(cur);
          }
        }
        return dirs;
      });
      const showPlaceholder = computed(() => activeTab.value === "files" && !fileContentLoaded.value);
      const showFileContent = computed(
        () => activeTab.value === "files" && fileContentLoaded.value
      );
      const hasDiffPatch = computed(() => {
        const item = activeDiffItem.value;
        return item && !item.too_large && Array.isArray(item.diff_rows) && item.diff_rows.length > 0;
      });
      const commitScanPercent = computed(() => {
        if (!commitScanTotal.value) return 0;
        return Math.round(commitScanProcessed.value / commitScanTotal.value * 100);
      });
      const displayedCommitFiles = computed(
        () => commitFiles.value.slice(0, commitFileDisplayLimit.value)
      );
      const remainingCommitFiles = computed(
        () => Math.max(commitFiles.value.length - displayedCommitFiles.value.length, 0)
      );
      function formatDate(value) {
        return formatDateValue(value, language.value);
      }
      function statusLabel(status) {
        return statusLabelValue(status, language.value);
      }
      function localizeMessage(message) {
        const key = BACKEND_MESSAGE_KEYS[message];
        return key ? t(key) : message;
      }
      function diffFileStatusClass(status) {
        return status ? `diff-file-${status}` : "";
      }
      function diffCellClass(row, side) {
        return diffCellClassValue(row, side);
      }
      function diffVersionLabel(version, fallbackKey) {
        if (!version) return t(fallbackKey === "from" ? "fromVersion" : "toVersion");
        const message = version.message || t("unnamedVersion");
        const shortId = version.short_version_id || String(version.version_id || "").slice(0, 8);
        return `${t(fallbackKey === "from" ? "fromVersion" : "toVersion")} · ${message} · ${shortId}`;
      }
      function setDiffViewMode(mode) {
        diffViewMode.value = mode === "all" ? "all" : "changed";
      }
      function updateDefaultInspectorText() {
        if (!selectedVersion.value && !fileContentRaw.value) {
          detailTitle.value = t("versionDetail");
          detailMeta.value = t("noVersionSelected");
        }
      }
      function updateSelectedVersionMeta() {
        if (!selectedVersion.value || detailTitle.value !== selectedVersion.value.message) return;
        detailMeta.value = [
          selectedVersion.value.short_version_id,
          formatDate(selectedVersion.value.timestamp),
          t("fileCount", { count: selectedVersion.value.changed_files })
        ].join(" · ");
      }
      watch(language, () => {
        updateDefaultInspectorText();
        updateSelectedVersionMeta();
        if (fileContentMessageKey.value) {
          const message = t(fileContentMessageKey.value);
          fileContentRaw.value = message;
          detailMeta.value = message;
        }
      });
      let toastTimer = null;
      function showToast(message, isError = false) {
        toastMessage.value = message;
        toastIsError.value = isError;
        toastVisible.value = true;
        clearTimeout(toastTimer);
        toastTimer = setTimeout(() => {
          toastVisible.value = false;
        }, 2600);
      }
      function openModal(name) {
        if (!diffMerge.guard(() => openModal(name))) return;
        if (name === "project") projectError.value = "";
        activeModal.value = name;
      }
      function closeModal() {
        if (submittingProject.value || submittingCommit.value || submittingDelete.value || browsingFolder.value) return;
        if (activeModal.value === "checkout-version") {
          checkout.cancel();
          return;
        }
        commitScanRequestController?.abort();
        commitScanRequestController = null;
        activeModal.value = null;
        pendingDeleteVersion.value = null;
        deleteVersionError.value = "";
        commitScanId.value = null;
      }
      watch(projectPathInput, () => {
        projectError.value = "";
      });
      function setActiveTab(tab) {
        if (tab !== "files" && tab !== "diff") return;
        if (tab !== activeTab.value && !diffMerge.guard(() => setActiveTab(tab))) return;
        if (tab !== "files") abortFileRequest();
        if (tab !== "diff") abortDiffRequests();
        activeTab.value = tab;
        if (tab === "diff" && compareIds.value.length === 2 && !diffData.value && !diffSummaryRequestController) {
          void loadDiff();
        } else if (tab === "diff" && diffData.value && activeDiffPath.value && !activeDiffDetail.value && !diffFileRequestController) {
          void selectDiffFile(activeDiffPath.value);
        }
      }
      function abortFileRequest() {
        fileRequestEpoch += 1;
        const controller = fileRequestController;
        fileRequestController = null;
        controller?.abort();
      }
      function abortDiffFileRequest() {
        diffFileRequestEpoch += 1;
        const controller = diffFileRequestController;
        diffFileRequestController = null;
        controller?.abort();
        diffFileLoading.value = false;
      }
      function abortDiffRequests() {
        diffSummaryRequestEpoch += 1;
        const controller = diffSummaryRequestController;
        diffSummaryRequestController = null;
        controller?.abort();
        abortDiffFileRequest();
      }
      function cancelProjectRequests() {
        projectRequestController?.abort();
        historyRequestController?.abort();
        statusRequestController?.abort();
        versionRequestController?.abort();
        projectRequestController = null;
        historyRequestController = null;
        statusRequestController = null;
        versionRequestController = null;
        abortFileRequest();
        abortDiffRequests();
      }
      function resetProjectSelectionState() {
        activeProject.value = null;
        versions.value = [];
        historyPage.value = 0;
        historyPages.value = 0;
        historyTotal.value = 0;
        historyLoading.value = false;
        selectedVersionId.value = null;
        selectedVersion.value = null;
        versionLoading.value = false;
        detailTree.value = [];
        compareIds.value = [];
        diffData.value = null;
        activeDiffDetail.value = null;
        activeDiffPath.value = null;
        diffPairKey.value = null;
        diffFileLoading.value = false;
        fileContentRaw.value = "";
        fileContentLoaded.value = false;
        fileContentMessageKey.value = null;
        statusCount.value = null;
        statusLoading.value = false;
        statusFailed.value = false;
        detailTitle.value = t("versionDetail");
        detailMeta.value = t("noVersionSelected");
        setActiveTab("files");
      }
      async function loadProjects(autoSelect = true) {
        const data = await api("/api/projects");
        projects.value = data.projects || [];
        if (activeProjectId.value && !projects.value.some((p) => p.project_id === activeProjectId.value)) {
          cancelProjectRequests();
          activeProjectId.value = null;
          resetProjectSelectionState();
        }
        if (autoSelect && !activeProjectId.value && projects.value.length) {
          await selectProject(projects.value[0].project_id);
        }
      }
      async function selectProject(projectId) {
        if (submittingCommit.value) return;
        if (!diffMerge.guard(() => selectProject(projectId))) return;
        cancelProjectRequests();
        activeProjectId.value = projectId;
        resetProjectSelectionState();
        await refreshActiveProject();
      }
      async function removeProject(project) {
        if (!project?.project_id) return;
        if (!diffMerge.guard(() => removeProject(project))) return;
        if (!confirm(t("removeProjectConfirm", { name: project.name }))) return;
        try {
          await api(`/api/projects/${project.project_id}`, { method: "DELETE" });
          clearProjectDiffCache(project.project_id);
          if (activeProjectId.value === project.project_id) {
            cancelProjectRequests();
            activeProjectId.value = null;
            resetProjectSelectionState();
          }
          await loadProjects();
          showToast(t("projectRemoved"));
        } catch (error) {
          showToast(error.message, true);
        }
      }
      async function refreshActiveProject(options = {}) {
        if (!activeProjectId.value) return;
        if (!options.skipMergeGuard && !diffMerge.guard(() => refreshActiveProject(options))) return;
        cancelProjectRequests();
        const pid = activeProjectId.value;
        const controller = new AbortController();
        projectRequestController = controller;
        const pageCount = Math.max(historyPage.value, Math.ceil(versions.value.length / HISTORY_PAGE_SIZE), 1);
        historyLoading.value = true;
        void refreshStatus(pid);
        try {
          const [projectData, ...historyData] = await Promise.all([
            api(`/api/projects/${pid}`, { signal: controller.signal }),
            ...Array.from({ length: pageCount }, (_, index) => api(
              `/api/projects/${pid}/versions?page=${index + 1}&per_page=${HISTORY_PAGE_SIZE}`,
              { signal: controller.signal }
            ))
          ]);
          if (controller.signal.aborted || activeProjectId.value !== pid) return;
          activeProject.value = projectData.project;
          versions.value = historyData.flatMap((page) => page.items || []);
          historyTotal.value = historyData[0].total ?? versions.value.length;
          historyPages.value = historyData[0].pages ?? 1;
          historyPage.value = Math.min(pageCount, historyPages.value);
          if (!selectedVersionId.value && versions.value.length) {
            await selectVersion(versions.value[0].version_id, options);
          } else if (selectedVersionId.value) {
            await selectVersion(selectedVersionId.value, options);
          }
          if (!controller.signal.aborted && activeProjectId.value === pid && activeTab.value === "diff") {
            setActiveTab("diff");
          }
        } catch (error) {
          if (!isAbortError(error)) showToast(error.message, true);
        } finally {
          if (projectRequestController === controller) {
            projectRequestController = null;
            historyLoading.value = false;
          }
        }
      }
      let initialized = false;
      function canRefreshSharedMetadata() {
        return initialized && !activeModal.value && !pendingWriteRequests.value && !diffMerge.dirty.value && !diffMerge.saving.value && !submittingProject.value && !submittingCommit.value && !checkingOut.value && !historyLoading.value && !statusLoading.value && !versionLoading.value && !fileRequestController && !diffSummaryRequestController && !diffFileRequestController;
      }
      async function refreshSharedMetadata(signal) {
        const projectId = activeProjectId.value;
        const previousVersions = versions.value;
        const previousTotal = historyTotal.value;
        const previousPage = historyPage.value;
        const requestEpoch = writeRequestEpoch;
        const list = await api("/api/projects", { signal });
        const latestProjects = list.projects || [];
        const stillCurrent = () => !signal.aborted && canRefreshSharedMetadata() && activeProjectId.value === projectId && versions.value === previousVersions && requestEpoch === writeRequestEpoch;
        if (!stillCurrent()) return false;
        if (!projectId || !latestProjects.some((project) => project.project_id === projectId)) {
          projects.value = latestProjects;
          if (projectId) {
            cancelProjectRequests();
            clearProjectDiffCache(projectId);
            activeProjectId.value = null;
            resetProjectSelectionState();
          }
          return true;
        }
        const [projectData, firstPage] = await Promise.all([
          api(`/api/projects/${projectId}`, { signal }),
          api(`/api/projects/${projectId}/versions?page=1&per_page=${HISTORY_PAGE_SIZE}`, { signal })
        ]);
        const growth = Math.max((firstPage.total ?? previousTotal) - previousTotal, 0);
        const pageCount = Math.min(firstPage.pages ?? 1, Math.max(
          previousPage,
          Math.ceil((previousVersions.length + growth) / HISTORY_PAGE_SIZE),
          1
        ));
        const otherPages = await Promise.all(Array.from({ length: Math.max(pageCount - 1, 0) }, (_, index) => api(`/api/projects/${projectId}/versions?page=${index + 2}&per_page=${HISTORY_PAGE_SIZE}`, { signal })));
        const latestVersions = [firstPage, ...otherPages].flatMap((page) => page.items || []);
        const loadedIds = new Set(latestVersions.map((item) => item.version_id));
        const selectedIds = new Set([selectedVersionId.value, ...compareIds.value].filter(Boolean));
        const deletedIds = [];
        for (const id of selectedIds) {
          if (loadedIds.has(id)) continue;
          try {
            await api(`/api/projects/${projectId}/versions/${encodeURIComponent(id)}`, { signal });
          } catch (error) {
            if (error.status === 404) deletedIds.push(id);
            else throw error;
          }
        }
        if (!stillCurrent()) return false;
        const workspaceChanged = activeProject.value?.current_version_id !== projectData.project?.current_version_id;
        const previousTrees = new Map(previousVersions.map((version) => [version.version_id, version.tree_id]));
        const contentChanged = latestVersions.some((version) => previousTrees.has(version.version_id) && previousTrees.get(version.version_id) && version.tree_id && previousTrees.get(version.version_id) !== version.tree_id);
        projects.value = latestProjects;
        activeProject.value = projectData.project;
        versions.value = latestVersions;
        historyTotal.value = firstPage.total ?? latestVersions.length;
        historyPages.value = firstPage.pages ?? 1;
        historyPage.value = pageCount;
        for (const id of deletedIds) clearDeletedVersionState(id);
        if (deletedIds.length) clearProjectDiffCache(projectId);
        if (workspaceChanged) {
          statusCount.value = null;
          statusFailed.value = false;
        }
        if (contentChanged) {
          clearProjectDiffCache(projectId);
          statusCount.value = null;
          statusFailed.value = false;
          if (activeTab.value === "diff" && compareIds.value.length === 2) {
            const preservePath = activeDiffPath.value;
            activeDiffDetail.value = null;
            await loadDiff({ preservePath });
          } else if (selectedVersionId.value) {
            const filePath = fileContentLoaded.value ? detailTitle.value : null;
            await selectVersion(selectedVersionId.value);
            if (filePath) await loadFileContent(filePath);
          }
        }
        return true;
      }
      const sharedMetadata = window.useServerSync({
        api,
        canRefresh: canRefreshSharedMetadata,
        refresh: refreshSharedMetadata
      });
      async function loadMoreVersions() {
        if (!activeProjectId.value || historyLoading.value || !hasMoreVersions.value) return;
        const projectId = activeProjectId.value;
        const page = historyPage.value + 1;
        const controller = new AbortController();
        historyRequestController?.abort();
        historyRequestController = controller;
        historyLoading.value = true;
        try {
          const data = await api(
            `/api/projects/${projectId}/versions?page=${page}&per_page=${HISTORY_PAGE_SIZE}`,
            { signal: controller.signal }
          );
          if (controller.signal.aborted || activeProjectId.value !== projectId) return;
          const seen = new Set(versions.value.map((version) => version.version_id));
          versions.value = [...versions.value, ...(data.items || []).filter((version) => !seen.has(version.version_id))];
          historyPage.value = data.page ?? page;
          historyPages.value = data.pages ?? historyPages.value;
          historyTotal.value = data.total ?? versions.value.length;
        } catch (error) {
          if (!isAbortError(error)) showToast(error.message, true);
        } finally {
          if (historyRequestController === controller) {
            historyRequestController = null;
            historyLoading.value = false;
          }
        }
      }
      async function refreshStatus(projectId) {
        statusRequestController?.abort();
        const controller = new AbortController();
        statusRequestController = controller;
        statusCount.value = null;
        statusLoading.value = true;
        statusFailed.value = false;
        try {
          const data = await api(`/api/projects/${projectId}/status`, { signal: controller.signal });
          if (controller.signal.aborted || activeProjectId.value !== projectId) return;
          if (!Number.isFinite(data.count)) throw new Error(t("requestFailed"));
          statusCount.value = data.count;
        } catch (error) {
          if (!isAbortError(error) && activeProjectId.value === projectId && statusRequestController === controller) {
            statusCount.value = null;
            statusFailed.value = true;
          }
        } finally {
          if (statusRequestController === controller) {
            statusRequestController = null;
            statusLoading.value = false;
          }
        }
      }
      async function selectVersion(versionId, options = {}) {
        if (!activeProjectId.value) return;
        if (!options.skipMergeGuard && !diffMerge.guard(() => selectVersion(versionId, options))) return;
        versionRequestController?.abort();
        abortFileRequest();
        abortDiffRequests();
        const controller = new AbortController();
        versionRequestController = controller;
        const projectId = activeProjectId.value;
        selectedVersionId.value = versionId;
        selectedVersion.value = null;
        versionLoading.value = true;
        detailTree.value = [];
        fileContentRaw.value = "";
        fileContentLoaded.value = false;
        fileContentMessageKey.value = null;
        detailTitle.value = t("loadingVersions");
        detailMeta.value = "";
        try {
          const [detailData, treeData] = await Promise.all([
            api(`/api/projects/${projectId}/versions/${versionId}`, { signal: controller.signal }),
            api(`/api/projects/${projectId}/tree/${versionId}`, { signal: controller.signal })
          ]);
          if (controller.signal.aborted || activeProjectId.value !== projectId || selectedVersionId.value !== versionId) return;
          selectedVersion.value = detailData.version;
          detailTree.value = treeData.tree || [];
          detailTitle.value = selectedVersion.value.message;
          detailMeta.value = [
            selectedVersion.value.short_version_id,
            formatDate(selectedVersion.value.timestamp),
            t("fileCount", { count: selectedVersion.value.changed_files })
          ].join(" · ");
          fileContentRaw.value = "";
          fileContentMessageKey.value = null;
        } catch (error) {
          if (!isAbortError(error)) {
            if (versionRequestController === controller) {
              detailTitle.value = t("versionDetail");
              detailMeta.value = t("requestFailed");
            }
            showToast(error.message, true);
          }
        } finally {
          if (versionRequestController === controller) {
            versionRequestController = null;
            versionLoading.value = false;
          }
        }
      }
      async function loadFileContent(filePath) {
        if (activeTab.value !== "files" || !activeProjectId.value || !selectedVersionId.value || versionLoading.value) return;
        abortDiffRequests();
        abortFileRequest();
        const requestEpoch = fileRequestEpoch;
        const controller = new AbortController();
        fileRequestController = controller;
        const projectId = activeProjectId.value;
        const versionId = selectedVersionId.value;
        try {
          const data = await api(
            `/api/projects/${projectId}/file/${versionId}?path=${encodeURIComponent(filePath)}`,
            { signal: controller.signal }
          );
          if (controller.signal.aborted || requestEpoch !== fileRequestEpoch || fileRequestController !== controller || activeProjectId.value !== projectId || selectedVersionId.value !== versionId) return;
          detailTitle.value = filePath;
          fileContentLoaded.value = true;
          if (data.message) {
            fileContentMessageKey.value = BACKEND_MESSAGE_KEYS[data.message] || null;
            const message = localizeMessage(data.message);
            fileContentRaw.value = message;
            detailMeta.value = message;
          } else {
            fileContentMessageKey.value = null;
            const content = data.content || "";
            fileContentRaw.value = content;
            detailMeta.value = `${formatBytes(data.size || 0)} · ${data.mime_type || "text/plain"}`;
            if (content.length <= MAX_HIGHLIGHT_CHARS) {
              ensureHighlightAssets().then((loaded) => {
                highlighterReady.value = loaded;
              });
            }
          }
        } catch (error) {
          if (!isAbortError(error)) showToast(error.message, true);
        } finally {
          if (fileRequestController === controller) fileRequestController = null;
        }
      }
      async function handleTimelineVersionClick(versionId) {
        if (activeTab.value === "diff") {
          await toggleCompare(versionId);
          return;
        }
        await selectVersion(versionId);
      }
      async function toggleCompare(versionId, event) {
        if (event?.target?.type === "checkbox") event.target.checked = compareIds.value.includes(versionId);
        if (!diffMerge.guard(() => toggleCompare(versionId))) return;
        const idx = compareIds.value.indexOf(versionId);
        if (idx >= 0) {
          compareIds.value = compareIds.value.filter((id) => id !== versionId);
        } else {
          if (compareIds.value.length >= 2) {
            showToast(t("maxCompareVersions"), true);
            return;
          }
          compareIds.value = [...compareIds.value, versionId];
        }
        if (compareIds.value.length === 2) {
          compareIds.value = orderedCompareIdsByAge();
          await loadDiff();
        } else {
          abortDiffRequests();
          diffData.value = null;
          activeDiffDetail.value = null;
          activeDiffPath.value = null;
          diffPairKey.value = null;
          diffFileLoading.value = false;
        }
      }
      function orderedCompareIdsByAge() {
        const newestFirstIndex = new Map(versions.value.map((version, index) => [version.version_id, index]));
        return [...compareIds.value].sort(
          (left, right) => (newestFirstIndex.get(right) ?? Number.MAX_SAFE_INTEGER) - (newestFirstIndex.get(left) ?? Number.MAX_SAFE_INTEGER)
        );
      }
      async function loadDiff(options = {}) {
        const [left, right] = orderedCompareIdsByAge();
        if (!left || !right || !activeProjectId.value) return;
        abortFileRequest();
        abortDiffRequests();
        const requestEpoch = diffSummaryRequestEpoch;
        const controller = new AbortController();
        diffSummaryRequestController = controller;
        const projectId = activeProjectId.value;
        const pairKey = `${projectId}\0${left}\0${right}`;
        const query = new URLSearchParams({
          from: left,
          to: right,
          details: "0",
          patch: "0"
        });
        try {
          const data = await api(`/api/projects/${projectId}/diff?${query}`, {
            signal: controller.signal
          });
          if (controller.signal.aborted || requestEpoch !== diffSummaryRequestEpoch || diffSummaryRequestController !== controller || activeProjectId.value !== projectId || orderedCompareIdsByAge().join("\0") !== [left, right].join("\0")) return;
          diffData.value = data;
          activeDiffDetail.value = null;
          activeDiffPath.value = null;
          diffPairKey.value = pairKey;
          setActiveTab("diff");
          const firstPath = (data.files || []).some((file) => file.path === options.preservePath) ? options.preservePath : data.files?.[0]?.path;
          if (firstPath) await selectDiffFile(firstPath, { skipMergeGuard: true });
        } catch (error) {
          if (!isAbortError(error)) showToast(error.message, true);
        } finally {
          if (diffSummaryRequestController === controller) {
            diffSummaryRequestController = null;
          }
        }
      }
      function removeDiffCacheEntry(key) {
        const entry = diffFileCache.get(key);
        if (!entry) return;
        diffFileCacheBytes -= entry.bytes;
        diffFileCacheRows -= entry.rows;
        diffFileCache.delete(key);
      }
      function clearProjectDiffCache(projectId) {
        const prefix = `${projectId}\0`;
        for (const key of diffFileCache.keys()) {
          if (key.startsWith(prefix)) removeDiffCacheEntry(key);
        }
      }
      function cacheDiffFile(key, item) {
        removeDiffCacheEntry(key);
        const rows = item.diff_rows || [];
        const bytes = rows.reduce(
          (total, row) => total + 180 + 2 * (String(row.left_text || "").length + String(row.right_text || "").length),
          512 + 2 * String(item.patch || "").length
        );
        if (bytes > DIFF_FILE_CACHE_BYTES || rows.length > DIFF_FILE_CACHE_ROWS) return;
        diffFileCache.set(key, { item, bytes, rows: rows.length });
        diffFileCacheBytes += bytes;
        diffFileCacheRows += rows.length;
        while (diffFileCache.size > DIFF_FILE_CACHE_LIMIT || diffFileCacheBytes > DIFF_FILE_CACHE_BYTES || diffFileCacheRows > DIFF_FILE_CACHE_ROWS) {
          removeDiffCacheEntry(diffFileCache.keys().next().value);
        }
      }
      async function selectDiffFile(path, options = {}) {
        if (path === activeDiffPath.value && activeDiffDetail.value && !options.skipMergeGuard) return;
        if (!options.skipMergeGuard && !diffMerge.guard(() => selectDiffFile(path, options))) return;
        activeDiffPath.value = path;
        activeDiffDetail.value = null;
        abortDiffFileRequest();
        const summary = (diffData.value?.files || []).find((file) => file.path === path);
        if (!summary) return;
        if (summary.too_large || summary.message) {
          activeDiffDetail.value = summary;
          diffFileLoading.value = false;
          return;
        }
        const [left, right] = orderedCompareIdsByAge();
        const projectId = activeProjectId.value;
        const pairKey = `${projectId}\0${left}\0${right}`;
        if (!projectId || diffPairKey.value !== pairKey) return;
        const cacheKey = `${pairKey}\0${path}`;
        const cached = diffFileCache.get(cacheKey);
        if (cached) {
          diffFileCache.delete(cacheKey);
          diffFileCache.set(cacheKey, cached);
          activeDiffDetail.value = cached.item;
          diffFileLoading.value = false;
          return;
        }
        const requestEpoch = diffFileRequestEpoch;
        const controller = new AbortController();
        diffFileRequestController = controller;
        diffFileLoading.value = true;
        const query = new URLSearchParams({
          from: left,
          to: right,
          path,
          details: "1",
          patch: "0"
        });
        try {
          const data = await api(`/api/projects/${projectId}/diff?${query}`, {
            signal: controller.signal
          });
          if (controller.signal.aborted || requestEpoch !== diffFileRequestEpoch || diffFileRequestController !== controller || activeProjectId.value !== projectId || diffPairKey.value !== pairKey || activeDiffPath.value !== path) return;
          const detail = data.file || (data.files || []).find((file) => file.path === path) || null;
          if (detail) {
            cacheDiffFile(cacheKey, detail);
            activeDiffDetail.value = detail;
          }
        } catch (error) {
          if (!isAbortError(error)) showToast(error.message, true);
        } finally {
          if (diffFileRequestController === controller) {
            diffFileRequestController = null;
            diffFileLoading.value = false;
          }
        }
      }
      function clearDeletedVersionState(versionId) {
        const wasSelected = selectedVersionId.value === versionId;
        const wasCompared = compareIds.value.includes(versionId);
        if (wasSelected) {
          selectedVersionId.value = null;
          selectedVersion.value = null;
          detailTree.value = [];
          detailTitle.value = t("versionDetail");
          detailMeta.value = t("noVersionSelected");
          fileContentRaw.value = "";
          fileContentLoaded.value = false;
          fileContentMessageKey.value = null;
        }
        if (wasCompared) {
          abortDiffRequests();
          compareIds.value = compareIds.value.filter((id) => id !== versionId);
          diffData.value = null;
          activeDiffDetail.value = null;
          activeDiffPath.value = null;
          diffPairKey.value = null;
          diffFileLoading.value = false;
          if (activeTab.value === "diff") setActiveTab("files");
        }
      }
      function deleteVersion(versionId) {
        if (!activeProjectId.value || !versionId || activeModal.value || submittingDelete.value || checkingOut.value) return;
        if (!diffMerge.guard(() => deleteVersion(versionId))) return;
        const version = versions.value.find((item) => item.version_id === versionId);
        pendingDeleteVersion.value = {
          projectId: activeProjectId.value,
          versionId,
          message: version?.message || t("unnamedVersion"),
          shortId: version?.short_version_id || String(versionId).slice(0, 8)
        };
        deleteVersionError.value = "";
        activeModal.value = "delete-version";
      }
      async function confirmDeleteVersion() {
        if (activeModal.value !== "delete-version" || !pendingDeleteVersion.value || submittingDelete.value) return;
        const { projectId, versionId } = pendingDeleteVersion.value;
        submittingDelete.value = true;
        deleteVersionError.value = "";
        try {
          const data = await api(
            `/api/projects/${projectId}/versions/${versionId}`,
            { method: "DELETE" }
          );
          if (activeProjectId.value === projectId) clearDeletedVersionState(versionId);
          const cleanup = data.cleanup || {};
          activeModal.value = null;
          pendingDeleteVersion.value = null;
          showToast(t("versionDeletedFromDisk", {
            trees: cleanup.deleted_trees || 0,
            objects: cleanup.deleted_objects || 0,
            size: formatBytes(cleanup.freed_bytes || 0)
          }));
          if (activeProjectId.value === projectId) await refreshActiveProject();
        } catch (error) {
          deleteVersionError.value = error.message;
        } finally {
          submittingDelete.value = false;
        }
      }
      async function createVersion() {
        if (!activeProjectId.value || submittingCommit.value || checkingOut.value) return;
        if (!diffMerge.guard(() => createVersion())) return;
        if (!directCreate.value) {
          await openCommitModal();
          return;
        }
        await saveVersion(nextCommitMessage(), null);
      }
      function nextCommitMessage() {
        if (activeProject.value?.next_version_message) {
          return activeProject.value.next_version_message;
        }
        const versionCount = activeProject.value?.version_count ?? versions.value.length;
        const maxDefaultNumber = versions.value.reduce((maximum, version) => {
          const match = /^V(\d+)$/i.exec(String(version.message || "").trim());
          return match ? Math.max(maximum, Number(match[1])) : maximum;
        }, 0);
        const nextNumber = Math.max(versionCount, maxDefaultNumber + 1);
        return versionCount ? `V${nextNumber}` : "initial";
      }
      function delay(milliseconds) {
        return new Promise((resolve) => setTimeout(resolve, milliseconds));
      }
      function showMoreCommitFiles() {
        commitFileDisplayLimit.value += COMMIT_FILES_PAGE_SIZE;
      }
      async function toggleCommitFileDetails() {
        if (commitFileDetailsVisible.value) {
          commitFileDetailsVisible.value = false;
          return;
        }
        commitFileDetailsVisible.value = true;
        if (commitFiles.value.length === commitFileCount.value) return;
        const projectId = activeProjectId.value;
        const scanId = commitScanId.value;
        if (!projectId || !scanId) {
          commitFileDetailsVisible.value = false;
          return;
        }
        commitFileDetailsLoading.value = true;
        try {
          const status = await api(
            `/api/projects/${projectId}/status-scans/${scanId}?include_files=1`,
            commitScanRequestController ? { signal: commitScanRequestController.signal } : {}
          );
          if (activeModal.value !== "commit" || commitScanId.value !== scanId) return;
          if (status.state === "failed") throw new Error(status.error || t("requestFailed"));
          commitFiles.value = status.files || [];
          commitFileCount.value = status.count ?? commitFiles.value.length;
          commitFilesEmpty.value = !commitFileCount.value;
        } catch (error) {
          commitFileDetailsVisible.value = false;
          if (!isAbortError(error)) showToast(error.message, true);
        } finally {
          if (activeModal.value === "commit" && commitScanId.value === scanId) {
            commitFileDetailsLoading.value = false;
          }
        }
      }
      async function openCommitModal() {
        if (!activeProjectId.value) return;
        commitScanRequestController?.abort();
        const controller = new AbortController();
        commitScanRequestController = controller;
        openModal("commit");
        commitMessage.value = nextCommitMessage();
        commitFiles.value = [];
        commitFileCount.value = 0;
        commitFilesLoading.value = true;
        commitFilesEmpty.value = false;
        commitScanFailed.value = false;
        commitScanId.value = null;
        commitFileDetailsVisible.value = false;
        commitFileDetailsLoading.value = false;
        commitScanProcessed.value = 0;
        commitScanTotal.value = 0;
        commitFileDisplayLimit.value = COMMIT_FILES_PAGE_SIZE;
        const projectId = activeProjectId.value;
        try {
          const started = await api(`/api/projects/${projectId}/status-scans`, {
            method: "POST",
            signal: controller.signal
          });
          if (controller.signal.aborted || commitScanRequestController !== controller || activeModal.value !== "commit" || activeProjectId.value !== projectId) return;
          const scanId = started.scan_id;
          commitScanId.value = scanId;
          let status = started;
          let pollDelay = 250;
          while (status.state === "running" && activeModal.value === "commit" && commitScanId.value === scanId) {
            commitScanProcessed.value = status.processed || 0;
            commitScanTotal.value = status.total || 0;
            const previousProcessed = status.processed || 0;
            await delay(pollDelay);
            if (controller.signal.aborted || activeModal.value !== "commit" || commitScanId.value !== scanId) return;
            status = await api(`/api/projects/${projectId}/status-scans/${scanId}`, {
              signal: controller.signal
            });
            pollDelay = (status.processed || 0) === previousProcessed ? Math.min(pollDelay + 100, 500) : 250;
          }
          if (activeModal.value !== "commit" || commitScanId.value !== scanId) return;
          if (status.state === "failed") throw new Error(status.error || t("requestFailed"));
          commitScanProcessed.value = status.processed || 0;
          commitScanTotal.value = status.total || 0;
          commitFileCount.value = status.count || 0;
          commitFilesEmpty.value = !commitFileCount.value;
        } catch (error) {
          if (!isAbortError(error)) {
            if (commitScanRequestController === controller) commitScanFailed.value = true;
            showToast(error.message, true);
          }
        } finally {
          if (commitScanRequestController === controller && activeModal.value === "commit") {
            commitFilesLoading.value = false;
          }
        }
      }
      async function submitCommit() {
        if (submittingCommit.value || commitFilesLoading.value || commitFilesEmpty.value || commitScanFailed.value) return;
        const msg = commitMessage.value.trim();
        if (!msg) {
          showToast(t("enterVersionMessage"), true);
          return;
        }
        await saveVersion(msg, commitScanId.value);
      }
      async function saveVersion(message, scanId) {
        if (!activeProjectId.value || submittingCommit.value) return;
        submittingCommit.value = true;
        const projectId = activeProjectId.value;
        const hadSelectedVersion = Boolean(selectedVersionId.value);
        try {
          const data = await api(`/api/projects/${projectId}/versions`, {
            method: "POST",
            body: JSON.stringify({ message, scan_id: scanId })
          });
          if (data.project && activeProjectId.value === projectId) activeProject.value = data.project;
          if (data.version && activeProjectId.value === projectId) {
            const { files: _files, ...versionSummary } = data.version;
            versions.value = [
              versionSummary,
              ...versions.value.filter((version) => version.version_id !== data.version.version_id)
            ];
            historyTotal.value += 1;
            historyPages.value = Math.ceil(historyTotal.value / HISTORY_PAGE_SIZE);
          }
          commitScanRequestController?.abort();
          commitScanRequestController = null;
          commitScanId.value = null;
          activeModal.value = null;
          showToast(t("versionCreated"));
          if (!hadSelectedVersion && data.version && activeProjectId.value === projectId) {
            await selectVersion(data.version.version_id);
          }
          if (activeProjectId.value === projectId) void refreshStatus(projectId);
        } catch (error) {
          const noChanges = error.message === "there are no changes to save";
          showToast(noChanges ? t("noCommitChanges") : error.message, !noChanges);
        } finally {
          submittingCommit.value = false;
        }
      }
      async function browseFolder() {
        if (browsingFolder.value || submittingProject.value) return;
        browsingFolder.value = true;
        projectError.value = "";
        try {
          const data = await api("/api/browse-folder", { method: "POST" });
          if (data.path) {
            projectPathInput.value = data.path;
            projectError.value = "";
          }
        } catch (error) {
          projectError.value = error.message;
          showToast(error.message, true);
        } finally {
          browsingFolder.value = false;
        }
      }
      async function submitProject() {
        if (submittingProject.value || browsingFolder.value) return;
        const path = projectPathInput.value.trim();
        projectError.value = "";
        if (!path) {
          projectError.value = t("enterProjectPath");
          return;
        }
        submittingProject.value = true;
        try {
          const data = await api("/api/projects", {
            method: "POST",
            body: JSON.stringify({ path })
          });
          activeModal.value = null;
          projectPathInput.value = "";
          showToast(t("projectAdded"));
          await loadProjects(false);
          await selectProject(data.project.project_id);
        } catch (error) {
          projectError.value = error.message;
          showToast(error.message, true);
        } finally {
          submittingProject.value = false;
        }
      }
      onMounted(async () => {
        try {
          await loadProjects();
        } catch (error) {
          showToast(error.message, true);
        } finally {
          initialized = true;
          void sharedMetadata.check();
        }
      });
      onBeforeUnmount(() => {
        cancelProjectRequests();
        commitScanRequestController?.abort();
        clearTimeout(toastTimer);
      });
      return {
        language,
        setLanguage,
        t,
        projects,
        activeProjectId,
        activeProject,
        versions,
        selectedVersionId,
        selectedVersion,
        versionLoading,
        checkingOut,
        historyLoading,
        historyTotal,
        hasMoreVersions,
        loadMoreVersions,
        detailTree,
        compareIds,
        diffData,
        activeDiffPath,
        diffViewMode,
        activeTab,
        statusCount,
        statusLoading,
        statusFailed,
        statusDisplay,
        fileContentRaw,
        highlighterReady,
        toastMessage,
        toastVisible,
        toastIsError,
        activeModal,
        projectPathInput,
        projectError,
        commitMessage,
        directCreate,
        commitFiles,
        commitFileCount,
        commitFilesLoading,
        commitFilesEmpty,
        commitScanFailed,
        commitFileDetailsVisible,
        commitFileDetailsLoading,
        commitScanProcessed,
        commitScanTotal,
        commitScanPercent,
        displayedCommitFiles,
        remainingCommitFiles,
        submittingProject,
        submittingCommit,
        browsingFolder,
        detailTitle,
        detailMeta,
        diffFileLoading,
        projectCount,
        currentVersionMessage,
        workspaceVersionId,
        shortVersionId,
        detailTabs,
        highlightedCode,
        activeDiffItem,
        displayedDiffRows,
        mergeSelected: diffMerge.selected,
        mergeStaged: diffMerge.staged,
        mergeBatches: diffMerge.batches,
        mergeDirty: diffMerge.dirty,
        mergeSaving: diffMerge.saving,
        mergeError: diffMerge.error,
        mergeEditable: diffMerge.editable,
        mergeDisabledReason: diffMerge.disabledReason,
        mergeText: diffMerge.text,
        selectMergeRow: diffMerge.selectRow,
        applyMerge: diffMerge.apply,
        undoMerge: diffMerge.undo,
        discardMerge: diffMerge.discard,
        confirmMerge: diffMerge.confirm,
        mergeDiscardTarget: diffMerge.discardTarget,
        cancelMergeDiscard: diffMerge.cancelDiscard,
        confirmMergeDiscard: diffMerge.confirmDiscard,
        changedPathsMap,
        changedDirPaths,
        showPlaceholder,
        showFileContent,
        hasDiffPatch,
        showToast,
        openModal,
        closeModal,
        setActiveTab,
        selectProject,
        refreshActiveProject,
        selectVersion,
        handleTimelineVersionClick,
        loadFileContent,
        toggleCompare,
        selectDiffFile,
        setDiffViewMode,
        checkoutVersion: checkout.start,
        confirmCheckoutVersion: checkout.confirm,
        pendingCheckout: checkout.pending,
        checkoutError: checkout.error,
        checkoutChanged: checkout.changed,
        deleteVersion,
        confirmDeleteVersion,
        pendingDeleteVersion,
        submittingDelete,
        deleteVersionError,
        createVersion,
        openCommitModal,
        submitCommit,
        toggleCommitFileDetails,
        showMoreCommitFiles,
        browseFolder,
        submitProject,
        removeProject,
        formatDate,
        formatBytes,
        statusLabel,
        localizeMessage,
        diffFileStatusClass,
        diffCellClass,
        diffVersionLabel
      };
    }
  });
  app.component("file-tree-node", FileTreeNode);
  app.component("virtual-diff-rows", VirtualDiffRows);
  app.component("diff-merge-toolbar", window.PPGitDiffMerge.DiffMergeToolbar);
  app.component("border-glow-button", BorderGlowButton);
  app.component("version-action-dialog", window.PPGitVersionActionDialog);
  app.mount("#app");
})();
