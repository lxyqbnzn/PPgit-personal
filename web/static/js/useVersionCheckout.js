(function() {
  "use strict";
  const { shallowRef, onBeforeUnmount } = Vue;
  function useVersionCheckout({ api, busy, activeModal, canStart, canConfirm, getTarget, onDirty, onComplete, showError, t }) {
    const pending = shallowRef(null);
    const error = shallowRef("");
    const changed = shallowRef(false);
    let disposed = false;
    function cancel() {
      if (busy.value) return;
      if (activeModal.value === "checkout-version") activeModal.value = null;
      pending.value = null;
      error.value = "";
      changed.value = false;
    }
    async function run(target, token = "") {
      if (busy.value || disposed) return;
      busy.value = true;
      error.value = "";
      changed.value = false;
      try {
        await api(`/api/projects/${target.projectId}/checkout/${target.versionId}`, {
          method: "POST",
          ...token ? { body: JSON.stringify({ confirmation_token: token }) } : {}
        });
        if (disposed) return;
        pending.value = null;
        if (activeModal.value === "checkout-version") activeModal.value = null;
        await onComplete(target);
      } catch (failure) {
        if (disposed) return;
        if (failure.code === "checkout_confirmation_required" && /^[a-f0-9]{64}$/.test(failure.confirmationToken || "")) {
          onDirty(target, failure.uncommittedCount);
          pending.value = { ...target, token: failure.confirmationToken };
          changed.value = Boolean(token);
          activeModal.value = "checkout-version";
        } else {
          const message = failure.code === "checkout_workspace_changed" ? t("checkoutChanged") : failure.message;
          if (activeModal.value === "checkout-version") error.value = message;
          else showError(message);
        }
      } finally {
        busy.value = false;
      }
    }
    async function start() {
      if (!canStart() || busy.value || disposed) return;
      changed.value = false;
      await run(getTarget());
    }
    async function confirm() {
      if (activeModal.value !== "checkout-version" || !pending.value || busy.value || disposed) return;
      const target = pending.value;
      if (!canConfirm(target)) {
        cancel();
        return;
      }
      await run(target, target.token);
    }
    onBeforeUnmount(() => {
      disposed = true;
    });
    return { pending, error, changed, start, confirm, cancel };
  }
  window.PPGitVersionCheckout = { useVersionCheckout };
})();
