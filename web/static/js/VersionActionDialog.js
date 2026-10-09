(function() {
  "use strict";
  const { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } = Vue;
  window.PPGitVersionActionDialog = {
    props: {
      kind: { type: String, default: "delete" },
      target: { type: Object, required: true },
      busy: Boolean,
      changed: Boolean,
      error: String,
      t: { type: Function, required: true }
    },
    emits: ["confirm", "cancel"],
    setup(props, { emit }) {
      const dialog = ref(null);
      const checkout = computed(() => props.kind === "checkout");
      const discard = computed(() => props.kind === "discard-merge");
      const titleKey = computed(() => discard.value ? "discardConfirm" : checkout.value ? "checkoutTitle" : "deleteVersionConfirm");
      const confirmKey = computed(() => discard.value ? "discardChanges" : checkout.value ? props.busy ? "checkingOut" : "confirmYes" : props.busy ? "deletingVersion" : "deleteAction");
      const previousFocus = document.activeElement;
      const focusCancel = () => dialog.value?.querySelector("[data-version-cancel]")?.focus();
      onMounted(focusCancel);
      watch(() => props.busy, async (busy) => {
        await nextTick();
        if (busy) dialog.value?.focus();
        else focusCancel();
      });
      onBeforeUnmount(() => {
        void nextTick(() => {
          if (previousFocus?.isConnected) previousFocus.focus();
        });
      });
      function keydown(event) {
        if (event.key === "Escape") {
          event.preventDefault();
          if (!props.busy) emit("cancel");
        } else if (event.key === "Tab") {
          const buttons = Array.from(dialog.value.querySelectorAll("button:not([disabled])"));
          const index = buttons.indexOf(document.activeElement);
          if (!buttons.length || event.shiftKey && index <= 0 || !event.shiftKey && index === buttons.length - 1) {
            event.preventDefault();
            buttons[event.shiftKey ? buttons.length - 1 : 0]?.focus();
          }
        }
      }
      return { dialog, checkout, discard, titleKey, confirmKey, keydown };
    },
    template: `
      <section ref="dialog" class="modal version-action-modal" :class="discard ? 'discard-merge-modal' : checkout ? 'checkout-version-modal' : 'delete-version-modal'" role="alertdialog" aria-modal="true"
        aria-labelledby="version-action-title" :aria-describedby="checkout ? 'version-action-warning version-action-target' : 'version-action-target'" :aria-busy="busy"
        tabindex="-1" @keydown="keydown">
        <div class="modal-head"><h3 id="version-action-title">{{ t(titleKey) }}</h3></div>
        <p v-if="checkout" id="version-action-warning" class="version-action-warning">{{ t('checkoutDirtyWarning') }}</p>
        <p id="version-action-target" class="version-action-target"><strong>{{ target.message }}</strong><span>{{ target.shortId }}</span></p>
        <p v-if="changed" class="field-error" role="alert">{{ t('checkoutChanged') }}</p>
        <p v-if="error" class="field-error" role="alert">{{ error }}</p>
        <div class="modal-actions">
          <button type="button" class="ghost-btn" data-version-cancel :disabled="busy" @click="$emit('cancel')">{{ t(checkout ? 'confirmNo' : 'cancel') }}</button>
          <button type="button" class="ghost-btn danger-btn" data-version-confirm :disabled="busy" @click="$emit('confirm')">
            <span v-if="busy" class="btn-spinner"></span>{{ t(confirmKey) }}
          </button>
        </div>
      </section>`
  };
})();
