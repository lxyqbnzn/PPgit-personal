(function() {
  "use strict";
  window.usePageLeaveGuard = function usePageLeaveGuard(pending) {
    let stopWatching = null;
    function beforeUnload(event) {
      if (!pending.value) return;
      event.preventDefault();
      event.returnValue = "";
    }
    Vue.onMounted(() => {
      stopWatching = Vue.watch(pending, (value) => {
        if (value) window.addEventListener("beforeunload", beforeUnload);
        else window.removeEventListener("beforeunload", beforeUnload);
      }, { immediate: true, flush: "sync" });
    });
    Vue.onBeforeUnmount(() => {
      stopWatching?.();
      window.removeEventListener("beforeunload", beforeUnload);
    });
  };
})();
