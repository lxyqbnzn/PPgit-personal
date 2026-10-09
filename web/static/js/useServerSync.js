(function() {
  "use strict";
  window.useServerSync = function useServerSync({ api, canRefresh, refresh, interval = 3e3 }) {
    let revision = null;
    let controller = null;
    let timer = null;
    let stopped = false;
    async function check() {
      if (stopped || controller || document.visibilityState === "hidden" || !canRefresh()) return;
      const request = new AbortController();
      controller = request;
      try {
        const data = await api("/api/revision", { signal: request.signal });
        if (request.signal.aborted || typeof data.revision !== "string") return;
        if (data.revision !== revision && canRefresh()) {
          if (await refresh(request.signal)) revision = data.revision;
        }
      } catch (error) {
      } finally {
        if (controller === request) controller = null;
      }
    }
    function schedule() {
      clearTimeout(timer);
      timer = null;
      if (stopped || document.visibilityState === "hidden") return;
      timer = setTimeout(async () => {
        await check();
        schedule();
      }, interval);
    }
    function onFocus() {
      void check();
    }
    function onVisibilityChange() {
      if (document.visibilityState === "hidden") controller?.abort();
      else void check();
      schedule();
    }
    Vue.onMounted(() => {
      window.addEventListener("focus", onFocus);
      document.addEventListener("visibilitychange", onVisibilityChange);
      schedule();
    });
    Vue.onBeforeUnmount(() => {
      stopped = true;
      controller?.abort();
      clearTimeout(timer);
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    });
    return { check };
  };
})();
